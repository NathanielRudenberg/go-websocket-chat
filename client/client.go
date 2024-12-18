package main

import (
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

// type Message struct {
// 	Username string `json:"username"`
// 	Message  string `json:"message"`
// }

func main() {
	log.Print("program running")
	message := make(chan string)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	// go handleMessages()

	u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws"}
	log.Printf("connecting to %s", u.String())

	conn, response, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Printf("handshake failed with status %d", response.StatusCode)
		log.Fatal("dial:", err)
	}

	defer conn.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)

		log.Print("bout to loop through messages")
		for {
			log.Print("bout to read messages")
			_, msg, err := conn.ReadMessage()
			log.Printf("read message: %s", string(msg))
			if err != nil {
				log.Println("read:", err)
				return
			}

			log.Printf("recv: %s", msg)
			if string(msg) == "Connected" {
				log.Printf("Successfully connected")
				// TODO idk send message to channel or something
				message <- "Pablo: compramos carros y camionetas viejas para desarmar"
			}
			log.Print("we read messages")
		}
	}()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	log.Printf("We made the ticker!")

	for {
		select {
		case <-done:
			log.Printf("done")
			return
		case m := <-message:
			log.Printf("Send Message %s", m)
			err := conn.WriteJSON(m)
			if err != nil {
				log.Println("write:", err)
				return
			}

		case t := <-ticker.C:
			err := conn.WriteMessage(websocket.TextMessage, []byte(t.String()))
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

func handleMessages() {
	// get handled, idiot
}
