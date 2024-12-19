package comm

import "fmt"

type Message struct {
	Username string `json:"username"`
	Message  string `json:"message"`
	Type     int    `json:"messageType"`
}

func (msg Message) String() string {
	return fmt.Sprintf("%s: %s", msg.Username, msg.Message)
}

const (
	Text = iota
	Command
	Info
)
