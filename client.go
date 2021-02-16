package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/webrtc"
	"io"
	"log"
	"sync"
	"time"
)

type Client struct {
	id                 string // UUID
	name               string // 사용자가 정의한 이름
	room               *Room  // 본인이 속한 Room
	peerConnections    map[string]*webrtc.PeerConnection
	perfectNegotiation map[string]*PerfectNegotiation
	websocket          *SafeConn
	ChanOutbound       chan *MessageProtocol // 나가는 Message 다룸
	ChanBroadcast      chan *MessageProtocol // 브로드캐스트할 채널. Room에서부터 받음.
	ChanInbound        chan *MessageProtocol // 들어오는 Message 다룸
	ChanConnected      chan bool
	ChanKill           chan bool
	UpTracks           map[string]*webrtc.TrackLocalStaticRTP
}

func NewClient(room *Room, c *websocket.Conn, clientName string) *Client {
	fmt.Println("Client: ", clientName, " created")
	conn := &SafeConn{c, sync.Mutex{}}
	client := &Client{
		id:                 uuid.New().String(),
		name:               clientName,
		room:               room,
		peerConnections:    make(map[string]*webrtc.PeerConnection),
		perfectNegotiation: make(map[string]*PerfectNegotiation),
		websocket:          conn,
		ChanOutbound:       make(chan *MessageProtocol, 256),
		ChanInbound:        make(chan *MessageProtocol, 256),
		ChanConnected:      make(chan bool),
		ChanKill:           make(chan bool),
		ChanBroadcast:      room.ChanBroadcast,
	}
	return client
}
func (c *Client) Run() {
	// pipeline
	fmt.Println("client run:", c.id)
	go c.readPump()
	go c.writePump()
	for {
		select {
		case _ = <-c.ChanConnected:
			fmt.Println("Client:Run: Connected.")
			c.room.ChanConnected <- c
			fmt.Println("Client:Run: ChanConnected에 전송띠")
		case kill := <-c.ChanKill:
			fmt.Println("Client:Run Kill", kill)
		}
	}
	//fmt.Println("client run:", c.id, "==========END")
}

func (c *Client) Close() {

}

func (c *Client) GetName() string {
	return c.name
}

func (c *Client) AddPeerConnection(id string) string {
	fmt.Printf("client[%s] AddPeerConnection: %s\n", c.name, id)
	pc, err := NewPeerConnection()
	if err != nil {
		panic(err)
	}
	pn := NewPerfectNegotiation()

	_, ok := c.peerConnections[id]
	if ok {
		panic(errors.New("이미 존재하는 client ID입니다."))
	}
	c.peerConnections[id] = pc
	c.perfectNegotiation[id] = pn

	pc.OnICECandidate(handleOnICECandidate(c, id))
	pc.OnConnectionStateChange(handleOnConnectionStateChange(c, id))
	pc.OnNegotiationNeeded(handleOnNegotiationNeeded(c, id))
	pc.OnICEConnectionStateChange(handleOnICEConnectionStateChange())
	//isMine := c.id == id
	//if isMine {
	pc.OnTrack(handleOnTrack(c)) // 원래는 이걸 나눴는데 안나눠도 되야지.
	//}
	return id
}

func handleOnICEConnectionStateChange() func(webrtc.ICEConnectionState) {
	return func(state webrtc.ICEConnectionState) {
		fmt.Printf("handleOnICEConnectionStateChange: %s \n", state.String())
	}
}

