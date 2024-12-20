package serverclient

import (
	"websocket-chat/comm"

	"github.com/gorilla/websocket"
)

type Client struct {
	Conn     *websocket.Conn
	isKeyHub bool
	DHDone   bool
}

func (C *Client) ReadMessage() (messageType int, p []byte, err error) {
	return C.Conn.ReadMessage()
}

func (C *Client) WriteMessage(messageType int, data []byte) error {
	return C.Conn.WriteMessage(messageType, data)
}

func (C *Client) Disconnect() {
	C.Conn.Close()
}

func (C *Client) WriteJSON(v interface{}) error {
	return C.Conn.WriteJSON(v)
}

func (C *Client) SendCommand(command string) error {
	return C.WriteJSON(comm.Message{Username: "server", Message: command, Type: comm.Command})
}

func (C *Client) SendInfo(info string) error {
	return C.WriteJSON(comm.Message{Username: "server", Message: info, Type: comm.Info})
}

func (C *Client) SendText(text string) error {
	return C.WriteJSON(comm.Message{Username: "server", Message: text, Type: comm.Text})
}

func (C *Client) SetIsKeyHub(isKeyHub bool) {
	C.isKeyHub = isKeyHub
}

func (C *Client) IsKeyHub() bool {
	return C.isKeyHub
}
