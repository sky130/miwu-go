package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"miwu-go/internal/miot/auth"

	"github.com/mdp/qrterminal/v4"
	"github.com/spf13/cobra"
	"rsc.io/qr"
)

func message(text string) result {
	return result{Data: map[string]string{"message": text}}
}

func (s *state) addLogin(root *cobra.Command) {
	var username string
	var qrLogin, passwordStdin bool
	cmd := s.add(root, "login", "Log in and securely save an account", func(ctx context.Context) (result, error) {
		if passwordStdin && username == "" {
			return result{}, usage("--password-stdin requires --username")
		}
		a, err := s.application()
		if err != nil {
			return result{}, err
		}
		name := username
		useQR := qrLogin
		if name == "" && !useQR {
			if !s.interactive() {
				return result{}, usage("specify --qr or --username with --password-stdin")
			}
			index, err := s.choose(ctx, "Login method", []string{"Scan QR code", "Username and password"})
			if err != nil {
				return result{}, err
			}
			useQR = index == 0
			if !useQR {
				name, err = s.line(ctx, "Username")
				if err != nil {
					return result{}, err
				}
			}
		}
		if useQR {
			return s.qrLogin(ctx)
		}
		var password string
		if passwordStdin {
			b, err := io.ReadAll(io.LimitReader(s.options.In, 1<<20))
			if err != nil {
				return result{}, err
			}
			password = strings.TrimRight(string(b), "\r\n")
		} else {
			if !s.interactive() || s.options.ReadPassword == nil {
				return result{}, usage("use --password-stdin outside an interactive terminal")
			}
			fmt.Fprint(s.options.Err, "Password: ")
			b, err := s.password(ctx)
			fmt.Fprintln(s.options.Err)
			if err != nil {
				return result{}, err
			}
			password = string(b)
		}
		if name == "" || password == "" {
			return result{}, usage("username and password cannot be empty")
		}
		id, err := a.LoginWithPassword(ctx, name, password)
		if errors.Is(err, auth.ErrChallengeRequired) && s.interactive() {
			yes, e := s.confirm(ctx, "Additional verification is required. Switch to QR login?")
			if e != nil {
				return result{}, e
			}
			if yes {
				return s.qrLogin(ctx)
			}
		}
		if err != nil {
			return result{}, err
		}
		return result{Data: map[string]string{"account": id, "message": "Logged in and saved"}}, nil
	})
	cmd.Flags().StringVarP(&username, "username", "u", "", "Login username")
	cmd.Flags().BoolVarP(&qrLogin, "qr", "q", false, "Use QR login")
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "Read password from standard input")
	cmd.MarkFlagsMutuallyExclusive("qr", "username")
	cmd.MarkFlagsMutuallyExclusive("qr", "password-stdin")
}

func (s *state) password(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	type read struct {
		value []byte
		err   error
	}
	done := make(chan read, 1)
	go func() { value, err := s.options.ReadPassword(); done <- read{value, err} }()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-done:
		return r.value, r.err
	}
}

func (s *state) qrLogin(ctx context.Context) (result, error) {
	a, err := s.application()
	if err != nil {
		return result{}, err
	}
	for {
		code, err := a.GenerateQRCode(ctx)
		if err != nil {
			return result{}, err
		}
		fmt.Fprintln(s.options.Err, "Scan with Mi Home:")
		qrterminal.GenerateWithConfig(code.Data, qrterminal.Config{Level: qr.L, Writer: s.options.Err, QuietZone: qrterminal.QUIET_ZONE, HalfBlocks: true})
		id, err := a.LoginWithQRCode(ctx, code)
		if errors.Is(err, auth.ErrQRCodeExpired) || errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
			if s.interactive() {
				yes, e := s.confirm(ctx, "QR login expired or timed out. Generate a new code?")
				if e != nil {
					return result{}, e
				}
				if yes {
					continue
				}
			}
		}
		if err != nil {
			return result{}, err
		}
		return result{Data: map[string]string{"account": id, "message": "Logged in and saved"}}, nil
	}
}

func isTimeout(err error) bool {
	var v interface{ Timeout() bool }
	return errors.As(err, &v) && v.Timeout()
}

func (s *state) addAccounts(root *cobra.Command) {
	parent := &cobra.Command{Use: "account", Short: "Manage saved accounts", Args: cobra.NoArgs}
	root.AddCommand(parent)
	s.add(parent, "list", "List saved accounts", func(ctx context.Context) (result, error) {
		a, err := s.application()
		if err != nil {
			return result{}, err
		}
		accounts, err := a.ListAccounts(ctx)
		if err != nil {
			return result{}, err
		}
		rows := make([][]string, 0, len(accounts))
		for _, account := range accounts {
			rows = append(rows, []string{account.ID, strconv.FormatBool(account.Default)})
		}
		return result{Data: accounts, Columns: []string{"ACCOUNT", "DEFAULT"}, Rows: rows}, nil
	})
	s.add(parent, "show", "Show account profile", func(ctx context.Context) (result, error) {
		application, accountID, err := s.applicationForAccount(ctx)
		if err != nil {
			return result{}, err
		}
		info, err := application.AccountInfo(ctx, accountID)
		if err != nil {
			return result{}, err
		}
		return result{Data: info}, nil
	})
	s.add(parent, "use", "Save the default account", func(ctx context.Context) (result, error) {
		// Explicit selection is intentional: an existing default must not hide this chooser.
		id := s.settings.Account
		a, err := s.application()
		if err != nil {
			return result{}, err
		}
		if id == "" {
			ids, e := a.AccountIDs(ctx)
			if e != nil {
				return result{}, e
			}
			i, e := s.choose(ctx, "Select default account", ids)
			if e != nil {
				return result{}, e
			}
			id = ids[i]
		}
		if err = a.UseAccount(ctx, id); err != nil {
			return result{}, err
		}
		return message("Default account saved: " + id), nil
	})
	var yes bool
	cmd := s.add(parent, "delete", "Delete a saved account", func(ctx context.Context) (result, error) {
		id, err := s.account(ctx)
		if err != nil {
			return result{}, err
		}
		if !yes {
			ok, e := s.confirm(ctx, "Delete account "+id+"?")
			if e != nil {
				return result{}, e
			}
			if !ok {
				return result{}, context.Canceled
			}
		}
		a, err := s.application()
		if err != nil {
			return result{}, err
		}
		if err = a.DeleteAccount(ctx, id); err != nil {
			return result{}, err
		}
		return message("Account deleted: " + id), nil
	})
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm deletion without prompting")
	s.add(parent, "check", "Check the remote session", func(ctx context.Context) (result, error) {
		application, accountID, err := s.applicationForAccount(ctx)
		if err != nil {
			return result{}, err
		}
		if err = application.CheckSession(ctx, accountID); err != nil {
			return result{}, err
		}
		return message("Session is valid"), nil
	})
	s.add(parent, "refresh", "Refresh and save the session", func(ctx context.Context) (result, error) {
		id, err := s.account(ctx)
		if err != nil {
			return result{}, err
		}
		a, err := s.application()
		if err != nil {
			return result{}, err
		}
		if err = a.Refresh(ctx, id); err != nil {
			return result{}, err
		}
		return message("Session refreshed and saved"), nil
	})
}
