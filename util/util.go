package util

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"math/big"
)

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
func GeneratePrivateKey(prime *big.Int) *big.Int {
	privateKey, _ := rand.Int(rand.Reader, prime)
	return privateKey
}

// Both parties calculate their own public keys
func CalculatePublicKey(prime, privateKey, base *big.Int) *big.Int {
	publicKey := new(big.Int).Exp(base, privateKey, prime)
	return publicKey
}

// Parties exchange public keys and use them to calculate shared secret
func CalculateSharedSecret(prime, privateKey, publicKey *big.Int) *big.Int {
	sharedSecret := new(big.Int).Exp(publicKey, privateKey, prime)
	return sharedSecret
}
