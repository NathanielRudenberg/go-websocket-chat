package main

import (
	"fmt"
	"net/http"
	"websocket-chat/chat"

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
	clients   = make(map[*websocket.Conn]bool)
	broadcast = make(chan MessageEvent)
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

func handleConnections(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()

	clients[conn] = true

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
		fmt.Println("Message:", msg.message)

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
