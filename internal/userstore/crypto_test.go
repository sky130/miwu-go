package userstore

import (
	"bytes"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/zalando/go-keyring"
)

type fakeKeyring struct {
	value          string
	getErr, setErr error
	sets           int
}

func (k *fakeKeyring) Get(string, string) (string, error) {
	return k.value, k.getErr
}

func (k *fakeKeyring) Set(_, _, v string) error {
	k.sets++
	k.value = v
	return k.setErr
}

func TestMasterKeyIsNeverReplacedOnReadFailure(t *testing.T) {
	denied := errors.New("keychain locked")
	for _, backend := range []*fakeKeyring{{getErr: denied}, {value: "invalid base64"}, {value: base64.StdEncoding.EncodeToString([]byte("short"))}} {
		if _, err := loadOrCreateMasterKey(backend, "service", "account"); err == nil || backend.sets != 0 {
			t.Fatal("master key was replaced on read failure")
		}
	}
	backend := &fakeKeyring{getErr: keyring.ErrNotFound}
	key, err := loadOrCreateMasterKey(backend, "service", "account")
	if err != nil || len(key) != 32 || backend.sets != 1 {
		t.Fatal("new key was not persisted")
	}
	backend.getErr = nil
	again, err := loadOrCreateMasterKey(backend, "service", "account")
	if err != nil || !bytes.Equal(key, again) || backend.sets != 1 {
		t.Fatal("existing key changed")
	}
	backend = &fakeKeyring{getErr: keyring.ErrNotFound, setErr: denied}
	if _, err := loadOrCreateMasterKey(backend, "service", "account"); !errors.Is(err, denied) {
		t.Fatal("key save failure hidden")
	}
}

func TestOpenRejectsWrongKeysAndNonces(t *testing.T) {
	a, _ := NewCipher(make([]byte, 32))
	b, _ := NewCipher(bytes.Repeat([]byte{1}, 32))
	box, err := a.Seal([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = b.Open(box); err == nil {
		t.Fatal("wrong key accepted")
	}
	box.Nonce = []byte{1}
	if _, err = a.Open(box); !errors.Is(err, ErrCorrupt) {
		t.Fatal(err)
	}
}
