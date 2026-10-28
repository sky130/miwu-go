// Package app owns workflows that span authentication, configuration and storage.
// It has no dependency on Cobra, terminal input or output formatting.
package app

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"miwu-go/internal/config"
	"miwu-go/internal/miot"
	"miwu-go/internal/miot/auth"
	"miwu-go/internal/miot/client/device"
	"miwu-go/internal/miot/client/home"
	profile "miwu-go/internal/miot/client/user"
	"miwu-go/internal/miot/model/user"
	"miwu-go/internal/miot/spec"
)

// Interfaces live with their consumer and contain only the operations it needs.
type ConfigStore interface {
	Load() (config.Config, error)
	Save(config.Config) error
}

type AccountStore interface {
	Save(context.Context, user.Credentials) error
	Load(context.Context, string) (user.Credentials, error)
	Delete(context.Context, string) error
	List(context.Context) ([]string, error)
}

type Authenticator interface {
	GenerateQRCode(context.Context) (*auth.QRCode, error)
	LoginWithQRCode(context.Context, *auth.QRCode) (user.Credentials, error)
	LoginWithPassword(context.Context, string, string) (user.Credentials, error)
	Refresh(context.Context, user.Credentials) (user.Credentials, error)
}

type App struct {
	config        ConfigStore
	accounts      func() (AccountStore, error)
	auth          Authenticator
	http          http.Client
	specification *spec.Client
	configMu      sync.Mutex
}

func New(cfg ConfigStore, open func() (AccountStore, error), authentication Authenticator, client http.Client) *App {
	return &App{
		config:        cfg,
		accounts:      sync.OnceValues(open),
		auth:          authentication,
		http:          client,
		specification: spec.NewClient(client),
	}
}

type services struct {
	user   *profile.Client
	home   *home.Client
	device *device.Client
}

func (a *App) services(ctx context.Context, id string) (*services, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if id == "" {
		return nil, miot.ErrAuthRequired
	}
	s, err := a.accounts()
	if err != nil {
		return nil, err
	}
	c, err := s.Load(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load account: %w", err)
	}
	if c.UserID == "" || c.ServiceToken == "" || c.ServiceSecurity == "" {
		return nil, miot.ErrAuthRequired
	}
	transport := miot.NewClient(a.http, c, miot.MiotBaseURL)
	return &services{
		user:   profile.NewClient(transport, c),
		home:   home.NewClient(transport),
		device: device.NewClient(transport),
	}, nil
}
