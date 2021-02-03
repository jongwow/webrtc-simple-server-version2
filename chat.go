package main

const (
	MessageEventCandidate = "candidate"
	MessageEventOffer     = "offer'"
)

type MessageProtocol struct {
	Id    string `json:"id"`
	Event string `json:"event"`
	Data  string `json:"data"`
}
