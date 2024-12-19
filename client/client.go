package main

import (
	"bufio"
	"fmt"
	"log"
	"math/big"
	"net/url"
	"os"
	"os/signal"
	"time"
	"websocket-chat/chat"
	"websocket-chat/util"

	"github.com/gorilla/websocket"
)

var (
	username string
	P        *big.Int
	psk      *big.Int
)

func doKeyExchange(conn *websocket.Conn) {
	// Receive P from server
	log.Println("Connecting to server. Exchanging keys...")
	_, Pbytes, err := conn.ReadMessage()
	if err != nil {
		log.Println("handshake:", err)
		return
	}
	P := new(big.Int).SetBytes(Pbytes)
	// Receive G from server
	_, Gbytes, err := conn.ReadMessage()
	if err != nil {
		log.Println("handshake:", err)
		return
	}
	G := new(big.Int).SetBytes(Gbytes)
	// Receive pub key from server
	_, serverPubKeyBytes, err := conn.ReadMessage()
	if err != nil {
		log.Println("handshake:", err)
		return
	}
	serverPubKey := new(big.Int).SetBytes(serverPubKeyBytes)
	// Calculate private key
	privateKey := util.GeneratePrivateKey(P)
	// Calculate public key
	publicKey := util.CalculatePublicKey(P, privateKey, G)
	// Send public key to server
	conn.WriteMessage(websocket.BinaryMessage, publicKey.Bytes())
	// Calculate PSK with server pub key
	psk = util.CalculateSharedSecret(P, privateKey, serverPubKey)
	log.Println("Finished exchanging keys")
	// Use PSK to encrypt and decrypt messages
	log.Println("Client PSK:", psk)
}

func main() {
	log.Print("program running")
	broadcast := make(chan chat.Message)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	// go handleMessages()

	// Get username from user
	fmt.Print("Enter your username: ")
	reader := bufio.NewReader(os.Stdin)
	username, _ = reader.ReadString('\n')
	username = username[:len(username)-1]

	u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws"}
	log.Printf("connecting to %s", u.String())

	conn, response, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Printf("handshake failed with status %d", response.StatusCode)
		log.Fatal("dial:", err)
	}
	defer conn.Close()

	doKeyExchange(conn)

	done := make(chan struct{})
	connectionHandler := func() {
		defer close(done)

		for {
			var msg chat.Message
			err := conn.ReadJSON(&msg)
			if err != nil {
				log.Println("read:", err)
				return
			}

			// Decrypt message
			decryptedBytes, err := util.Decrypt(msg.Message, psk.Bytes())
			if err != nil {
				log.Println("decryption:", err)
				continue
			}
			decryptedMessage := string(decryptedBytes)

			fmt.Println(msg)
			fmt.Println(decryptedMessage)
		}
	}

	writeHandler := func() {
		for {
			// fmt.Print("You: ")
			writeMsg := chat.Message{Username: username, Message: ""}
			messageInput, _ := reader.ReadString('\n')
			msgLength := len(messageInput)
			messageInput = messageInput[:msgLength-1]
			// Encrypt the message
			encryptedMessage, err := util.Encrypt([]byte(messageInput), psk.Bytes())
			if err != nil {
				log.Println("encryption:", err)
				continue
			}

			writeMsg.Message = encryptedMessage
			if writeMsg.Message != "" {
				broadcast <- writeMsg
			}
			// select {
			// case <-done:
			// 	log.Println("done")
			// 	return
			// case <-time.After(time.Millisecond):
			// }
		}
	}

	go connectionHandler()
	go writeHandler()

	// ticker := time.NewTicker(time.Second)
	// defer ticker.Stop()

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
		// case t := <-ticker.C:
		// 	message := chat.Message{Username: "PabloTest", Message: t.String()}
		// 	timeMessage := message
		// 	err := conn.WriteJSON(timeMessage)
		// 	if err != nil {
		// 		log.Println("write:", err)
		// 		return
		// 	}
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
