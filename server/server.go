package main

import (
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
	clients = make(map[*serverclient.Client]bool)
	// incomingClients = make(map[*serverclient.Client]bool)
	ids       = make(map[string]*serverclient.Client)
	broadcast = make(chan MessageEvent)
	P         = util.GeneratePrime()
	G         = big.NewInt(2)
	keyHub    *serverclient.Client
	mu        sync.Mutex
)

func main() {
	hostPort := flag.Int("port", 8080, "Server Port")
	flag.Parse()
	http.HandleFunc("/", homePage)
	http.HandleFunc("/ws", handleConnections)
	// http.HandleFunc("/connect", handleJoin)
	// http.HandleFunc("/key-exchange", handleKeyExchange) // The key hub connects here to exchange keys with new clients

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
	fmt.Fprint(w, "server is running")
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
		conn.Close()
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
	client := &serverclient.Client{Conn: conn}
	clients[client] = true
	ids[clientIdString] = client

	if keyHub == nil {
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

		message := comm.Message{Username: "server", Message: "exchange-keys", Type: comm.Command, Data: []byte(clientIdString)}
		messageEvent := MessageEvent{message: message, recipient: keyHub}
		broadcast <- messageEvent
	}

	for {
		// listenMessages(conn, client)

		var msg comm.Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			delete(clients, client)
			delete(ids, clientIdString)
			if client.IsKeyHub() {
				// Choose new key hub
				chooseNewKeyHub()
			}
			return
		}

		if msg.Type == comm.Text || msg.Type == comm.Command {
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

		// Handle unicast messaging
		if msgEvent.message.Destination != "" {
			// Find the client based on UUID
			recipient := ids[msgEvent.message.Destination]

			if recipient != nil {
				// Send the message directly to the recipient
				err := recipient.WriteJSON(msgEvent.message)
				if err != nil {
					fmt.Println("handleMessages:", err)
					recipient.Disconnect()
					delete(clients, recipient)
					delete(ids, msgEvent.message.Destination)
					if recipient.IsKeyHub() {
						chooseNewKeyHub()
					}
				}
			}
			// Skip broadcast because it's a unicast message
			continue
		}

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
