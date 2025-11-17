package messageservice

import (
	"encoding/json"
	"log"
	"math/big"

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
	}
}

var (
	tempPSK     *big.Int
	tempHubUUID string
)

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
			Message:     "init-key-exchange",
			Type:        comm.Command,
			Data:        payloadBytes,
			Destination: newClientUUID,
		}

		*broadcast <- msg

	case "init-key-exchange":
		// Extract keyhub information payload
		var initPayload KeyExchangePayload
		err := json.Unmarshal(command.Data, &initPayload)
		if err != nil {
			log.Println("init-key-exchange: unmarhsal error:", err)
		}

		// Store the key hub UUID
		tempHubUUID = initPayload.HubUUID

		// Generate client's own keys
		util.SetP(new(big.Int).SetBytes(initPayload.P))
		util.SetG(new(big.Int).SetBytes(initPayload.G))
		util.GeneratePrivateKey()
		util.CalculatePublicKey(util.GetG())

		// Calculate PSK
		hubPubKey := new(big.Int).SetBytes(initPayload.PubKey)
		tempPSK = util.CalculateSharedSecret(hubPubKey)

		response := comm.Message{
			Username:    clientID.String(),
			Message:     "key-exchange-resp",
			Type:        comm.Command,
			Data:        util.GetPublicKey().Bytes(),
			Destination: tempHubUUID,
		}

		*broadcast <- response

	case "key-exchange-resp":
		// Get client's UUID and public key
		newClientUUID := command.Username
		clientPubKey := new(big.Int).SetBytes(command.Data)

		// Calculate PSK
		psk := util.CalculateSharedSecret(clientPubKey)

		// Encrypt room key
		encryptedRoomKey, err := util.Encrypt(util.GetRoomKey(), psk.Bytes())
		if err != nil {
			log.Println("key-exchange-resp: encrypt error:", err)
			return
		}

		// Send the room key to the new client
		roomKeyMsg := comm.Message{
			Username:    clientID.String(),
			Message:     "finish-key-exchange",
			Type:        comm.Command,
			Data:        []byte(encryptedRoomKey),
			Destination: newClientUUID,
		}

		*broadcast <- roomKeyMsg

	case "finish-key-exchange":
		// Get room key
		encryptedRoomKey := string(command.Data)

		// Decrypt it
		roomKey, err := util.Decrypt(encryptedRoomKey, tempPSK.Bytes())
		if err != nil {
			log.Println("finish-key-exchange: decrypt error:", err)
			return
		}

		// Save room key
		util.SetRoomKey(roomKey)
		log.Println("Handshake complete!")

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