func (c *Client) AddTrack(u *Uptrack) {
	fmt.Printf("Client[%s] Addtrack: %s\n", c.id, u.id)
	pc, ok := c.peerConnections[u.id]
	if !ok {
		log.Println("존재하지 않는 peerConnection 입니다.")
		panic(errors.New("존재하지 않는 peerConnection 입니다.")) //TODO: 나중에는 예외처리하기
		return
	}

	//=== 현재 Packet을 받는 Track ===//
	myPktFChan := make(chan *rtp.Packet, 100)
	myPktHChan := make(chan *rtp.Packet, 100)
	myPktQChan := make(chan *rtp.Packet, 100)
	currentChan := myPktFChan
	u.Register(&myPktFChan, &myPktHChan, &myPktQChan)
	//defer u.Unregister(&myPktFChan, &myPktHChan, &myPktQChan)
	//=== Track 추가하기 ===//
	//TODO: 지금은 Video밖에 없음~ 그리고 일단 코덱은 VP8로 통일~ 나중엔 코덱도 Uptrack에 넣으면 될 듯.
	outputTrack, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: "video/vp8"}, u.id, u.id) //TODO: 이름도 rand하게하거나, 기존의 이름을 가져오면 좋을 듯.
	if err != nil {
		panic(err)
	}
	rtpSender, err := pc.AddTrack(outputTrack)
	if err != nil {
		panic(err)
	}
	fmt.Println("rtpSender: ", rtpSender.Track().Kind())

	//=== Network 상태 확인하기 ===//
	processRTCP := func(sender *webrtc.RTPSender) {
		for {
			if readRTCP, _, rtcpErr := sender.ReadRTCP(); rtcpErr != nil {
				return
			} else {
				for _, rRTCP := range readRTCP {
					bitrate, rtcpErr := getBitrate(rRTCP)
					if rtcpErr == nil {
						//fmt.Println("Bitrate: ", bitrate)
						// Bitrate보고 Packet 받는 source 를 변경하는 로직.
						if bitrate < 300000 {
							if currentChan != myPktQChan {
								currentChan = myPktQChan
							}
						} else if bitrate < 1000000 {
							if currentChan != myPktHChan {
								currentChan = myPktHChan
							}
						} else {
							if currentChan != myPktFChan {
								currentChan = myPktFChan
							}
						}
					}
				}
			}
		}
	}
	for idx, sender := range pc.GetSenders() {
		fmt.Printf("pc: %s, idx: %d\n", c.id, idx)
		kind := sender.Track().Kind()
		if kind == webrtc.RTPCodecTypeVideo {
			go processRTCP(sender)
		}
	}

	//=== RTP 패킷을 전달받아 Write  ===//
	go func() {
		go func() {

			var currTimestamp uint32
			for i := uint16(0); ; i++ {

				packet := <-myPktQChan
				currTimestamp += packet.Timestamp
				packet.Timestamp = currTimestamp
				// Keep an increasing sequence number
				packet.SequenceNumber = i
				if currentChan != myPktQChan {
					continue
				}
				fmt.Printf("client[%s]는 packet(%s):%d 을 받았습니다.\n", c.GetName(), "q", i)
				if err := outputTrack.WriteRTP(packet); err != nil && !errors.Is(err, io.ErrClosedPipe) {
					panic(err)
				}
			}
		}()
		go func() {

			var currTimestamp uint32
			for i := uint16(0); ; i++ {
				packet := <-myPktHChan
				currTimestamp += packet.Timestamp
				packet.Timestamp = currTimestamp
				// Keep an increasing sequence number
				packet.SequenceNumber = i
				if currentChan != myPktHChan {
					continue
				}
				fmt.Printf("client[%s]는 packet(%s):%d 을 받았습니다.\n", c.GetName(), "h", i)
				if err := outputTrack.WriteRTP(packet); err != nil && !errors.Is(err, io.ErrClosedPipe) {
					panic(err)
				}
			}
		}()
		go func() {

			var currTimestamp uint32
			for i := uint16(0); ; i++ {
				packet := <-myPktFChan
				currTimestamp += packet.Timestamp
				packet.Timestamp = currTimestamp
				// Keep an increasing sequence number
				packet.SequenceNumber = i
				if currentChan != myPktFChan {
					continue
				}
				fmt.Printf("client[%s]는 packet(%s):%d 을 받았습니다.\n", c.GetName(), "f", i)
				if err := outputTrack.WriteRTP(packet); err != nil && !errors.Is(err, io.ErrClosedPipe) {
					panic(err)
				}
			}
		}()
	}()
}

//TODO: Audio 고려 안함!! Video만 고려함!
func handleOnTrack(c *Client) func(*webrtc.TrackRemote, *webrtc.RTPReceiver) {
	return func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		fmt.Println("handleOnTrack")
		trackLocal, err := webrtc.NewTrackLocalStaticRTP(track.Codec().RTPCodecCapability, track.ID(), "remoteStreamID")
		if err != nil {
			panic(err)
		}
		//== track이 add됐다는 사실을 알리는 method 필요.

		buf := make([]byte, 2048)
		for {
			i, _, err := track.Read(buf)
			if err != nil {
				log.Println("[Ontrack] remote Read Warning: ", err)
				return
			}
			if _, err = trackLocal.Write(buf[:i]); err != nil {
				log.Println("[Ontrack] trackLocal Write Warning: ", err)
				return
			}
		}
	}
}

