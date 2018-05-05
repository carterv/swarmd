package authentication

import (
	"crypto/sha256"
	"swarmd/packets"
	"crypto/aes"
	"crypto/cipher"
	"io"
	"crypto/rand"
	"log"
	"errors"
)

func MakeKey(seed string) [32]byte {
	return sha256.Sum256([]byte(seed))
}

func encrypt(raw []byte, key []byte) ([]byte, error) {
	c, err := aes.NewCipher(key)
	if err != nil {
		log.Print("Cipher creation failed during encryption")
		return nil, err
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		log.Print("GCM failed during encryption")
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		log.Print("Nonce creation failed during encryption")
		return nil, err
	}

	return gcm.Seal(nonce, nonce, raw, nil), nil
}

func EncryptPacket(pkt packets.SerializedPacket, key [32]byte) []byte {
	output, err := encrypt(pkt, key[:])
	if err != nil {
		log.Print("Error during encryption")
		log.Fatal(err)
	}
	return output
}

func decrypt(ciphertext []byte, key []byte) ([]byte, error) {
	c, err := aes.NewCipher(key)
	if err != nil {
		log.Print("Cipher creation failed during decryption")
		return nil, err
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		log.Print("GCM creation failed during decryption")
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext is too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func DecryptPacket(pkt []byte, key [32]byte) packets.SerializedPacket {
	output, err := decrypt(pkt, key[:])
	if err != nil {
		log.Print("Error during decryption")
		log.Fatal(err)
	}
	return output
}
