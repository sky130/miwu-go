package userstore

import (
	"errors"
	"os"
	"testing"

	"miwu-go/internal/miot/model/user"
)

func TestLoadRejectsBadRecords(t *testing.T) {
	cipher, err := NewCipher(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewFileStore(t.TempDir(), cipher)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		body string
		want error
	}{
		{`{}`, ErrUnsupportedVersion},
		{`{"version":99}`, ErrUnsupportedVersion},
		{`{"version":1,"session":{}}`, ErrCorrupt},
		{`{`, ErrCorrupt},
	} {
		if err := os.WriteFile(store.pathFor("123"), []byte(tt.body), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Load(t.Context(), "123"); !errors.Is(err, tt.want) {
			t.Fatalf("%s: %v", tt.body, err)
		}
	}
}

func TestCredentialsRoundTrip(t *testing.T) {
	cipher, _ := NewCipher(make([]byte, 32))
	store, err := NewFileStore(t.TempDir(), cipher)
	if err != nil {
		t.Fatal(err)
	}
	want := user.Credentials{Identity: user.Identity{UserID: "123"}, Session: user.Session{ServiceToken: "secret"}}
	if err := store.Save(t.Context(), want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load(t.Context(), "123")
	if err != nil || got != want {
		t.Fatalf("round trip failed: %v", err)
	}
	if err := store.Delete(t.Context(), "123"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(t.Context(), "123"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}