func handleOnNegotiationNeeded(c *Client, id string) func() {
	return func() {
		fmt.Printf("client[%s], perfect negotiation\n", c.name)
		c.perfectNegotiation[id].makingOffer = true
		defer func() {
			if c.perfectNegotiation[id] != nil {
				c.perfectNegotiation[id].makingOffer = false
			}
		}()

		offer, err := c.peerConnections[id].CreateOffer(nil)
		if err != nil {
			log.Println("CreateOffer Error:", err)
			panic(err)
		}

		if err = c.peerConnections[id].SetLocalDescription(offer); err != nil {
			log.Println("SetLocalDescription Error:", err)
			panic(err)
		}

		offerStr, err := json.Marshal(*c.peerConnections[id].LocalDescription())
		if err != nil {
			log.Println("JSON Marshal Error:", err)
			panic(err)
		}

		c.ChanOutbound <- &MessageProtocol{Id: id, Event: MessageEventOffer, Data: string(offerStr)}
	}
}

func handleOnConnectionStateChange(c *Client, id string) func(webrtc.PeerConnectionState) {
	return func(state webrtc.PeerConnectionState) {
		fmt.Printf("client[%s]의 PC[%s]의 ConnectionState: %s\n", c.id, id, state.String())
		if state == webrtc.PeerConnectionStateFailed {
			log.Println("이 PeerConnection은 Failed상태. TODO: 예외처리가 필요")
		}
		if state == webrtc.PeerConnectionStateClosed {
			log.Println("peerConnections is closed")
		}
		if state == webrtc.PeerConnectionStateConnected {
			fmt.Println("PeerConnection Connected")
			c.ChanConnected <- true
		}
	}

}

func handleOnICECandidate(c *Client, id string) func(*webrtc.ICECandidate) {
	return func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}
		candidateStr, err := json.Marshal(candidate.ToJSON())
		if err != nil {
			log.Println("handleOnICECandidate Error: ", err)
			return
		}
		c.ChanOutbound <- &MessageProtocol{Id: id, Event: MessageEventCandidate, Data: string(candidateStr)}
	}
}

