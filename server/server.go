package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"time"

	"websocket-chat/comm"
	serverclient "websocket-chat/server/serverClient"
	"websocket-chat/util"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var (
	clients = make(map[*serverclient.Client]bool)
	ids     = make(map[string]*serverclient.Client)
	P       = util.GeneratePrime()
	G       = big.NewInt(2)
	rdb     = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
		Protocol: 2,
	})
	ctx            = context.Background()
	ChatChannel    = "global_chat"
	KeyHubRedisKey = "chatroom-key-hub-id"
)

func main() {
	hostPort := flag.Int("port", 8080, "Server Port")
	flag.Parse()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}
	log.Println("Connected to Redis")
	http.HandleFunc("/", homePage)
	http.HandleFunc("/ws", handleConnections)

	go handleMessages()

	fmt.Printf("Server started on :%s\n", fmt.Sprint(*hostPort))
	err = http.ListenAndServe(fmt.Sprintf(":%d", *hostPort), nil)
	if err != nil {
		panic("Error starting server: " + err.Error())
	}
}

// func setKeyHub(client *serverclient.Client) {
// 	client.SetIsKeyHub(true)
// 	keyHub = client
// }
//
// func chooseNewKeyHub() {
// 	keyHub = nil
// 	for c := range clients {
// 		setKeyHub(c)
// 		break
// 	}
// }

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

	for {
		success, err := rdb.SetNX(ctx, KeyHubRedisKey, clientIdString, 0).Result()
		if err != nil {
			log.Printf("Error setting key hub in Redis: %v", err)
			// Try again
			time.Sleep(100 * time.Millisecond)
			continue
		}

		if success {
			client.SetIsKeyHub(true)

			makeKeysMessage := comm.Message{
				Username:    "server",
				Message:     "generate-keys",
				Type:        comm.Command,
				Destination: clientIdString,
			}

			payloadMsg, _ := json.Marshal(makeKeysMessage)
			if err := rdb.Publish(ctx, ChatChannel, payloadMsg).Err(); err != nil {
				log.Printf("Error publishing 'generate-keys' command to Redis: %v", err)
			}
			// Done! Leave the loop.
			break
		} else {
			currentKeyHub, err := rdb.Get(ctx, KeyHubRedisKey).Result()
			if err == redis.Nil {
				/* Key hub left right after we checked for it. Loop again. */
				continue
			} else if err != nil {
				log.Printf("Redis error fetching Key Hub: %v", err)
				time.Sleep(100 * time.Millisecond)
				continue
			} else {
				// Found a key hub. Request keys from it.
				message := comm.Message{
					Username:    "server",
					Message:     "exchange-keys",
					Type:        comm.Command,
					Data:        []byte(clientIdString),
					Destination: currentKeyHub,
				}
				payloadMsg, _ := json.Marshal(message)
				if err := rdb.Publish(ctx, ChatChannel, payloadMsg).Err(); err != nil {
					log.Printf("Error publishing 'exchange-keys' command to Redis: %v", err)
				}
				// Done! Leave the loop.
				break
			}
		}
	}

	for {
		var msg comm.Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			delete(clients, client)
			delete(ids, clientIdString)
			if client.IsKeyHub() {
				rdb.Del(ctx, KeyHubRedisKey)

				electionMsg := comm.Message{
					Username: "server",
					Message:  "elect-new-key-hub",
					Type:     comm.Command,
				}
				payloadMsg, _ := json.Marshal(electionMsg)
				rdb.Publish(ctx, ChatChannel, payloadMsg)
			}
			return
		}

		if msg.Type == comm.Text || msg.Type == comm.Command || msg.Type == comm.Info {
			if msg.Type == comm.Text {
				msg.SenderID = clientIdString
			}

			payloadBytes, err := json.Marshal(msg)
			if err != nil {
				log.Printf("Marshal error: %v", err)
				continue
			}
			if err := rdb.Publish(ctx, ChatChannel, payloadBytes).Err(); err != nil {
				log.Printf("Error publishing to Redis: %v", err)
			}
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
	// Subscribe to the Redis channel
	pubsub := rdb.Subscribe(ctx, ChatChannel)
	defer pubsub.Close()

	redisChannel := pubsub.Channel()

	// Loop over channel to listen for messages
	for msg := range redisChannel {
		var chatMessage comm.Message

		// Deserialize the Redis message
		if err := json.Unmarshal([]byte(msg.Payload), &chatMessage); err != nil {
			log.Printf("Error unmarshalling message from Redis: %v", err)
			continue
		}

		// Handle server commands
		if chatMessage.Type == comm.Command && chatMessage.Message == "elect-new-key-hub" {
			if len(ids) > 0 {
				for candidateID, candidateClient := range ids {
					success, err := rdb.SetNX(ctx, KeyHubRedisKey, candidateID, 0).Result()
					if err != nil {
						log.Printf("Election error: %v", err)
						break
					}

					if success {
						candidateClient.SetIsKeyHub(true)
					}
					break
				}
			}
			continue
		}

		// Unicast messaging
		if chatMessage.Destination != "" {
			recipient := ids[chatMessage.Destination]

			if recipient != nil {
				err := recipient.WriteJSON(chatMessage)
				if err != nil {
					fmt.Println("handleMessages:", err)
					recipient.Disconnect()
					delete(clients, recipient)
					delete(ids, chatMessage.Destination)
					if recipient.IsKeyHub() {
						rdb.Del(ctx, KeyHubRedisKey)
					}
				}
			}
			continue
		}

		// Broadcast messaging
		for clientID, client := range ids {
			if chatMessage.SenderID != "" && clientID == chatMessage.SenderID {
				continue
			}

			err := client.WriteJSON(chatMessage)
			if err != nil {
				log.Println("handleMessages:", err)
				client.Disconnect()
				delete(clients, client)
				delete(ids, clientID)
			}
		}
	}
}
