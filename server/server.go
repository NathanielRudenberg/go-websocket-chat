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
	clients            = make(map[*Client]bool)
	broadcast          = make(chan MessageEvent)
	P         *big.Int = util.GeneratePrime()
	G                  = big.NewInt(2)
	keyHub    *Client
	// privateKey                              = util.GeneratePrivateKey(P)
	// publicKey                               = util.CalculatePublicKey(P, privateKey, G)
	// psk                            *big.Int
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
// func doKeyExchange(client *Client) {
// 	log.Println("New client connected. Exchanging keys...")
// 	// Send P to client
// 	client.WriteMessage(websocket.BinaryMessage, P.Bytes())
// 	// Send G to client
// 	client.WriteMessage(websocket.BinaryMessage, G.Bytes())
// 	// Send pub key to client
// 	client.WriteMessage(websocket.BinaryMessage, publicKey.Bytes())
// 	// Receive client's pub key
// 	_, clientPubKeyBytes, err := client.ReadMessage()
// 	if err != nil {
// 		log.Println("pub key recv:", err)
// 		delete(clients, client)
// 		return
// 	}
// 	clientPubKey := new(big.Int).SetBytes(clientPubKeyBytes)
// 	// Calculate PSK with pub key
// 	psk = util.CalculateSharedSecret(P, privateKey, clientPubKey)
// 	_ = psk
// 	log.Println("Finished exchanging keys")
// 	// Use PSK to encrypt and decrypt
// 	log.Println("Server PSK:", psk)
// }

func negotiateKeys(newClient *Client, keyHub *Client) {
	// Tell hub and new client to exchange keys
	if keyHub.DHDone {
		log.Println("telling key hub to exchange keys")
		keyHub.WriteJSON(comm.Message{Username: "server", Message: "ke", Type: comm.Info})
	}
	keyHub.WriteJSON(comm.Message{Username: "server", Message: "exchange-keys", Type: comm.Command})
	newClient.WriteJSON(comm.Message{Username: "server", Message: "exchange-keys", Type: comm.Command})
	// Hub sends P and G to both clients
	// Send P  and G to key hub
	log.Println("Sending P and G to clients")
	keyHub.WriteMessage(websocket.BinaryMessage, P.Bytes())
	keyHub.WriteMessage(websocket.BinaryMessage, G.Bytes())
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
		log.Println("nc pub key recv:", err)
		newClient.Disconnect()
		return
	}
	newClientPubKey := new(big.Int).SetBytes(newClientPubKeyBytes)
	log.Println("Received public key from new client:", newClientPubKey)
	// Receive key hub's public key
	log.Println("Receiving public key from key hub")
	_, oldClientPubKeyBytes, err := keyHub.ReadMessage()
	if err != nil {
		log.Println("kh pub key recv:", err)
		// delete(clients, currentClient)
		return
	}
	oldClientPubKey := new(big.Int).SetBytes(oldClientPubKeyBytes)
	log.Println("Received public key from key hub:", oldClientPubKey)
	// Send key hub's public key to new client
	log.Println("Sending key hub's public key to new client")
	newClient.WriteMessage(websocket.BinaryMessage, oldClientPubKey.Bytes())
	log.Println("Sent key hub's public key to new client")
	// Send new client's public key to key hub
	log.Println("Sending new client's public key to key hub")
	keyHub.WriteMessage(websocket.BinaryMessage, newClientPubKey.Bytes())
	log.Println("Sent new client's public key to key hub")
	// Each client calculates the PSK

	// clients[currentClient] = true
	// clients[newClient] = true
	keyHub.DHDone = true
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("handle connections:", err)
		return
	}
	defer conn.Close()

	client := &Client{conn: conn, isKeyHub: false, DHDone: false}

	if len(clients) == 0 {
		log.Println("Setting key hub")
		setKeyHub(client)
	}

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

	// When there are two clients connecting, do the key exchange
	if len(clients) == 2 {
		log.Println(("We have enough clients for a key exchange! What an exciting time!"))
		log.Println("Doing key exchange!")
		negotiateKeys(client, keyHub)
		client.DHDone = true
	}
	log.Println("We left the if block for some reason")

	for {
		// if client.isKeyHub {
		// 	log.Println("Checking messages again")
		// }
		if client.DHDone {
			if client.isKeyHub {
				log.Println("You should not see this message during a key exchange. If you do, something went wrong.")
			}
			var msg comm.Message
			// if client.isKeyHub {
			// 	log.Println("Reading message from key hub")
			// } else {
			// 	log.Println("Reading message from nonhub")
			// }
			err := conn.ReadJSON(&msg)
			if err != nil {
				delete(clients, client)
				if client.isKeyHub {
					fmt.Println("read messages:", err)
					log.Println("Key hub disconnected")
					keyHub = nil
				}
				return
			}

			if msg.Type == comm.Text {
				// log.Println("There was a new message")
				if client.isKeyHub {
					log.Println("Message received:", msg)
				}
				messageEvent := MessageEvent{message: msg, client: client}
				broadcast <- messageEvent
			}

			if msg.Type == comm.Info {
				if msg.Message == "ke" {
					log.Println("Key hub needs to do a key exchange")
					client.DHDone = false
				}
			}
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
		// fmt.Println("Message:", msg.message)
		// fmt.Println("Decrypted message:", decryptedMessage)

		for client := range clients {
			var err error
			if client != msg.client {
				err = client.WriteJSON(msg.message)
			}
			if err != nil {
				fmt.Println("handle messages:", err)
				client.Disconnect()
				delete(clients, client)
			}
		}
	}
}
