package userstore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"github.com/zalando/go-keyring"
)

type sealedBox struct {
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
}

type keyringBackend interface {
	Get(string, string) (string, error)
	Set(string, string, string) error
}

type systemKeyring struct{}

func (systemKeyring) Get(service, account string) (string, error) {
	return keyring.Get(service, account)
}

func (systemKeyring) Set(service, account, value string) error {
	return keyring.Set(service, account, value)
}

func LoadOrCreateMasterKey(service, account string) ([]byte, error) {
	return loadOrCreateMasterKey(systemKeyring{}, service, account)
}

func loadOrCreateMasterKey(backend keyringBackend, service, account string) ([]byte, error) {
	if v, err := backend.Get(service, account); err == nil {
		key, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return nil, fmt.Errorf("decode master key: %w", err)
		}
		if len(key) != 32 {
			return nil, fmt.Errorf("master key must be 32 bytes")
		}
		return key, nil
	} else if !errors.Is(err, keyring.ErrNotFound) {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := backend.Set(service, account, base64.StdEncoding.EncodeToString(key)); err != nil {
		return nil, err
	}
	return key, nil
}

type Cipher struct {
	aead cipher.AEAD
}

func NewCipher(key []byte) (*Cipher, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("cipher: need 32-byte key, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

func (c *Cipher) Seal(plain []byte) (sealedBox, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return sealedBox{}, err
	}
	return sealedBox{Nonce: nonce, Ciphertext: c.aead.Seal(nil, nonce, plain, nil)}, nil
}

func (c *Cipher) Open(b sealedBox) ([]byte, error) {
	if len(b.Nonce) != c.aead.NonceSize() {
		return nil, fmt.Errorf("%w: invalid nonce length", ErrCorrupt)
	}
	return c.aead.Open(nil, b.Nonce, b.Ciphertext, nil)
}
