package main

import "github.com/pion/rtp"

type Uptrack struct {
	id       string
	pktFChan *chan rtp.Packet
	pktHChan *chan rtp.Packet
	pktQChan *chan rtp.Packet
	// chanF, H, Q
	// 이름
}
