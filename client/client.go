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
	"websocket-chat/comm"
	"websocket-chat/util"

	"github.com/gorilla/websocket"
)

var (
	username string
	P        *big.Int
	psk      *big.Int
)

func doKeyExchange(conn *websocket.Conn) {
	log.Println("Connecting to server. Exchanging keys...")
	// Receive P from server
	log.Println("Receiving P from server")
	_, Pbytes, err := conn.ReadMessage()
	if err != nil {
		log.Println("handshake:", err)
		return
	}
	log.Println("Received P from server")
	P := new(big.Int).SetBytes(Pbytes)
	// Receive G from server
	log.Println("Receiving G from server")
	_, Gbytes, err := conn.ReadMessage()
	if err != nil {
		log.Println("handshake:", err)
		return
	}
	log.Println("Received G from server")
	G := new(big.Int).SetBytes(Gbytes)

	// Calculate private key
	privateKey := util.GeneratePrivateKey(P)
	// Calculate public key
	publicKey := util.CalculatePublicKey(P, privateKey, G)

	// Send public key to server
	log.Println("Sending public key to server:", publicKey)
	log.Println("Public key bytes:", publicKey.Bytes())
	conn.WriteMessage(websocket.BinaryMessage, publicKey.Bytes())
	log.Println("Sent public key to server")
	// Receive pub key from server
	log.Println("Receiving public key from server")
	_, serverPubKeyBytes, err := conn.ReadMessage()
	if err != nil {
		log.Println("handshake:", err)
		return
	}
	log.Println("Received public key from server")
	serverPubKey := new(big.Int).SetBytes(serverPubKeyBytes)
	// Calculate PSK with server pub key
	psk = util.CalculateSharedSecret(P, privateKey, serverPubKey)
	log.Println("Finished exchanging keys")
	// Use PSK to encrypt and decrypt messages
	log.Println("Client PSK:", psk)

	// TODO: Implement key exchange with multiple clients
}

func main() {
	log.Print("program running")
	broadcast := make(chan comm.Message)
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

	// doKeyExchange(conn)

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

			if msg.Type == comm.Command {
				log.Println("Command received, gonna do a key exchange")
				doKeyExchange(conn)
			}

			if msg.Type == comm.Info {
				log.Println("Info received")
				if msg.Message == "ke" {
					err := conn.WriteJSON(comm.Message{Username: username, Message: "ke", Type: comm.Info})
					if err != nil {
						log.Println("send info:", err)
					}
				}
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
			encryptedMessage, err := util.Encrypt([]byte(messageInput), psk.Bytes())
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
