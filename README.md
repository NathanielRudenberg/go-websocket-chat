# Go Websocket Chat

![preview](https://github.com/user-attachments/assets/a044bd91-e015-48d5-9428-7ed20ecf9205)

## A simple TCP websocket and encryption demo

This is a very simple console-based chat program. It supports an arbitrary number of simultaneous client connections and features end-to-end encryption between all clients. The code is divided between the server and the client. The server supports one chatroom.

While the chat is encrypted, it is definitely not secure. This was a quick project just to get something working; I did not have security in mind. Any number of attacks could theoretically compromise the messages. If I truly wanted this to be unbreachable, maybe I would implement something like the Signal protocol.


### Inital setup
After cloning the repo, `cd` into the project directory and set up the Go modules.
```console
foo@bar:~$ git clone https://github.com/NathanielRudenberg/go-websocket-chat.git
foo@bar:~$ cd go-websocket-chat
foo@bar:~/go-websocket-chat$ go mod download
```

### Running the server
```console
foo@bar:~/go-websocket-chat$ cd server
foo@bar:~/go-websocket-chat/server$ go run .
```
The default port is 8080. To specify a port to listen on:
```console
foo@bar:~/go-websocket-chat/server$ go run . -port <port number>
```

### Running the client
```console
foo@bar:~/go-websocket-chat$ cd client
foo@bar:~/go-websocket-chat/client$ go run . -username <chat username>
```
The default host is `localhost:8080`. To specify a different host or port, use the `-host` and `-port` options:
```console
foo@bar:~/go-websocket-chat/client$ go run . -username <chat username> -host <hostname> -port <port-number>
```
