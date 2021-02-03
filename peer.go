package main

import (
	"github.com/pion/webrtc"
	"log"
)

type PerfectNegotiation struct {
	polite                       bool
	makingOffer                  bool
	ignoreOffer                  bool
	isSettingRemoteAnswerPending bool
}

func NewPerfectNegotiation() *PerfectNegotiation {
	pn := &PerfectNegotiation{
		polite:                       false,
		makingOffer:                  false,
		ignoreOffer:                  false,
		isSettingRemoteAnswerPending: false,
	}
	return pn
}

func NewPeerConnection() (*webrtc.PeerConnection, error) {
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
		SDPSemantics: webrtc.SDPSemanticsUnifiedPlan,
	}
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		log.Println("peerConnections Creation Error", err)
		return nil, err
	}
	return peerConnection, nil
}
