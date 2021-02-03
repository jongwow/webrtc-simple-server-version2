package main

import (
	"github.com/gorilla/websocket"
	"log"
	"math/rand"
	"net/http"
	"net/url"
)

const (
	paramRequiredMessage = "RoomID가 필요합니다"
)

var sfu = NewSFU()

func appHandler(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			log.Println(r.Header.Get("Origin"))
			return true
		},
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	// Websocket 받을 수 있게 설정
	unsafeConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
	}
	defer unsafeConn.Close()

	// RoomID를 받았는지 검증
	roomId, ok := getRoomIdFromUrl(r)
	if !ok { // 방 번호를 입력하지 않고 접속 시도.
		unsafeConn.WriteMessage(websocket.TextMessage, []byte(paramRequiredMessage))
		return
	}

	// 해당하는 방을 찾기
	room := sfu.GetRoom(roomId)

	// Client를 생성
	clientName := "익명" + string(rune(rand.Intn(100))) // 만약 로그인 절차가 있어 이름을 받는다면 이곳에 전달.
	client := NewClient(room, unsafeConn, clientName) // 새로운 Client를 생성

	// 생성이 완료되면 Client에게 생성 완료 Signal 전송
	err = unsafeConn.WriteJSON(&MessageProtocol{
		Id:    client.id,
		Event: "establish",
		Data:  "",
	})
	if err != nil {
		panic(err)
	}

	// 생성한 Client를 방에 입장
	room.AddClient(client) // client를 입장
}

func getRoomIdFromUrl(r *http.Request) (roomId string, exist bool) {
	roomId, exist = getParamFromQuery(r.URL.Query(), "roomId")
	if !exist {
		return
	}
	return
}

func getParamFromQuery(query url.Values, param string) (matchedParam string, exist bool) {
	params, exist := query[param]
	if !exist || len(params[0]) < 1 {
		log.Println("Url Param", param, "is missing")
		return matchedParam, exist
	}
	return params[0], exist
}
