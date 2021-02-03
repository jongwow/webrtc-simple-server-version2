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
	pktFChan           chan rtp.Packet
	pktHChan           chan rtp.Packet
	pktQChan           chan rtp.Packet
}

func NewClient(room *Room, c *websocket.Conn, clientName string) *Client {
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
		ChanBroadcast:      room.ChanBroadcast,
	}
	client.pktFChan = make(chan rtp.Packet, 100)
	client.pktHChan = make(chan rtp.Packet, 100)
	client.pktQChan = make(chan rtp.Packet, 100)
	return client
}
func (c *Client) Run() {
	for {
		select {
		case message := <-c.ChanOutbound:
			fmt.Println(message)
		case _ = <-c.ChanConnected:
			fmt.Println("Client: Run Connected.")
			c.room.ChanConnected <- c
		}
	}
}

func (c *Client) Close() {

}

func (c *Client) GetName() string {
	return c.name
}

func (c *Client) AddPeerConnection(id string) string {
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
	//isMine := c.id == id
	//if isMine {
	pc.OnTrack(handleOnTrack(c)) // 원래는 이걸 나눴는데 안나눠도 되야지.
	//}
	return id
}

//TODO: Audio 고려 안함!! Video만 고려함!
func handleOnTrack(c *Client) func(*webrtc.TrackRemote, *webrtc.RTPReceiver) {
	return func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		trackRid := track.RID()

		// packet이 delta가 되게끔 lastTimestamp를 바꿀 것.
		myPC := c.peerConnections[c.id]
		var lastTimestamp uint32
		const maxTotalBitrate = 2000000

		go func() {
			ticker := time.NewTicker(3 * time.Second)
			for range ticker.C {
				fmt.Printf("Sending pli for stream with rid: %q, ssrc: %d, mimeType: %q\n", trackRid, track.SSRC(), track.Codec().MimeType)
				if writeErr := myPC.WriteRTCP([]rtcp.Packet{
					&rtcp.ReceiverEstimatedMaximumBitrate{Bitrate: maxTotalBitrate, SSRCs: []uint32{uint32(track.SSRC())}},
				}); writeErr != nil {
					fmt.Println(writeErr)
				}
				if writeErr := myPC.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(track.SSRC())}}); writeErr != nil {
					fmt.Println(writeErr)
				}
			}
		}()

		for {
			rtp, _, readErr := track.ReadRTP()
			if readErr != nil {
				panic(readErr)
			}

			// Change the timestamp to only be the delta
			oldTimestamp := rtp.Timestamp
			if lastTimestamp == 0 {
				rtp.Timestamp = 0
			} else {
				rtp.Timestamp -= lastTimestamp
			}
			lastTimestamp = oldTimestamp

			// track에 맞춰서 rtp Chan에 해당 packets들을 넣어줌.
			go func() {
				//TODO: 이렇게 우회하면  deadlock같은게 없으려나
				// 이 chan에서 deadlock 없는게 과연 좋은걸까? realtime의 whole point 자체가 패킷 몇개 유실되도 상관없어야하는거아닐까
				if trackRid == "f" {
					fmt.Println("track RID f sent pkt")
					c.pktFChan <- *rtp
				} else if trackRid == "h" {
					fmt.Println("track RID h sent pkt")
					c.pktHChan <- *rtp
				} else if trackRid == "q" {
					fmt.Println("track RID q sent pkt")
					c.pktQChan <- *rtp
				}
			}()
		}
	}
}

func handleOnNegotiationNeeded(c *Client, id string) func() {
	return func() {
		fmt.Printf("(perfect negotiation)\n")
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
		fmt.Printf("peer[%d]의 PC[%d]의 ConnectionState: %s\n", c.id, id, state.String())
		if state == webrtc.PeerConnectionStateFailed {
			log.Println("이 PeerConnection은 Failed상태. TODO: 예외처리가 필요")
		}
		if state == webrtc.PeerConnectionStateClosed {
			log.Println("peerConnections is closed")
		}
		if state == webrtc.PeerConnectionStateConnected {
			c.ChanConnected <- true
			fmt.Println("PeerConnection Connected")
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
