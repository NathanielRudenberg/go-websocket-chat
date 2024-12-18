package chat

import "fmt"

type Message struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

func (msg Message) String() string {
	return fmt.Sprintf("%s: %s", msg.Username, msg.Message)
}

func TestFunc() string {
	return "pablotestfunc"
}
