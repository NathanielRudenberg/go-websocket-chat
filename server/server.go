package main

import (
	"flag"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"websocket-chat/comm"
	serverclient "websocket-chat/server/serverClient"
	"websocket-chat/util"

	"github.com/gorilla/websocket"
)

type MessageEvent struct {
	message comm.Message
	client  *serverclient.Client
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var (
	clients            = make(map[*serverclient.Client]bool)
	broadcast          = make(chan MessageEvent)
	P         *big.Int = util.GeneratePrime()
	G                  = big.NewInt(2)
	keyHub    *serverclient.Client
)

func main() {
	// Each connection only supports one goroutine for Read and one for Write
	// Client connects to "connect" endpoint (functions as a key exchange request)

	hostPort := flag.Int("port", 8080, "Server Port")
	flag.Parse()
	http.HandleFunc("/", homePage)
	http.HandleFunc("/ws", handleConnections)
	http.HandleFunc("/connect", handleJoin)

	go handleMessages()

	fmt.Printf("Server started on :%s\n", fmt.Sprint(*hostPort))
	err := http.ListenAndServe(fmt.Sprintf(":%d", *hostPort), nil)
	if err != nil {
		panic("Error starting server: " + err.Error())
	}
}

func setKeyHub(client *serverclient.Client) {
	client.SetIsKeyHub(true)
	keyHub = client
}

func chooseNewKeyHub() {
	keyHub = nil
	for c := range clients {
		setKeyHub(c)
		break
	}
}

func homePage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Pablo")
}

func negotiateKeys(newClient *serverclient.Client, keyHub *serverclient.Client) {
	// Tell hub and new client to exchange keys
	if keyHub.DHDone {
		log.Println("telling key hub to exchange keys")
		keyHub.WriteJSON(comm.Message{Username: "server", Message: "ke", Type: comm.Info})
	}
	keyHub.SendCommand("exchange-keys")
	newClient.SendCommand("exchange-keys")
	// Hub sends P and G to both clients
	// Send P  and G to key hub
	keyHub.WriteBinaryMessage(P.Bytes())
	keyHub.WriteBinaryMessage(G.Bytes())
	// Send P and G to new client
	newClient.WriteBinaryMessage(P.Bytes())
	newClient.WriteBinaryMessage(G.Bytes())
	// Both calculate their private and public keys
	// Each client sends its public key to the other
	// Receive new client's public key
	_, newClientPubKeyBytes, err := newClient.ReadMessage()
	if err != nil {
		log.Println("nc pub key recv:", err)
		newClient.Disconnect()
		return
	}
	newClientPubKey := new(big.Int).SetBytes(newClientPubKeyBytes)
	// Receive key hub's public key
	_, oldClientPubKeyBytes, err := keyHub.ReadMessage()
	if err != nil {
		log.Println("kh pub key recv:", err)
		keyHub.Disconnect()
		chooseNewKeyHub()
		newClient.Disconnect()
		return
	}
	oldClientPubKey := new(big.Int).SetBytes(oldClientPubKeyBytes)
	// Send key hub's public key to new client
	newClient.WriteBinaryMessage(oldClientPubKey.Bytes())
	// Send new client's public key to key hub
	keyHub.WriteBinaryMessage(newClientPubKey.Bytes())
	// Each client calculates the PSK
	// Presumably, at this point, the key hub would send the shared room key to the new client. If it has one.
	keyHub.SendCommand("share-room-key")
	_, roomKey, err := keyHub.ReadMessage()
	if err != nil {
		log.Println("kh room key recv:", err)
		keyHub.Disconnect()
		chooseNewKeyHub()
		return
	}
	newClient.WriteJSON(comm.Message{Username: "server", Message: "rk", Data: roomKey, Type: comm.Info})
	keyHub.DHDone = true
}

func handleJoin(w http.ResponseWriter, r *http.Request) {
	// ??? Key hub opens new connection to server, exchanges keys with new client on that connection, closes the connection
	// Client performs key exchange and whatever with key hub
	// Client connects to chat endpoint (currently /ws)
	// Profit????? I guess?
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("handle join:", err)
		return
	}
	defer conn.Close()
	log.Println("New client wants to join")

}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("handle connections:", err)
		return
	}
	defer conn.Close()

	client := &serverclient.Client{Conn: conn, DHDone: false}
	clients[client] = true

	// When there are two clients connecting, do the key exchange
	if keyHub == nil {
		setKeyHub(client)
	}

	for {
		// if client.DHDone {
		// listenMessages(conn, client)

		var msg comm.Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			delete(clients, client)
			if client.IsKeyHub() {
				fmt.Println("read messages:", err)
				log.Println("Key hub disconnected")
				// Choose new key hub
				// chooseNewKeyHub()
			} else {
				log.Println("read: non kh client disconnected")
			}
			return
		}

		if msg.Type == comm.Text {
			messageEvent := MessageEvent{message: msg, client: client}
			broadcast <- messageEvent
		}

		if msg.Type == comm.Info {
			if msg.Message == "ke" {
				log.Println("Key hub needs to do a key exchange")
				client.DHDone = false
			}
		}
		// }
	}
}

func handleMessages() {
	for {
		msg := <-broadcast
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

// func listenMessages(conn *websocket.Conn, client *serverclient.Client) {
// 	var msg comm.Message
// 	// if client.isKeyHub {
// 	// 	log.Println("Reading message from key hub")
// 	// } else {
// 	// 	log.Println("Reading message from nonhub")
// 	// }
// 	err := conn.ReadJSON(&msg)
// 	if err != nil {
// 		delete(clients, client)
// 		if client.IsKeyHub() {
// 			fmt.Println("read messages:", err)
// 			log.Println("Key hub disconnected")
// 			// Choose new key hub
// 			// chooseNewKeyHub()
// 		} else {
// 			log.Println("read: non kh client disconnected")
// 		}
// 		return
// 	}

// 	if msg.Type == comm.Text {
// 		// if client.isKeyHub {
// 		// 	log.Println("Message received:", msg)
// 		// }
// 		messageEvent := MessageEvent{message: msg, client: client}
// 		broadcast <- messageEvent
// 	}

// 	if msg.Type == comm.Info {
// 		if msg.Message == "ke" {
// 			log.Println("Key hub needs to do a key exchange")
// 			client.DHDone = false
// 		}
// 	}
// }
