// Package bootstrap assembles the shared application for each executable.
package bootstrap

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"miwu-go/internal/app"
	"miwu-go/internal/config"
	"miwu-go/internal/miot/auth"
	"miwu-go/internal/userstore"
)

// These identifiers locate existing encryption keys. They are independent of
// the module and executable names and must remain stable across renames.
const (
	keyringService = "miwu-cli"
	keyringAccount = "account"
)

type Options struct {
	ConfigDir      string
	DataDir        string
	RequestTimeout time.Duration
	Diagnostics    io.Writer
}

// Open builds the application without accessing the keyring or account files.
// A nil Diagnostics writer disables request diagnostics.
func Open(options Options) (*app.App, error) {
	if options.RequestTimeout == 0 {
		options.RequestTimeout = time.Minute
	}
	if options.RequestTimeout < 0 {
		return nil, fmt.Errorf("request timeout must be positive")
	}
	configDir := options.ConfigDir
	if configDir == "" {
		var err error
		configDir, err = config.DefaultDir()
		if err != nil {
			return nil, err
		}
	}
	dataDir := options.DataDir
	if dataDir == "" {
		dataDir = filepath.Join(configDir, "accounts")
	}

	var transport http.RoundTripper = http.DefaultTransport
	if options.Diagnostics != nil {
		transport = &traceTransport{base: transport, out: options.Diagnostics}
	}
	client := http.Client{Timeout: options.RequestTimeout, Transport: transport}
	openAccounts := func() (app.AccountStore, error) {
		key, err := userstore.LoadOrCreateMasterKey(keyringService, keyringAccount)
		if err != nil {
			return nil, fmt.Errorf("load account key: %w", err)
		}
		cipher, err := userstore.NewCipher(key)
		if err != nil {
			return nil, err
		}
		return userstore.NewFileStore(dataDir, cipher)
	}

	return app.New(config.NewFile(configDir), openAccounts, auth.NewClient(client), client), nil
}
