package main

import (
	"bufio"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net/url"
	"os"
	"os/signal"
	"time"
	"websocket-chat/comm"
	"websocket-chat/util"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var (
	username                    string
	P, G, privateKey, publicKey *big.Int
	roomKey                     []byte
	id                          = uuid.New()
)

func checkRoomKey() {
	if roomKey == nil {
		roomKey = make([]byte, 32)
		_, err := rand.Read(roomKey)
		if err != nil {
			log.Println("room key:", err)
		}
	}
}

func sendEncryptedMessage(messageType int, data []byte, key []byte, conn *websocket.Conn) error {
	encryptedMessage, err := util.Encrypt(data, key)
	if err != nil {
		return err
	}
	return conn.WriteMessage(messageType, []byte(encryptedMessage))
}

func handleInfo(info *comm.Message, conn *websocket.Conn) {
	switch info.Message {
	case "ke":
		err := conn.WriteJSON(comm.Message{Username: username, Message: "ke", Type: comm.Info})
		if err != nil {
			log.Println("send info:", err)
		}
	}
}

func handleCommand(command *comm.Message, conn *websocket.Conn) {
	switch command.Message {
	case "exchange-keys":
		// Only the key hub should receive this command
		hostName := "localhost"
		hostPort := 8080
		go shareKeys(&hostName, &hostPort)
	case "generate-keys":
		P = util.GeneratePrime()
		G = big.NewInt(2)
		privateKey = util.GeneratePrivateKey(P)
		publicKey = util.CalculatePublicKey(P, privateKey, G)
		checkRoomKey()
	case "join-chat":

	}
}

func doKeyExchange(conn *websocket.Conn) error {
	// Receive P, G, key hub public key from server
	_, Pbytes, err := conn.ReadMessage()
	if err != nil {
		newError := errors.New("Error receiving P from server:" + err.Error())
		return newError
	}
	P = new(big.Int).SetBytes(Pbytes)

	_, Gbytes, err := conn.ReadMessage()
	if err != nil {
		newError := errors.New("Error receiving G from server:" + err.Error())
		return newError
	}
	G = new(big.Int).SetBytes(Gbytes)

	_, keyHubPubKeyBytes, err := conn.ReadMessage()
	if err != nil {
		newError := errors.New("Error receiving key hub's public key:" + err.Error())
		return newError
	}
	keyHubPubKey := new(big.Int).SetBytes(keyHubPubKeyBytes)

	// Calculate private key
	privateKey = util.GeneratePrivateKey(P)
	// Calculate public key
	publicKey = util.CalculatePublicKey(P, privateKey, G)

	// Send public key to server
	err = conn.WriteMessage(websocket.BinaryMessage, publicKey.Bytes())
	if err != nil {
		newError := errors.New("Error sending public key to server:" + err.Error())
		return newError
	}
	// Calculate PSK with server pub key
	psk := util.CalculateSharedSecret(P, privateKey, keyHubPubKey)

	// Receive room key from server
	_, encryptedRoomKeyBytes, err := conn.ReadMessage()
	if err != nil {
		newError := errors.New("Error receiving room key from server:" + err.Error())
		return newError
	}

	roomKey, err = util.Decrypt(string(encryptedRoomKeyBytes), psk.Bytes())
	if err != nil {
		newError := errors.New("Error decrypting room key:" + err.Error())
		return newError
	}
	return nil
}

func shareKeys(hostName *string, hostPort *int) error {
	u := url.URL{Scheme: "ws", Host: fmt.Sprintf("%s:%d", *hostName, *hostPort), Path: "/key-exchange"}
	conn, response, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Printf("handshake failed with status %d", response.StatusCode)
		log.Fatal("dial:", err)
		return err
	}
	defer conn.Close()

	// Send P, G, public key to server
	err = conn.WriteMessage(websocket.BinaryMessage, P.Bytes())
	if err != nil {
		newError := errors.New("Error sending P to server:" + err.Error())
		return newError
	}

	err = conn.WriteMessage(websocket.BinaryMessage, G.Bytes())
	if err != nil {
		newError := errors.New("Error sending G to server:" + err.Error())
		return newError
	}

	err = conn.WriteMessage(websocket.BinaryMessage, publicKey.Bytes())
	if err != nil {
		newError := errors.New("Error sending public key to server:" + err.Error())
		return newError
	}

	// Receive other client's public key
	_, clientPubKeyBytes, err := conn.ReadMessage()
	if err != nil {
		newError := errors.New("Error receiving client's public key:" + err.Error())
		return newError
	}
	clientPubKey := new(big.Int).SetBytes(clientPubKeyBytes)

	// Calculate PSK
	psk := util.CalculateSharedSecret(P, privateKey, clientPubKey)

	// Send encrypted room key
	encryptedRoomKey, err := util.Encrypt(roomKey, psk.Bytes())
	if err != nil {
		newError := errors.New("Error encrypting room key:" + err.Error())
		return newError
	}
	err = conn.WriteMessage(websocket.BinaryMessage, []byte(encryptedRoomKey))
	if err != nil {
		newError := errors.New("Error sending room key to server:" + err.Error())
		return newError
	}
	return nil
}

