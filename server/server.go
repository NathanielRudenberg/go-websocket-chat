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

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type MessageEvent struct {
	message   comm.Message
	client    *serverclient.Client
	recipient *serverclient.Client
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var (
	clients            = make(map[*serverclient.Client]bool)
	ids                = make(map[string]*serverclient.Client)
	broadcast          = make(chan MessageEvent)
	P         *big.Int = util.GeneratePrime()
	G                  = big.NewInt(2)
	keyHub    *serverclient.Client
)

func main() {
	// Each connection only supports one goroutine for Read and one for Write
	// Client connects to "connect" endpoint (functions as a key exchange request)

	// ??? Key hub opens new connection to server, exchanges keys with new client on that connection, closes the connection
	// Client performs key exchange and whatever with key hub
	// Client connects to chat endpoint (currently /ws)
	// Profit????? I guess?

	// Hold a queue of incoming clients
	// This queue might need a mutex, idk yet
	// When a client tries to connect, add it to the queue
	// Open a new connection from the key hub
	// The key hub locks the mutex, pulls a client from the queue, unlocks the mutex, exchanges keys with the client, then shares the room key
	// The key hub closes the connection
	// The client connects to the chat endpoint, now able to send encrypted messages

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
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("handle join:", err)
		return
	}
	defer conn.Close()
	client := &serverclient.Client{Conn: conn}

	// Get client ID
	var joinMessage comm.Message
	err = client.Conn.ReadJSON(&joinMessage)
	if err != nil {
		log.Println("handle join: get id:", err)
		return
	}

	if joinMessage.Type == comm.Info && joinMessage.Message == "join" {
		clientId := (*uuid.UUID)(joinMessage.Data)
		clientIdString := clientId.String()
		ids[clientIdString] = client
		err = ids[clientIdString].WriteJSON(comm.Message{Username: "server", Message: "join-chat", Type: comm.Command})
		if err != nil {
			log.Println("handle join: send join chat command:", err)
			delete(ids, clientIdString)
			return
		}
		ids[clientIdString].Conn = nil
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("handle connections:", err)
		return
	}
	defer conn.Close()

	// Get client ID
	var joinMessage comm.Message
	err = conn.ReadJSON(&joinMessage)
	if err != nil {
		log.Println("handle connections: get id:", err)
		return
	}
	var clientId *uuid.UUID
	if joinMessage.Type == comm.Info && joinMessage.Message == "join" {
		clientId = (*uuid.UUID)(joinMessage.Data)
	} else {
		fmt.Println("Invalid join message")
		return
	}

	clientIdString := clientId.String()
	client := ids[clientIdString]
	client.Conn = conn
	clients[client] = true

	// When there are two clients connecting, do the key exchange
	if keyHub == nil {
		setKeyHub(client)
		// newMessageForKeyHub := comm.Message{Username: "server", Message: "Ayo you are the key hub", Type: comm.Text}
		// messageEvent := MessageEvent{message: newMessageForKeyHub, recipient: keyHub}
		// broadcast <- messageEvent
	} else {
		// Send a message to key hub to open new connection?
		// Make key hub channel for the new connection?
		// newMessageForKeyHub := comm.Message{Username: "server", Message: "Ayo you are the key hub AND someone new has joined", Type: comm.Text}
		// messageEvent := MessageEvent{message: newMessageForKeyHub, recipient: keyHub}
		// broadcast <- messageEvent
	}

	for {
		// listenMessages(conn, client)

		var msg comm.Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			delete(clients, client)
			if client.IsKeyHub() {
				// fmt.Println("read messages:", err)
				log.Println("Key hub disconnected")
				// Choose new key hub
				chooseNewKeyHub()
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
	}
}

func handleMessages() {
	for {
		msgEvent := <-broadcast
		if msgEvent.recipient != nil {
			err := msgEvent.recipient.WriteJSON(msgEvent.message)
			if err != nil {
				fmt.Println("handle messages:", err)
				msgEvent.recipient.Disconnect()
				delete(clients, msgEvent.recipient)
			}
		} else {
			for client := range clients {
				var err error
				if client != msgEvent.client {
					err = client.WriteJSON(msgEvent.message)
				}
				if err != nil {
					fmt.Println("handle messages:", err)
					client.Disconnect()
					delete(clients, client)
				}
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
