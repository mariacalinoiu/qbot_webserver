package handlers

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	cryptoRand "crypto/rand"
	"encoding/hex"
	"io"
	"math/rand"
	"time"
	"unsafe"
)

const (
	encryptionPassphrase = "encryptionPass"

	letterBytes   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	letterIdxBits = 6
	letterIdxMask = 1<<letterIdxBits - 1
	letterIdxMax  = 63 / letterIdxBits
)

func GenerateToken(n int) string {
	src := rand.NewSource(time.Now().UnixNano())
	b := make([]byte, n)
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return *(*string)(unsafe.Pointer(&b))
}

func EncryptPassword(password string) (string, error) {
	password, err := encrypt([]byte(password), encryptionPassphrase)
	if err != nil {
		return "", err
	}

	return password, nil
}

func DecryptPassword(password string) (string, error) {
	password, err := decrypt([]byte(password), encryptionPassphrase)
	if err != nil {
		return "", err
	}

	return password, nil
}

func encrypt(data []byte, passphrase string) (string, error) {
	block, _ := aes.NewCipher([]byte(createHash(passphrase)))
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(cryptoRand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	return string(ciphertext), nil
}

func decrypt(data []byte, passphrase string) (string, error) {
	key := []byte(createHash(passphrase))
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func createHash(key string) string {
	hasher := md5.New()
	hasher.Write([]byte(key))
	return hex.EncodeToString(hasher.Sum(nil))
}
