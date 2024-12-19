package main

import (
	"fmt"
	"log"
	"math/big"
	"net/http"
	"websocket-chat/comm"
	"websocket-chat/util"

	"github.com/gorilla/websocket"
)

type Client struct {
	conn     *websocket.Conn
	isKeyHub bool
	DHDone   bool
}

func (C *Client) ReadMessage() (messageType int, p []byte, err error) {
	return C.conn.ReadMessage()
}

func (C *Client) WriteMessage(messageType int, data []byte) error {
	return C.conn.WriteMessage(messageType, data)
}

func (C *Client) Disconnect() {
	C.conn.Close()
}

func (C *Client) WriteJSON(v interface{}) error {
	return C.conn.WriteJSON(v)
}

type MessageEvent struct {
	message comm.Message
	client  *Client
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var (
	clients             = make(map[*Client]bool)
	broadcast           = make(chan MessageEvent)
	P          *big.Int = util.GeneratePrime()
	G                   = big.NewInt(2)
	privateKey          = util.GeneratePrivateKey(P)
	publicKey           = util.CalculatePublicKey(P, privateKey, G)
	psk        *big.Int
	keyHub     *Client
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

func setKeyHub(client *Client) {
	client.isKeyHub = true
	keyHub = client
}

func homePage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Pablo")
}

// Diffie-Hellman handshake
func doKeyExchange(client *Client) {
	log.Println("New client connected. Exchanging keys...")
	// Send P to client
	client.WriteMessage(websocket.BinaryMessage, P.Bytes())
	// Send G to client
	client.WriteMessage(websocket.BinaryMessage, G.Bytes())
	// Send pub key to client
	client.WriteMessage(websocket.BinaryMessage, publicKey.Bytes())
	// Receive client's pub key
	_, clientPubKeyBytes, err := client.ReadMessage()
	if err != nil {
		log.Println(err)
		delete(clients, client)
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

func negotiateKeys(newClient *Client, currentClient *Client) {
	currentClient.DHDone = false
	// Hub sends P and G to both clients
	// Send P  and G to old client
	log.Println("Sending P and G to clients")
	currentClient.WriteMessage(websocket.BinaryMessage, P.Bytes())
	currentClient.WriteMessage(websocket.BinaryMessage, G.Bytes())
	// Send P and G to new client
	newClient.WriteMessage(websocket.BinaryMessage, P.Bytes())
	newClient.WriteMessage(websocket.BinaryMessage, G.Bytes())
	log.Println("Sent P and G to clients")
	// Both calculate their private and public keys
	// Each client sends its public key to the other
	// Receive new client's public key
	log.Println("Receiving public key from new client")
	_, newClientPubKeyBytes, err := newClient.ReadMessage()
	if err != nil {
		log.Println("pub key recv:", err)
		newClient.Disconnect()
		return
	}
	newClientPubKey := new(big.Int).SetBytes(newClientPubKeyBytes)
	log.Println("Received public key from new client:", newClientPubKey)
	// Receive old client's public key
	log.Println("Receiving public key from old client")
	_, oldClientPubKeyBytes, err := currentClient.ReadMessage()
	if err != nil {
		log.Println("pub key recv:", err)
		// delete(clients, currentClient)
		return
	}
	oldClientPubKey := new(big.Int).SetBytes(oldClientPubKeyBytes)
	log.Println("Received public key from old client:", oldClientPubKey)
	// Send old client's public key to new client
	log.Println("Sending old client's public key to new client")
	newClient.WriteMessage(websocket.BinaryMessage, oldClientPubKey.Bytes())
	log.Println("Sent old client's public key to new client")
	// Send new client's public key to old client
	log.Println("Sending new client's public key to old client")
	currentClient.WriteMessage(websocket.BinaryMessage, newClientPubKey.Bytes())
	log.Println("Sent new client's public key to old client")
	// Each client calculates the PSK

	// clients[currentClient] = true
	// clients[newClient] = true
	currentClient.DHDone = true
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()

	client := &Client{conn: conn, isKeyHub: false, DHDone: false}

	// When there are two clients connecting, do the key exchange
	if len(clients) == 1 {
		log.Println(("We have enough clients for a key exchange! What an exciting time!"))
		for oldClient := range clients {
			log.Println("Doing key exchange!")
			// delete(clients, oldClient)
			negotiateKeys(client, oldClient)
		}
		client.DHDone = true
	}
	log.Println("We left the if block for some reason")

	// doKeyExchange(client)

	// if len(clients) == 0 {
	// 	setKeyHub(client)
	// 	encryptedPabloMessage, err := util.Encrypt([]byte("You are the key czar"), psk.Bytes())
	// 	if err != nil {
	// 		log.Println("encryption:", err)
	// 	}
	// 	keyHub.WriteJSON(comm.Message{Username: "ServerPablo", Message: encryptedPabloMessage})
	// }

	clients[client] = true

	for {
		if client.DHDone {
			var msg comm.Message
			err := conn.ReadJSON(&msg)
			if err != nil {
				fmt.Println(err)
				delete(clients, client)
				return
			}
			log.Println("There was a new message")
			log.Println("Message received:", msg)
			messageEvent := MessageEvent{message: msg, client: client}
			broadcast <- messageEvent
		}
	}
}

func handleMessages() {
	for {
		msg := <-broadcast
		// decryptedBytes, err := util.Decrypt(msg.message.Message, psk.Bytes())
		// if err != nil {
		// 	log.Println("decryption:", err)
		// 	continue
		// }
		// decryptedMessage := string(decryptedBytes)
		fmt.Println("Message:", msg.message)
		// fmt.Println("Decrypted message:", decryptedMessage)

		for client := range clients {
			var err error
			if client != msg.client {
				err = client.WriteJSON(msg.message)
			}
			if err != nil {
				fmt.Println(err)
				client.Disconnect()
				delete(clients, client)
			}
		}
	}
}
