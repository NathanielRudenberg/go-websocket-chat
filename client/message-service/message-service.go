package messageservice

import (
	"websocket-chat/comm"
	"websocket-chat/util"

	"github.com/gorilla/websocket"
)

var (
	hostName *string
	hostPort *int
)

func SendEncryptedMessage(messageType int, data []byte, key []byte, conn *websocket.Conn) error {
	encryptedMessage, err := util.Encrypt(data, key)
	if err != nil {
		return err
	}
	return conn.WriteMessage(messageType, []byte(encryptedMessage))
}

func HandleInfo(info *comm.Message, conn *websocket.Conn) {
	switch info.Message {
	// case "ke":
	// 	err := conn.WriteJSON(comm.Message{Username: username, Message: "ke", Type: comm.Info})
	// 	if err != nil {
	// 		log.Println("send info:", err)
	// 	}
	}
}

func HandleCommand(command *comm.Message) {
	switch command.Message {
	case "exchange-keys":
		// Only the key hub should receive this command
		go util.ShareKeys(hostName, hostPort)
	case "generate-keys":
		util.GenerateKeys()
	case "join-chat":

	}
}

func SetHostInfo(hostNameArg *string, hostPortArg *int) {
	hostName = hostNameArg
	hostPort = hostPortArg
}
