package main

import (
	"bufio"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"
	"websocket-chat/chat"

	"github.com/gorilla/websocket"
)

var username string

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

			fmt.Println(msg)
		}
	}

	writeHandler := func() {
		for {
			// fmt.Print("You: ")
			writeMsg := chat.Message{Username: username, Message: ""}
			writeMsg.Message, _ = reader.ReadString('\n')
			msgLength := len(writeMsg.Message)
			writeMsg.Message = writeMsg.Message[:msgLength-1]
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
