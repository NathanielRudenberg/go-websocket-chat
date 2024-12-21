package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"sync"
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
	clients         = make(map[*serverclient.Client]bool)
	incomingClients = make(map[*serverclient.Client]bool)
	ids             = make(map[string]*serverclient.Client)
	broadcast       = make(chan MessageEvent)
	P               = util.GeneratePrime()
	G               = big.NewInt(2)
	keyHub          *serverclient.Client
	mu              sync.Mutex
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
	// When a client tries to connect: lock the mutex, add it to the queue, unlock the mutex
	// Open a new connection from the key hub
	// The key hub locks the mutex, pulls a client from the queue, unlocks the mutex, exchanges keys with the client, then shares the room key
	// The key hub closes the connection
	// The client connects to the chat endpoint, now able to send encrypted messages

	hostPort := flag.Int("port", 8080, "Server Port")
	flag.Parse()
	http.HandleFunc("/", homePage)
	http.HandleFunc("/ws", handleConnections)
	http.HandleFunc("/connect", handleJoin)
	http.HandleFunc("/key-exchange", handleKeyExchange) // The key hub connects here to exchange keys with new clients

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

func negotiateKeys(newClient *serverclient.Client, keyHubConnection *websocket.Conn) error {
	// Tell hub and new client to exchange keys
	if keyHub.DHDone {
		log.Println("telling key hub to exchange keys")
		keyHub.WriteJSON(comm.Message{Username: "server", Message: "ke", Type: comm.Info})
	}
	serverclient.SendCommand(keyHubConnection, "exchange-keys")
	newClient.SendCommand("exchange-keys")
	// Hub sends P and G to both clients
	// Send P  and G to key hub
	serverclient.WriteBinaryMessage(keyHubConnection, P.Bytes())
	serverclient.WriteBinaryMessage(keyHubConnection, G.Bytes())
	// Send P and G to new client
	newClient.WriteBinaryMessage(P.Bytes())
	newClient.WriteBinaryMessage(G.Bytes())
	// Both calculate their private and public keys
	// Each client sends its public key to the other
	// Receive new client's public key
	_, newClientPubKeyBytes, err := newClient.ReadMessage()
	if err != nil {
		newError := errors.New("Error receiving new client's public key:" + err.Error())
		newClient.Disconnect()
		return newError
	}
	newClientPubKey := new(big.Int).SetBytes(newClientPubKeyBytes)
	// Receive key hub's public key
	_, oldClientPubKeyBytes, err := keyHubConnection.ReadMessage()
	if err != nil {
		newError := errors.New("Error receiving key hub's public key:" + err.Error())
		// keyHubConnection.Disconnect()
		newClient.Disconnect()
		return newError
	}
	oldClientPubKey := new(big.Int).SetBytes(oldClientPubKeyBytes)
	// Send key hub's public key to new client
	newClient.WriteBinaryMessage(oldClientPubKey.Bytes())
	// Send new client's public key to key hub
	serverclient.WriteBinaryMessage(keyHubConnection, newClientPubKey.Bytes())
	// Each client calculates the PSK
	// Presumably, at this point, the key hub would send the shared room key to the new client. If it has one.
	serverclient.SendCommand(keyHubConnection, "share-room-key")
	_, roomKey, err := keyHubConnection.ReadMessage()
	if err != nil {
		log.Println("kh room key recv:", err)
		newError := errors.New("Error receiving room key from key hub:" + err.Error())
		// keyHubConnection.Disconnect()
		return newError
	}
	newClient.WriteJSON(comm.Message{Username: "server", Message: "rk", Data: roomKey, Type: comm.Info})
	return nil
}

func handleKeyExchange(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("handle join:", err)
		return
	}
	defer conn.Close()

	mu.Lock()
	var incomingClient *serverclient.Client
	for client := range incomingClients {
		incomingClient = client
		delete(incomingClients, client)
		break
	}
	mu.Unlock()
	_ = incomingClient

	// negotiateKeys(incomingClient, conn)
}

func handleJoin(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("handle join:", err)
		return
	}
	defer conn.Close()
	client := &serverclient.Client{Conn: conn}
	mu.Lock()
	incomingClients[client] = true
	mu.Unlock()

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

	// If there is a key hub, do key exchange
	if keyHub != nil {
		exchangeKeys := comm.Message{Username: "server", Message: "exchange-keys", Type: comm.Command}
		messageEvent := MessageEvent{message: exchangeKeys, recipient: keyHub}
		broadcast <- messageEvent
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

	if keyHub == nil {
		mu.Lock()
		delete(incomingClients, client)
		mu.Unlock()
		setKeyHub(client)
		makeKeysMessage := comm.Message{Username: "server", Message: "generate-keys", Type: comm.Command}
		messageEvent := MessageEvent{message: makeKeysMessage, recipient: keyHub}
		broadcast <- messageEvent
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
