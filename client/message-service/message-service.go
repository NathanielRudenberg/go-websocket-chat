package messageservice

import (
	"encoding/json"
	"log"

	"websocket-chat/comm"
	"websocket-chat/util"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var (
	hostName  *string
	hostPort  *int
	broadcast *chan comm.Message
	clientID  *uuid.UUID
)

type KeyExchangePayload struct {
	P       []byte `json:"p"`
	G       []byte `json:"g"`
	PubKey  []byte `json:"pubKey"`
	HubUUID string `json:"hubUUID"`
}

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
		// go util.ShareKeys(hostName, hostPort)
		newClientUUID := string(command.Data)

		initPayload := KeyExchangePayload{
			P:       util.GetP().Bytes(),
			G:       util.GetG().Bytes(),
			PubKey:  util.GetPublicKey().Bytes(),
			HubUUID: clientID.String(),
		}

		payloadBytes, err := json.Marshal(initPayload)
		if err != nil {
			log.Println("exchange-keys: marshal error:", err)
		}

		msg := comm.Message{
			Username:    clientID.String(),
			Message:     "key-exchange-init",
			Type:        comm.Command,
			Data:        payloadBytes,
			Destination: newClientUUID,
		}

		*broadcast <- msg
	case "generate-keys":
		util.GenerateKeys()
	case "join-chat":

	}
}

func SetHostInfo(hostNameArg *string, hostPortArg *int, broadcastChan *chan comm.Message, id *uuid.UUID) {
	hostName = hostNameArg
	hostPort = hostPortArg
	broadcast = broadcastChan
	clientID = id
}
