package main

const (
	MessageEventCandidate = "candidate"
	MessageEventOffer     = "offer"
	MessageEventAnswer    = "answer"
	MessageEventBroadcast = "broadcast"
)

type MessageProtocol struct {
	Id    string `json:"id"`
	Event string `json:"event"`
	Data  string `json:"data"`
}
