package main

import "fmt"

type Room struct {
	name          string
	ChanEnter     chan *Client
	ChanLeave     chan *Client
	ChanBroadcast chan *MessageProtocol
	clientMap     map[string]*Client
	ChanConnected chan *Client
	UptrackMaps   map[string]*Uptrack
}

func (r *Room) Run() {
	for {
		select {
		case client := <-r.ChanEnter:
			// 이부분도 함수로 빼면 깔끔하지 않을까?
			fmt.Println("새로운 Client 등록", client.GetName())
			// clientMap에 등록
			r.clientMap[client.GetName()] = client

			// client를 실행
			client.AddPeerConnection(client.id)

			// 다른 client들에게 들어왔다고 알려주기
			for _, otherClient := range r.clientMap {
				other := otherClient
				if other.id != client.id {
					go func() {
						other.AddPeerConnection(client.id)
						client.AddPeerConnection(other.id)
					}()
				}
			}
			fmt.Printf("Client[%s]등록 완료띠\n", client.GetName())
		case client := <-r.ChanLeave:
			fmt.Println("기존 Client 종료", client.GetName())
		// 다른 client에게서도 해당 client 삭제
		// 해당 client를 삭제
		case client := <-r.ChanConnected:
			// client가 연결이 완료됐어요.
			fmt.Println("client", client.id, " 연결 완료. uptracks를 업데이트.")
			_, ok := r.UptrackMaps[client.id]
			if ok {
				fmt.Println("이미 존재하는 Uptrack 입니다.")
				continue
			}
			// 생성된 chan을 uptracksMap에 등록
			u := &Uptrack{
				id:       client.id,
				pktFChan: client.pktFChan,
				pktHChan: client.pktFChan,
				pktQChan: client.pktFChan,
			}
			r.UptrackMaps[client.id] = u

			u.Start()

			// 등록된 upTrack을 기존의 client들에게 알려주기
			for _, other := range r.clientMap {
				one := other
				go func() {
					// 자기 자신이 아닐 경우 AddTrack을 해주기
					if one.id != u.id {
						one.AddTrack(u)
					}
				}()
			}
		case message := <-r.ChanBroadcast:
			fmt.Println("브로드캐스트 Message received", message.Id)
		}
	}
}

func (r *Room) Close() {
	fmt.Println("room: Close called")
}

func (r *Room) isEmpty() bool {
	fmt.Println("room: isEmpty called")
	return len(r.clientMap) == 0
}

func (r *Room) AddClient(client *Client) {
	fmt.Println("ChanEnter <- client", client.id)
	r.ChanEnter <- client
}

func NewRoom(name string) *Room {
	fmt.Println("room: NewRoom called")
	room := &Room{
		name:          name,
		ChanBroadcast: make(chan *MessageProtocol, 256),
		ChanEnter:     make(chan *Client),
		ChanConnected: make(chan *Client),
		ChanLeave:     make(chan *Client),
		UptrackMaps:   make(map[string]*Uptrack),
		clientMap:     make(map[string]*Client),
	}
	return room
}
