package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"sync"
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
