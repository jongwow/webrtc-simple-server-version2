package main

import (
	"github.com/pion/rtp"
	"sync"
)

type Uptrack struct {
	sync.RWMutex
	id        string
	pktFChan  chan *rtp.Packet
	pktHChan  chan *rtp.Packet
	pktQChan  chan *rtp.Packet
	downChans [3][]*chan *rtp.Packet
	// chanF, H, Q
	// 이름
}

func (u *Uptrack) Register(fChan *chan *rtp.Packet, hChan *chan *rtp.Packet, qChan *chan *rtp.Packet) {
	u.Lock()
	u.downChans[0] = append(u.downChans[0], fChan)
	u.downChans[1] = append(u.downChans[1], hChan)
	u.downChans[2] = append(u.downChans[2], qChan)
	u.Unlock()
}

func (u *Uptrack) Start() {
	go u.forwardPacketF(0)
	go u.forwardPacketH(1)
	go u.forwardPacketQ(2)
}

func (u *Uptrack) Unregister(fChan *chan *rtp.Packet, hChan *chan *rtp.Packet, qChan *chan *rtp.Packet) {
	u.Lock()
	idx := -1
	for i, cha := range u.downChans[0] {
		if fChan == cha {
			idx = i
			break
		}
	}
	if idx != -1 {
		u.downChans[0][idx] = u.downChans[0][len(u.downChans[0])-1]
		u.downChans[0][len(u.downChans[0])-1] = nil
		u.downChans[0] = u.downChans[0][:len(u.downChans[0])-1]
	}
	for i, cha := range u.downChans[1] {
		if hChan == cha {
			idx = i
			break
		}
	}

	if idx != -1 {
		u.downChans[1][idx] = u.downChans[1][len(u.downChans[1])-1]
		u.downChans[1][len(u.downChans[1])-1] = nil
		u.downChans[1] = u.downChans[1][:len(u.downChans[1])-1]
	}
	for i, cha := range u.downChans[2] {
		if qChan == cha {
			idx = i
			break
		}
	}
	if idx != -1 {
		u.downChans[2][idx] = u.downChans[2][len(u.downChans[2])-1]
		u.downChans[2][len(u.downChans[2])-1] = nil
		u.downChans[2] = u.downChans[2][:len(u.downChans[2])-1]
	}
	u.Unlock()
}

func (u *Uptrack) forwardPacketF(layer int) {
	for pkt := range u.pktFChan {
		u.Lock()
		for _, cha := range u.downChans[layer] {
			*cha <- pkt
		}
		u.Unlock()
	}
}

func (u *Uptrack) forwardPacketH(layer int) {
	for pkt := range u.pktHChan {
		u.Lock()
		for _, cha := range u.downChans[layer] {
			*cha <- pkt
		}
		u.Unlock()
	}
}

func (u *Uptrack) forwardPacketQ(layer int) {
	for pkt := range u.pktQChan {
		u.Lock()
		for _, cha := range u.downChans[layer] {
			*cha <- pkt
		}
		u.Unlock()
	}
}
