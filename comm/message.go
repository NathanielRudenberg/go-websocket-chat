package comm

import (
	"fmt"
	"websocket-chat/util"
)

type Message struct {
	Username string `json:"username"`
	Message  string `json:"message"`
	Type     int    `json:"messageType"`
	Data     []byte `json:"data"`
}

func (msg Message) String() string {
	if msg.Type == Text {
		return fmt.Sprintf("%s: %s", msg.Username, msg.Message)
	}
	return fmt.Sprintf("{%s %s %d %s}", msg.Username, msg.Message, msg.Type, msg.Data)
}

func (msg *Message) Print() error {
	key := util.GetRoomKey()
	decryptedBytes, err := util.Decrypt(msg.Message, key)
	if err != nil {
		return err
	}
	decryptedMessage := string(decryptedBytes)
	fmt.Printf("%s: %s\n", msg.Username, decryptedMessage)
	return nil
}

func (msg *Message) GetDecryptedMessage() (string, error) {
	key := util.GetRoomKey()
	decryptedBytes, err := util.Decrypt(msg.Message, key)
	if err != nil {
		return "", err
	}
	decryptedMessage := string(decryptedBytes)

	message := fmt.Sprintf("%s: %s", msg.Username, decryptedMessage)
	return message, nil
}

const (
	Text = iota
	Command
	Info
)

// Valid command names
const (
	ExchangeKeys = iota
	ShareRoomKey
	GenerateKeys
)
