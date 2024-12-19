package main

import (
	"fmt"
	"log"
	"math/big"
	"net/http"
	"websocket-chat/chat"
	"websocket-chat/util"

	"github.com/gorilla/websocket"
)

type MessageEvent struct {
	message chat.Message
	conn    *websocket.Conn
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var (
	clients             = make(map[*websocket.Conn]bool)
	broadcast           = make(chan MessageEvent)
	P          *big.Int = util.GeneratePrime()
	G                   = big.NewInt(2)
	privateKey          = util.GeneratePrivateKey(P)
	publicKey           = util.CalculatePublicKey(P, privateKey, G)
	psk        *big.Int
)

func main() {
	http.HandleFunc("/", homePage)
	http.HandleFunc("/ws", handleConnections)

	go handleMessages()

	fmt.Println("Server started on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		panic("Error starting server: " + err.Error())
	}
}

func homePage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Pablo")
}

// Diffie-Helmann handshake
func doKeyExchange(conn *websocket.Conn) {
	log.Println("New client connected. Exchanging keys...")
	// Send P to client
	conn.WriteMessage(websocket.BinaryMessage, P.Bytes())
	// Send G to client
	conn.WriteMessage(websocket.BinaryMessage, G.Bytes())
	// Send pub key to client
	conn.WriteMessage(websocket.BinaryMessage, publicKey.Bytes())
	// Receive client's pub key
	_, clientPubKeyBytes, err := conn.ReadMessage()
	if err != nil {
		log.Println(err)
		delete(clients, conn)
		return
	}
	clientPubKey := new(big.Int).SetBytes(clientPubKeyBytes)
	// Calculate PSK with pub key
	psk = util.CalculateSharedSecret(P, privateKey, clientPubKey)
	_ = psk
	log.Println("Finished exchanging keys")
	// Use PSK to encrypt and decrypt
	log.Println("Server PSK:", psk)
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()

	clients[conn] = true
	doKeyExchange(conn)

	for {
		var msg chat.Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			fmt.Println(err)
			delete(clients, conn)
			return
		}
		messageEvent := MessageEvent{message: msg, conn: conn}
		broadcast <- messageEvent
	}
}

func handleMessages() {
	for {
		msg := <-broadcast
		decryptedBytes, err := util.Decrypt(msg.message.Message, psk.Bytes())
		if err != nil {
			log.Println("decryption:", err)
			continue
		}
		decryptedMessage := string(decryptedBytes)
		fmt.Println("Message:", msg.message)
		fmt.Println("Decrypted message:", decryptedMessage)

		for client := range clients {
			var err error
			if client != msg.conn {
				err = client.WriteJSON(msg.message)
			}
			if err != nil {
				fmt.Println(err)
				client.Close()
				delete(clients, client)
			}
		}
	}
}