func getBitrate(rRTCP rtcp.Packet) (uint64, error) {
	h := rtcp.Header{}

	marshal, rtcpErr := rRTCP.Marshal()
	if rtcpErr != nil {
		return 0, rtcpErr
	}
	rtcpErr = h.Unmarshal(marshal)
	if rtcpErr != nil {
		return 0, rtcpErr
	}
	if h.Type == rtcp.TypePayloadSpecificFeedback && h.Count == rtcp.FormatREMB {
		r := new(rtcp.ReceiverEstimatedMaximumBitrate)
		err := r.Unmarshal(marshal)
		return r.Bitrate, err
	} else {
		return 0, errors.New("없음")
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(PingPeriod)
	defer func() {
		ticker.Stop()
	}()
	for {
		select {
		case message, ok := <-c.ChanOutbound:
			if !ok {
				log.Println("Channel Closed")
				_ = c.websocket.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.websocket.SetWriteDeadline(time.Now().Add(WriteWait))

			err := c.websocket.safeWriteJSON(message)
			if err != nil {
				log.Println("Write error:", err)
				return
			}
		case <-ticker.C:
			c.websocket.SetWriteDeadline(time.Now().Add(WriteWait))
			if err := c.websocket.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}

	}
}

func (c *Client) readPump() {
	defer func() {
		//c.conn.Close()
	}()

	c.websocket.SetReadDeadline(time.Now().Add(PongWait))
	c.websocket.SetPongHandler(func(string) error { c.websocket.SetReadDeadline(time.Now().Add(PongWait)); return nil })
	message := &MessageProtocol{}
	for {
		_, raw, err := c.websocket.ReadMessage()
		if err != nil {
			log.Println("Read Warning: ", err)
			return
		} else if err := json.Unmarshal(raw, &message); err != nil {
			log.Println("jsonMarshalError:", err)
			return
		}
		fmt.Printf("Message From Room[%s], Client[%s]... %s\n", c.room.name, c.name, message.Event)
		switch message.Event {
		case MessageEventCandidate:
			if c.handleCandidate(message) {
				fmt.Println("c.handleCandidate(message) Error")
				return
			}
		case MessageEventAnswer:
			if c.handleAnswer(message) {
				fmt.Println("c.handleAnswer(message) Error")
				return
			}
		case MessageEventOffer:
			if c.handleOffer(message) {
				fmt.Println("c.handleOffer(message) Error")
				return
			}
		case MessageEventBroadcast:
			if c.handleBroadcast(message) {
				fmt.Println("c.handleBroadcast(message) Error")
				return
			}
		default:
			fmt.Println("Unseen Data")
		}
	}
}

func (c *Client) handleCandidate(message *MessageProtocol) bool {
	fmt.Printf("> client [%s]에서 Candidate가 들어왔어요!\n", c.id)

	candidate := webrtc.ICECandidateInit{}
	if err := json.Unmarshal([]byte(message.Data), &candidate); err != nil {
		log.Println("Icecandidate Error: ", err)
		return true
	}
	if err := c.peerConnections[message.Id].AddICECandidate(candidate); err != nil {
		if !c.perfectNegotiation[message.Id].ignoreOffer {
			log.Println("add icecandidate Error: ", err)
			return true
		}
	}
	return false

}

func (c *Client) handleAnswer(message *MessageProtocol) bool {
	fmt.Printf("> client [%s]에서 Answer가 들어왔어요!\n", c.id)

	c.perfectNegotiation[message.Id].ignoreOffer = false
	if c.perfectNegotiation[message.Id].ignoreOffer {
		return false
	}

	c.perfectNegotiation[message.Id].isSettingRemoteAnswerPending = true

	answer := webrtc.SessionDescription{}
	if err := json.Unmarshal([]byte(message.Data), &answer); err != nil {
		log.Println(err)
		return true
	}

	if err := c.peerConnections[message.Id].SetRemoteDescription(answer); err != nil {
		log.Println(err)
		return true
	}
	c.perfectNegotiation[message.Id].isSettingRemoteAnswerPending = false

	return false

}

func (c *Client) handleOffer(message *MessageProtocol) bool {
	fmt.Printf("> client[%s]에서 Offer가 들어왔어요!\n", c.id)
	readyForOffer := !c.perfectNegotiation[message.Id].makingOffer &&
		(c.peerConnections[message.Id].SignalingState() == webrtc.SignalingStateStable || c.perfectNegotiation[message.Id].isSettingRemoteAnswerPending)

	offerCollision := !readyForOffer

	c.perfectNegotiation[message.Id].ignoreOffer = !c.perfectNegotiation[message.Id].polite && offerCollision
	if c.perfectNegotiation[message.Id].ignoreOffer {
		return false
	}
	c.perfectNegotiation[message.Id].isSettingRemoteAnswerPending = false // offer 라서 false

	offer := webrtc.SessionDescription{}
	if err := json.Unmarshal([]byte(message.Data), &offer); err != nil {
		log.Println(err)
		return true
	}
	if err := c.peerConnections[message.Id].SetRemoteDescription(offer); err != nil {
		log.Println(err)
		return true
	}
	c.perfectNegotiation[message.Id].isSettingRemoteAnswerPending = false

	// answer 를 생성
	answer, err := c.peerConnections[message.Id].CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	err = c.peerConnections[message.Id].SetLocalDescription(answer)
	if err != nil {
		panic(err)
	}

	payload, err := json.Marshal(answer)
	if err != nil {
		panic(err)
	}

	c.ChanOutbound <- &MessageProtocol{Id: message.Id, Event: MessageEventAnswer, Data: string(payload)}

	return false
}

func (c *Client) handleBroadcast(message *MessageProtocol) bool {
	fmt.Printf("> client[%s]에서 Broadcast가 들어왔어요!\n", c.id)
	if by, err := json.Marshal(message.Data); err == nil {
		c.ChanBroadcast <- &MessageProtocol{
			Id:    c.id,
			Event: MessageEventBroadcast,
			Data:  string(by),
		}
	}
	return false

}
