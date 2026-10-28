package app

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"miwu-go/internal/miot/auth"
	"miwu-go/internal/miot/model/user"
)

type Account struct {
	ID      string `json:"id"`
	Default bool   `json:"default"`
}

type AccountInfo struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

func (a *App) ListAccounts(ctx context.Context) ([]Account, error) {
	ids, err := a.AccountIDs(ctx)
	if err != nil {
		return nil, err
	}
	defaultID, err := a.DefaultAccount()
	if err != nil {
		return nil, err
	}
	accounts := make([]Account, 0, len(ids))
	for _, id := range ids {
		accounts = append(accounts, Account{ID: id, Default: id == defaultID})
	}
	return accounts, nil
}

func (a *App) AccountInfo(ctx context.Context, accountID string) (AccountInfo, error) {
	clients, err := a.services(ctx, accountID)
	if err != nil {
		return AccountInfo{}, err
	}
	info, err := clients.user.GetInfo(ctx)
	if err != nil {
		return AccountInfo{}, err
	}
	return AccountInfo{
		ID:       strconv.FormatInt(info.UserID, 10),
		Nickname: info.Nickname,
		Avatar:   info.Avatar,
	}, nil
}

func (a *App) CheckSession(ctx context.Context, accountID string) error {
	clients, err := a.services(ctx, accountID)
	if err != nil {
		return err
	}
	return clients.user.CheckSession(ctx)
}

func (a *App) AccountIDs(ctx context.Context) ([]string, error) {
	s, err := a.accounts()
	if err != nil {
		return nil, err
	}
	return s.List(ctx)
}

func (a *App) DefaultAccount() (string, error) {
	a.configMu.Lock()
	defer a.configMu.Unlock()

	cfg, err := a.config.Load()
	return cfg.DefaultAccount, err
}

func (a *App) UseAccount(ctx context.Context, id string) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()

	s, err := a.accounts()
	if err != nil {
		return err
	}
	if _, err = s.Load(ctx, id); err != nil {
		return fmt.Errorf("select account: %w", err)
	}
	cfg, err := a.config.Load()
	if err != nil {
		return err
	}
	cfg.DefaultAccount = id
	return a.config.Save(cfg)
}

func (a *App) DeleteAccount(ctx context.Context, id string) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()

	s, err := a.accounts()
	if err != nil {
		return err
	}
	cfg, err := a.config.Load()
	if err != nil {
		return err
	}
	original := cfg
	if cfg.DefaultAccount == id {
		cfg.DefaultAccount = ""
		if err = a.config.Save(cfg); err != nil {
			return err
		}
	}
	if err = s.Delete(ctx, id); err != nil {
		if original != cfg {
			err = errors.Join(err, a.config.Save(original))
		}
		return fmt.Errorf("delete account: %w", err)
	}
	return nil
}

func (a *App) save(ctx context.Context, c user.Credentials) (string, error) {
	s, err := a.accounts()
	if err != nil {
		return "", err
	}
	if err = s.Save(ctx, c); err != nil {
		return "", fmt.Errorf("save credentials: %w", err)
	}
	return c.UserID, nil
}

func (a *App) LoginWithPassword(ctx context.Context, name, password string) (string, error) {
	if _, err := a.accounts(); err != nil {
		return "", err
	}
	c, err := a.auth.LoginWithPassword(ctx, name, password)
	if err != nil {
		return "", err
	}
	return a.save(ctx, c)
}

func (a *App) GenerateQRCode(ctx context.Context) (*auth.QRCode, error) {
	if _, err := a.accounts(); err != nil {
		return nil, err
	}
	return a.auth.GenerateQRCode(ctx)
}

func (a *App) LoginWithQRCode(ctx context.Context, code *auth.QRCode) (string, error) {
	c, err := a.auth.LoginWithQRCode(ctx, code)
	if err != nil {
		return "", err
	}
	return a.save(ctx, c)
}

func (a *App) Refresh(ctx context.Context, id string) error {
	s, err := a.accounts()
	if err != nil {
		return err
	}
	c, err := s.Load(ctx, id)
	if err != nil {
		return err
	}
	refreshed, err := a.auth.Refresh(ctx, c)
	if err != nil {
		return err
	}
	_, err = a.save(ctx, refreshed)
	return err
}