func initJoin(hostName *string, hostPort *int) error {
	u := url.URL{Scheme: "ws", Host: fmt.Sprintf("%s:%d", *hostName, *hostPort), Path: "/connect"}
	// log.Printf("connecting to %s", u.String())
	log.Printf("connecting to %s:%d", *hostName, *hostPort)
	conn, response, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Printf("handshake failed with status %d", response.StatusCode)
		log.Fatal("dial:", err)
		return err
	}
	defer conn.Close()

	// Send join message
	uuidBinary, err := id.MarshalBinary()
	if err != nil {
		log.Println("marshal uuid:", err)
		return err
	}
	err = conn.WriteJSON(comm.Message{Username: username, Message: "join", Type: comm.Info, Data: uuidBinary})
	if err != nil {
		log.Println("send join:", err)
		return err
	}

	var msg comm.Message
	err = conn.ReadJSON(&msg)
	if err != nil {
		log.Println("read join chat command:", err)
		return err
	}
	if msg.Type == comm.Info {
		switch msg.Message {
		case "kh-join-done":
			// Should only receive if key hub
			return nil
		case "cl":
			// Should only receive if not key hub
			err := doKeyExchange(conn)
			if err != nil {
				newError := errors.New("Error doing key exchange:" + err.Error())
				return newError
			}
			return nil
		}
	}
	return errors.New("could not join chat")
}

func main() {
	broadcast := make(chan comm.Message)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	hostName := flag.String("host", "localhost", "Server Hostname")
	hostPort := flag.Int("port", 8080, "Server Port")
	flag.Parse()

	// go handleMessages()

	// Get username from user
	fmt.Print("Enter your username: ")
	reader := bufio.NewReader(os.Stdin)
	username, _ = reader.ReadString('\n')
	username = username[:len(username)-1]
	// username := "PabloDebug"

	err := initJoin(hostName, hostPort)
	if err != nil {
		log.Println("join server:", err)
		return
	}

	u := url.URL{Scheme: "ws", Host: fmt.Sprintf("%s:%d", *hostName, *hostPort), Path: "/ws"}

	conn, response, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Printf("handshake failed with status %d", response.StatusCode)
		log.Fatal("dial:", err)
	}
	defer conn.Close()
	log.Println("Joined chat")

	done := make(chan struct{})
	connectionHandler := func() {
		defer close(done)

		for {
			var msg comm.Message
			err := conn.ReadJSON(&msg)
			if err != nil {
				log.Println("read:", err)
				return
			}

			if msg.Type == comm.Text {
				err := msg.Print(roomKey)
				// fmt.Println(msg)
				if err != nil {
					log.Println("decryption:", err)
					continue
				}
			}

			if msg.Type == comm.Command {
				handleCommand(&msg, conn)
			}

			if msg.Type == comm.Info {
				handleInfo(&msg, conn)
			}
		}
	}

	inputHandler := func() {
		// First message to send upon connection is the uuid
		uuidBinary, err := id.MarshalBinary()
		if err != nil {
			log.Println("marshal uuid:", err)
			return
		}
		firstJoinMessage := comm.Message{Username: username, Message: "join", Type: comm.Info, Data: uuidBinary}
		broadcast <- firstJoinMessage
		for {
			messageInput, _ := reader.ReadString('\n')
			msgLength := len(messageInput)
			messageInput = messageInput[:msgLength-1]
			if messageInput == "" {
				continue
			}
			// Encrypt the message
			encryptedMessage, err := util.Encrypt([]byte(messageInput), roomKey)
			if err != nil {
				log.Println("encryption:", err)
				continue
			}

			writeMsg := comm.Message{Username: username, Message: encryptedMessage}
			// writeMsg := comm.Message{Username: username, Message: messageInput, Type: comm.Text}
			broadcast <- writeMsg
			// select {
			// case <-done:
			// 	log.Println("done")
			// 	return
			// case <-time.After(time.Millisecond):
			// }
		}
	}

	go connectionHandler()
	go inputHandler()

	for {
		select {
		case <-done:
			log.Printf("done")
			return
		case m := <-broadcast:
			err := conn.WriteJSON(m)
			if err != nil {
				log.Println("write:", err)
				return
			}
		case <-interrupt:
			log.Println("interrupt")
			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}

			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		}
	}
}

// TODO get handled, idiot
// func handleMessages() {
// }
