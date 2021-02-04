package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"sync"
	"time"
)

const (
	WriteWait = 10 * time.Second
	// Time allowed to read the next pong message from the peer.
	PongWait = 60 * time.Second
	// Send pings to peer with this period. Must be less than pongWait.
	PingPeriod = (PongWait * 9) / 10 // 54초마다 핑퐁
)

type SafeConn struct {
	*websocket.Conn
	sync.Mutex
}

func (t *SafeConn) safeWriteJSON(v *MessageProtocol) error {
	fmt.Println("safeWriteJSON Called, ", v.Id, " : ", v.Event)
	t.Lock()
	defer t.Unlock()

	return t.WriteJSON(v)
}
