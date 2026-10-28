package app

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"miwu-go/internal/config"
	"miwu-go/internal/miot/auth"
	"miwu-go/internal/miot/model/user"
	"miwu-go/internal/userstore"
)

type memoryConfig struct {
	value config.Config
	fail  error
}

func (c *memoryConfig) Load() (config.Config, error) {
	return c.value, nil
}

func (c *memoryConfig) Save(v config.Config) error {
	if c.fail != nil {
		return c.fail
	}
	c.value = v
	return nil
}

type memoryAccounts struct {
	credentials        user.Credentials
	saveErr, deleteErr error
	deleted            bool
}

func (s *memoryAccounts) Load(context.Context, string) (user.Credentials, error) {
	return s.credentials, nil
}

func (s *memoryAccounts) Save(_ context.Context, c user.Credentials) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.credentials = c
	return nil
}

func (s *memoryAccounts) List(context.Context) ([]string, error) {
	return []string{s.credentials.UserID}, nil
}

func (s *memoryAccounts) Delete(context.Context, string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deleted = true
	return nil
}

type fakeAuth struct{}

func (fakeAuth) GenerateQRCode(context.Context) (*auth.QRCode, error) {
	return &auth.QRCode{}, nil
}

func (fakeAuth) LoginWithQRCode(context.Context, *auth.QRCode) (user.Credentials, error) {
	return user.Credentials{Identity: user.Identity{UserID: "123"}}, nil
}

func (fakeAuth) LoginWithPassword(context.Context, string, string) (user.Credentials, error) {
	return user.Credentials{Identity: user.Identity{UserID: "123"}}, nil
}

func (fakeAuth) Refresh(_ context.Context, c user.Credentials) (user.Credentials, error) {
	c.ServiceToken = "new"
	return c, nil
}

func TestWorkflowFailuresAreNotReportedAsSuccess(t *testing.T) {
	disk := errors.New("disk unavailable")
	cfg := &memoryConfig{value: config.Config{DefaultAccount: "123"}}
	store := &memoryAccounts{credentials: user.Credentials{Identity: user.Identity{UserID: "123"}, Session: user.Session{ServiceToken: "old"}}}
	a := New(cfg, func() (AccountStore, error) { return store, nil }, fakeAuth{}, http.Client{})
	cfg.fail = disk
	if err := a.UseAccount(t.Context(), "123"); !errors.Is(err, disk) {
		t.Fatal(err)
	}
	if err := a.DeleteAccount(t.Context(), "123"); !errors.Is(err, disk) || store.deleted {
		t.Fatal("account deleted despite config failure")
	}
	cfg.fail = nil
	store.deleteErr = disk
	if err := a.DeleteAccount(t.Context(), "123"); !errors.Is(err, disk) || cfg.value.DefaultAccount != "123" {
		t.Fatal("failed deletion lost default")
	}
	store.saveErr = disk
	if id, err := a.LoginWithPassword(t.Context(), "name", "password"); !errors.Is(err, disk) || id != "" {
		t.Fatal("login save failure hidden")
	}
	if err := a.Refresh(t.Context(), "123"); !errors.Is(err, disk) || store.credentials.ServiceToken != "old" {
		t.Fatal("refresh changed stored credentials on failure")
	}
	store.saveErr = nil
	store.deleteErr = nil
	if err := a.DeleteAccount(t.Context(), "123"); err != nil || cfg.value.DefaultAccount != "" || !store.deleted {
		t.Fatal(err)
	}
}

func TestConstructionAndDefaultDoNotOpenAccounts(t *testing.T) {
	a := New(&memoryConfig{}, func() (AccountStore, error) {
		t.Fatal("opened account store unexpectedly")
		return nil, userstore.ErrNotFound
	}, fakeAuth{}, http.Client{})
	if _, err := a.DefaultAccount(); err != nil {
		t.Fatal(err)
	}
}
