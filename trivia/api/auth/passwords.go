package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	"golang.org/x/crypto/bcrypt"
)

// Default pepper that is used if SetAESKeyHex is not called from somewhere else. It's never used.
const passwordAESPepper = "7c001eb77d617bc94ee1c357c23932dbe6713833022535afc779dfb04ffb06fd"
const bcryptCost int = 10

var passwordAESKey = make([]byte, 32)

func init() {
	SetAESKeyHex(passwordAESPepper)
}

// SetAESKeyHex sets the global pepper used to encrypt HASHED passwords before they are stored
// in a database.
func SetAESKeyHex(pepperHex string) {
	decodedKey, err := hex.DecodeString(pepperHex)
	if err != nil {
		panic(fmt.Errorf("auth: error decoding aesKey: %v", err))
	}

	if len(decodedKey) != 32 {
		panic(fmt.Sprintf("auth: init expects the passwordAESKey to be 32 bytes (key is %d bytes)", len(decodedKey)))
	}

	passwordAESKey = decodedKey
}

// padToBlocksize pads a slice of bytes so that its length is a multiple of the given blocksize.
// It then returns the new padded slice.
func padToBlocksize(unpadded []byte, blocksize int, paddingByte byte) []byte {
	required := blocksize - (len(unpadded) % blocksize)
	newSlice := unpadded
	if required > 0 {
		newSlice = make([]byte, len(unpadded)+required)
		copy(newSlice, unpadded)
		for i := len(unpadded); i < len(unpadded)+required; i++ {
			newSlice[i] = paddingByte
		}
	}
	return newSlice
}

func encrypt(unencrypted []byte) (encrypted []byte, err error) {
	unencrypted = padToBlocksize(unencrypted, aes.BlockSize, 0x04)
	block, err := aes.NewCipher(passwordAESKey)
	if err != nil {
		return nil, err
	}

	encrypted = make([]byte, aes.BlockSize+len(unencrypted))
	iv := encrypted[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err // #CLEANUP I might actually need to panic here but it's not important right now.
	}

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(encrypted[aes.BlockSize:], unencrypted)

	return
}

func decrypt(encrypted []byte) (unencrypted []byte, err error) {
	block, err := aes.NewCipher(passwordAESKey)
	if err != nil {
		return nil, err
	}

	if len(encrypted) < aes.BlockSize {
		return nil, fmt.Errorf("ciphertext (encrypted) is smaller than AES block size")
	}

	iv := encrypted[:aes.BlockSize]
	encrypted = encrypted[aes.BlockSize:]

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(encrypted, encrypted)
	unencrypted = encrypted

	return
}

// PreparePassword prepares a password for storage by passing it through sha256, bcrypt, and then aes256.
func PreparePassword(password string) ([]byte, error) {
	passwordBytes := []byte(password)

	hasher := sha256.New()
	_, err := hasher.Write(passwordBytes)
	if err != nil {
		return nil, err
	}
	hashed := hasher.Sum(nil)

	bcryptedPassword, err := bcrypt.GenerateFromPassword(hashed, bcryptCost)
	if err != nil {
		return nil, err
	}

	encrypted, err := encrypt(bcryptedPassword)
	if err != nil {
		return nil, err
	}

	return encrypted, nil
}

// ComparePassword compares a prepared password that has been stored somewhere to a plaintext password taken from a user.
func ComparePassword(stored []byte, password string) error {
	passwordBytes := []byte(password)
	hasher := sha256.New()
	_, err := hasher.Write(passwordBytes)
	if err != nil {
		return err
	}
	hashed := hasher.Sum(nil)

	storedUnencrypted, err := decrypt(stored)
	if err != nil {
		return err
	}

	return bcrypt.CompareHashAndPassword(storedUnencrypted, hashed)
}
