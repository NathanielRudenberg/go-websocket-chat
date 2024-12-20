package main

import (
	"bufio"
	"crypto/rand"
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

	"github.com/gorilla/websocket"
)

var (
	username                      string
	P, G, psk                     *big.Int
	myP, myG, myPrivKey, myPubKey *big.Int
	roomKey                       []byte
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

func doKeyExchange(conn *websocket.Conn) {
	// Receive P from server
	_, Pbytes, err := conn.ReadMessage()
	if err != nil {
		log.Println("handshake:", err)
		return
	}
	P = new(big.Int).SetBytes(Pbytes)
	// Receive G from server
	_, Gbytes, err := conn.ReadMessage()
	if err != nil {
		log.Println("handshake:", err)
		return
	}
	G = new(big.Int).SetBytes(Gbytes)

	// Calculate private key
	privateKey := util.GeneratePrivateKey(P)
	// Calculate public key
	publicKey := util.CalculatePublicKey(P, privateKey, G)

	// Send public key to server
	err = conn.WriteMessage(websocket.BinaryMessage, publicKey.Bytes())
	if err != nil {
		log.Println("handshake:", err)
		return
	}
	// Receive pub key from server
	_, serverPubKeyBytes, err := conn.ReadMessage()
	if err != nil {
		log.Println("handshake:", err)
		return
	}
	serverPubKey := new(big.Int).SetBytes(serverPubKeyBytes)
	// Calculate PSK with server pub key
	psk = util.CalculateSharedSecret(P, privateKey, serverPubKey)
}

func handleInfo(info *comm.Message, conn *websocket.Conn) {
	switch info.Message {
	case "ke":
		err := conn.WriteJSON(comm.Message{Username: username, Message: "ke", Type: comm.Info})
		if err != nil {
			log.Println("send info:", err)
		}
	case "rk":
		// Received room key
		rkString := string(info.Data)
		decryptedKeyBytes, err := util.Decrypt(rkString, psk.Bytes())
		if err != nil {

		}
		roomKey = decryptedKeyBytes
	}
}

func handleCommand(command *comm.Message, conn *websocket.Conn) {
	switch command.Message {
	case "exchange-keys":
		doKeyExchange(conn)
	case "share-room-key":
		checkRoomKey()
		err := sendEncryptedMessage(websocket.BinaryMessage, roomKey, psk.Bytes(), conn)
		if err != nil {
			log.Println("send room key:", err)
		}
	case "generate-keys":
		myP = util.GeneratePrime()
		myG = big.NewInt(2)
		myPrivKey = util.GeneratePrivateKey(myP)
		myPubKey = util.CalculatePublicKey(myP, myPrivKey, myG)
		checkRoomKey()
	}
}

func main() {
	log.Print("program running")
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

	u := url.URL{Scheme: "ws", Host: fmt.Sprintf("%s:%d", *hostName, *hostPort), Path: "/ws"}
	log.Printf("connecting to %s", u.String())

	conn, response, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Printf("handshake failed with status %d", response.StatusCode)
		log.Fatal("dial:", err)
	}
	defer conn.Close()

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
			// log.Printf("Send Message %s", m)
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
