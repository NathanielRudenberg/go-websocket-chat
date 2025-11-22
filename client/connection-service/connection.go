package connectionservice

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/url"
	"time"

	messageservice "websocket-chat/client/message-service"
	"websocket-chat/comm"
	"websocket-chat/util"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var (
	username  string
	hostName  *string
	hostPort  *int
	id        = uuid.New()
	broadcast = make(chan comm.Message)
	chatInput = make(chan string)
)

func SendChat(message string) {
	chatInput <- message
}

func BroadcastMessage(message string) error {
	encryptedMessage, err := util.Encrypt([]byte(message), util.GetRoomKey())
	if err != nil {
		log.Println("encryption:", err)
		newError := errors.New("Error encrypting message:" + err.Error())
		return newError
	}

	writeMsg := comm.Message{Username: username, Message: encryptedMessage, Type: comm.Text}
	broadcast <- writeMsg
	return nil
}

func ConnectToChatServer(chatChannel *chan string, closeChannel *chan struct{}, stopApp func()) {
	// interrupt := make(chan os.Signal, 1)
	// signal.Notify(interrupt, os.Interrupt)

	hostName = flag.String("host", "localhost", "Server Hostname")
	hostPort = flag.Int("port", 8080, "Server Port")
	user := flag.String("username", "PabloDebug", "Username")
	flag.Parse()

	username = *user

	u := url.URL{Scheme: "ws", Host: fmt.Sprintf("%s:%d", *hostName, *hostPort), Path: "/ws"}

	conn, response, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Printf("handshake failed with status %d", response.StatusCode)
		log.Fatal("dial:", err)
	}
	defer conn.Close()
	messageservice.SetHostInfo(hostName, hostPort, &broadcast, &id)

	done := make(chan struct{})
	connectionHandler := func() {
		defer close(done)

		for {
			var msg comm.Message
			err := conn.ReadJSON(&msg)
			if err != nil {
				log.Println("read:", err)
				stopApp()
				return
			}

			if msg.Type == comm.Text {
				// err := msg.Print()
				decryptedMessage, err := msg.GetDecryptedMessage()
				// fmt.Println(msg)
				if err != nil {
					log.Println("decryption:", err)
					continue
				}
				*chatChannel <- decryptedMessage
			}

			if msg.Type == comm.Command {
				messageservice.HandleCommand(&msg)
			}

			if msg.Type == comm.Info {
				messageservice.HandleInfo(&msg, conn)
			}
		}
	}

	inputHandler := func() {
		// First message to send upon connection is the uuid
		uuidBinary, err := id.MarshalBinary()
		if err != nil {
			log.Println("marshal uuid:", err)
			return
		}
		firstJoinMessage := comm.Message{Username: username, Message: "join", Type: comm.Info, Data: uuidBinary}
		broadcast <- firstJoinMessage
		for {
			chatMessage := <-chatInput
			BroadcastMessage(chatMessage)
		}
	}

	go connectionHandler()
	go inputHandler()

	for {
		select {
		case <-done:
			log.Printf("done")
			return
		case m := <-broadcast:
			err := conn.WriteJSON(m)
			if err != nil {
				log.Println("write:", err)
				return
			}
		case <-*closeChannel:
			log.Println("close")
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
