package util

// https://www.codingexplorations.com/blog/understanding-encryption-in-go-a-developers-guide
// https://eminmuhammadi.com/articles/diffiehellman-key-exchange-example-in-golang

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/url"

	"github.com/gorilla/websocket"
)

var (
	P, G, privateKey, publicKey *big.Int
	roomKey                     []byte
)

// Clear the current line in the terminal after pressing return
func ClearLine() {
	fmt.Print("\033[1A\033[K")
}

func ClearTerminal() {
	fmt.Print("\033[H\033[2J")
}

func GenerateKeys() {
	P = GeneratePrime()
	G = big.NewInt(2)
	GeneratePrivateKey()
	CalculatePublicKey(G)
	checkRoomKey()
}

func checkRoomKey() {
	if roomKey == nil {
		roomKey = make([]byte, 32)
		_, err := rand.Read(roomKey)
		if err != nil {
			log.Println("room key:", err)
		}
	}
}

func Encrypt(plaintext []byte, key []byte) (string, error) {
	// key, _ := hex.DecodeString(keyString) // Convert the key to bytes
	// key := []byte(keyString)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize] // Initialization vector
	if _, err = io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	// Return the encoded hex string
	return hex.EncodeToString(ciphertext), nil
}

func Decrypt(ciphertext string, key []byte) ([]byte, error) {
	// key, _ := hex.DecodeString(keyString)
	ciphertextBytes, _ := hex.DecodeString(ciphertext)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(ciphertextBytes) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}

	iv := ciphertextBytes[:aes.BlockSize]
	ciphertextBytes = ciphertextBytes[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertextBytes, ciphertextBytes)

	return ciphertextBytes, nil
}

// Needs to generate a modulus used by both server and client
func GeneratePrime() *big.Int {
	prime, _ := rand.Prime(rand.Reader, 256)
	return prime
}

// Both parties generate private keys
func GeneratePrivateKey() {
	privateKey, _ = rand.Int(rand.Reader, P)
}

// Both parties calculate their own public keys
func CalculatePublicKey(base *big.Int) {
	publicKey = new(big.Int).Exp(base, privateKey, P)
}

// Parties exchange public keys and use them to calculate shared secret
func CalculateSharedSecret(publicKeyRemote *big.Int) *big.Int {
	sharedSecret := new(big.Int).Exp(publicKeyRemote, privateKey, P)
	return sharedSecret
}

func DoKeyExchange(conn *websocket.Conn) error {
	// Receive P, G, key hub public key from server
	_, Pbytes, err := conn.ReadMessage()
	if err != nil {
		newError := errors.New("Error receiving P from server:" + err.Error())
		return newError
	}
	P = new(big.Int).SetBytes(Pbytes)

	_, Gbytes, err := conn.ReadMessage()
	if err != nil {
		newError := errors.New("Error receiving G from server:" + err.Error())
		return newError
	}
	G = new(big.Int).SetBytes(Gbytes)

	_, keyHubPubKeyBytes, err := conn.ReadMessage()
	if err != nil {
		newError := errors.New("Error receiving key hub's public key:" + err.Error())
		return newError
	}
	keyHubPubKey := new(big.Int).SetBytes(keyHubPubKeyBytes)

	// Calculate private key
	GeneratePrivateKey()

	// Calculate public key
	CalculatePublicKey(G)

	// Send public key to server
	err = conn.WriteMessage(websocket.BinaryMessage, publicKey.Bytes())
	if err != nil {
		newError := errors.New("Error sending public key to server:" + err.Error())
		return newError
	}
	// Calculate PSK with server pub key
	psk := CalculateSharedSecret(keyHubPubKey)

	// Receive room key from server
	_, encryptedRoomKeyBytes, err := conn.ReadMessage()
	if err != nil {
		newError := errors.New("Error receiving room key from server:" + err.Error())
		return newError
	}

	roomKey, err = Decrypt(string(encryptedRoomKeyBytes), psk.Bytes())
	if err != nil {
		newError := errors.New("Error decrypting room key:" + err.Error())
		return newError
	}
	return nil
}

func ShareKeys(hostName *string, hostPort *int) error {
	u := url.URL{Scheme: "ws", Host: fmt.Sprintf("%s:%d", *hostName, *hostPort), Path: "/key-exchange"}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal(err)
		return err
	}
	defer conn.Close()

	// Send P, G, public key to server
	err = conn.WriteMessage(websocket.BinaryMessage, P.Bytes())
	if err != nil {
		newError := errors.New("Error sending P to server:" + err.Error())
		return newError
	}

	err = conn.WriteMessage(websocket.BinaryMessage, G.Bytes())
	if err != nil {
		newError := errors.New("Error sending G to server:" + err.Error())
		return newError
	}

	err = conn.WriteMessage(websocket.BinaryMessage, publicKey.Bytes())
	if err != nil {
		newError := errors.New("Error sending public key to server:" + err.Error())
		return newError
	}

	// Receive other client's public key
	_, clientPubKeyBytes, err := conn.ReadMessage()
	if err != nil {
		newError := errors.New("Error receiving client's public key:" + err.Error())
		return newError
	}
	clientPubKey := new(big.Int).SetBytes(clientPubKeyBytes)

	// Calculate PSK
	psk := CalculateSharedSecret(clientPubKey)

	// Send encrypted room key
	encryptedRoomKey, err := Encrypt(roomKey, psk.Bytes())
	if err != nil {
		newError := errors.New("Error encrypting room key:" + err.Error())
		return newError
	}
	err = conn.WriteMessage(websocket.BinaryMessage, []byte(encryptedRoomKey))
	if err != nil {
		newError := errors.New("Error sending room key to server:" + err.Error())
		return newError
	}
	return nil
}

func GetRoomKey() []byte {
	return roomKey
}