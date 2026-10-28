// Package cli implements terminal interaction and script-friendly commands.
package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"miwu-go/internal/app"
	"miwu-go/internal/miot"

	"github.com/spf13/cobra"
)

type Settings struct {
	ConfigDir, DataDir, Account, Home, Output string
	NonInteractive, Verbose                   bool
	Timeout                                   time.Duration
}

type Options struct {
	In           io.Reader
	Out, Err     io.Writer
	Terminal     bool
	ReadPassword func() ([]byte, error)
	// InterruptInput releases a pending read on cancellation. The process owner
	// supplies this because Run must not close an arbitrary caller-owned reader.
	InterruptInput func()
	// Open is lazy: help and local commands never initialize credentials.
	Open func(Settings) (*app.App, error)
}

type state struct {
	options  Options
	settings Settings
	reader   *bufio.Reader
	app      *app.App
	commands map[string]*cobra.Command
	executed bool
}

type usageError struct {
	error
}

func usage(format string, args ...any) error {
	return &usageError{fmt.Errorf(format, args...)}
}

// Run emits each error once and returns a process exit code; it never calls os.Exit.
func Run(ctx context.Context, args []string, options Options) int {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if options.InterruptInput != nil {
		stop := context.AfterFunc(ctx, options.InterruptInput)
		defer stop()
	}
	if options.In == nil {
		options.In = strings.NewReader("")
	}
	if options.Out == nil {
		options.Out = io.Discard
	}
	if options.Err == nil {
		options.Err = io.Discard
	}
	s := &state{options: options, reader: bufio.NewReader(options.In), commands: map[string]*cobra.Command{}}
	root := s.root()
	root.SetArgs(args)
	if err := root.ExecuteContext(ctx); err != nil {
		// Parsing can fail before --output is bound. Preserve requested machine output.
		for i, arg := range args {
			if arg == "--" {
				break
			}
			if arg == "--output=json" || arg == "--output" && i+1 < len(args) && args[i+1] == "json" {
				s.settings.Output = "json"
			}
		}
		if !s.executed {
			var argument *usageError
			if !errors.As(err, &argument) {
				err = &usageError{err}
			}
		}
		s.report(err)
		return exitCode(err)
	}
	return 0
}

func (s *state) root() *cobra.Command {
	root := &cobra.Command{Use: "miwu", Short: "Manage Xiaomi homes and devices", SilenceErrors: true, SilenceUsage: true, Args: cobra.NoArgs}
	root.SetIn(s.options.In)
	root.SetOut(s.options.Out)
	root.SetErr(s.options.Err)
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return &usageError{err} })
	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		if s.settings.Output != "table" && s.settings.Output != "json" {
			return usage("--output must be table or json")
		}
		if s.settings.Home != "" {
			id, err := strconv.ParseInt(s.settings.Home, 10, 64)
			if err != nil || id <= 0 {
				return usage("--home must be a positive integer")
			}
		}
		if s.settings.Timeout <= 0 {
			return usage("--timeout must be positive")
		}
		return nil
	}
	flags := root.PersistentFlags()
	flags.StringVar(&s.settings.Account, "account", "", "Account ID (otherwise use the default)")
	flags.StringVar(&s.settings.Home, "home", "", "Home ID")
	flags.StringVar(&s.settings.Output, "output", "table", "Output: table or json")
	flags.BoolVar(&s.settings.NonInteractive, "non-interactive", false, "Never ask for input")
	flags.BoolVar(&s.settings.Verbose, "verbose", false, "Print sanitized request diagnostics")
	flags.DurationVar(&s.settings.Timeout, "timeout", time.Minute, "Timeout per network request")
	flags.StringVar(&s.settings.ConfigDir, "config-dir", "", "Configuration directory")
	flags.StringVar(&s.settings.DataDir, "data-dir", "", "Encrypted account directory")
	root.RunE = func(cmd *cobra.Command, _ []string) error {
		s.executed = true
		if !s.interactive() {
			return cmd.Help()
		}
		return s.runMenu(cmd.Context())
	}
	s.addLogin(root)
	s.addAccounts(root)
	s.addHomes(root)
	s.addDevices(root)
	s.addScenes(root)
	s.addSpecs(root)
	return root
}

func (s *state) add(parent *cobra.Command, name, description string, run func(context.Context) (result, error)) *cobra.Command {
	cmd := &cobra.Command{Use: name, Short: description, Args: func(_ *cobra.Command, args []string) error {
		if len(args) > 0 {
			return usage("unexpected arguments: %s", strings.Join(args, " "))
		}
		return nil
	}}
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		s.executed = true
		value, err := run(cmd.Context())
		if err != nil {
			if value.Data != nil {
				if outErr := s.emit(value, err); outErr != nil {
					return outErr
				}
				return &reportedError{err}
			}
			return err
		}
		return s.emit(value, nil)
	}
	parent.AddCommand(cmd)
	key := name
	if parent.Name() != "miwu" {
		key = parent.Name() + " " + name
	}
	s.commands[key] = cmd
	return cmd
}

func (s *state) application() (*app.App, error) {
	if s.app != nil {
		return s.app, nil
	}
	open := s.options.Open
	if open == nil {
		return nil, errors.New("application factory is required")
	}
	a, err := open(s.settings)
	if err == nil {
		s.app = a
	}
	return a, err
}

func (s *state) interactive() bool {
	return s.options.Terminal && !s.settings.NonInteractive && s.settings.Output != "json"
}

func (s *state) line(ctx context.Context, prompt string) (string, error) {
	if !s.interactive() {
		return "", usage("%s; provide explicit arguments in non-interactive mode", prompt)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	fmt.Fprint(s.options.Err, prompt+": ")
	type read struct {
		text string
		err  error
	}
	done := make(chan read, 1)
	go func() { text, err := s.reader.ReadString('\n'); done <- read{text, err} }()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case r := <-done:
		if r.err != nil {
			return "", r.err
		}
		return strings.TrimSpace(r.text), nil
	}
}

func (s *state) choose(ctx context.Context, prompt string, labels []string) (int, error) {
	if len(labels) == 0 {
		return 0, errors.New("no items available")
	}
	if !s.interactive() {
		return 0, usage("%s; provide an explicit selection", prompt)
	}
	for {
		fmt.Fprintln(s.options.Err, prompt)
		for i, label := range labels {
			fmt.Fprintf(s.options.Err, "  %d) %s\n", i+1, joinColumns([]string{label}))
		}
		answer, err := s.line(ctx, "Choose a number")
		if err != nil {
			return 0, err
		}
		n, err := strconv.Atoi(answer)
		if err == nil && n >= 1 && n <= len(labels) {
			return n - 1, nil
		}
		fmt.Fprintln(s.options.Err, "Enter one of the listed numbers.")
	}
}

func (s *state) confirm(ctx context.Context, prompt string) (bool, error) {
	answer, err := s.line(ctx, prompt+" [y/N]")
	return strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes"), err
}

func (s *state) account(ctx context.Context) (string, error) {
	if s.settings.Account != "" {
		return s.settings.Account, nil
	}
	a, err := s.application()
	if err != nil {
		return "", err
	}
	id, err := a.DefaultAccount()
	if err != nil {
		return "", err
	}
	if id != "" {
		ids, err := a.AccountIDs(ctx)
		if err != nil {
			return "", err
		}
		for _, v := range ids {
			if v == id {
				return id, nil
			}
		}
		fmt.Fprintln(s.options.Err, "The default account is unavailable.")
	}
	if !s.interactive() {
		return "", usage("--account is required when no valid default account is configured")
	}
	ids, err := a.AccountIDs(ctx)
	if err != nil {
		return "", err
	}
	if len(ids) == 0 {
		return "", fmt.Errorf("%w: run miwu login", miot.ErrAuthRequired)
	}
	index, err := s.choose(ctx, "Select account", ids)
	if err != nil {
		return "", err
	}
	return ids[index], nil
}

func (s *state) applicationForAccount(ctx context.Context) (*app.App, string, error) {
	id, err := s.account(ctx)
	if err != nil {
		return nil, "", err
	}
	application, err := s.application()
	return application, id, err
}

func (s *state) runMenu(ctx context.Context) error {
	keys := []string{"login", "account list", "account show", "account use", "account delete", "account check", "account refresh", "home list", "device list", "device show", "device get", "device set", "device action", "scene list", "scene run", "spec show", "spec languages", "spec icon", "spec parse-urn"}
	for {
		labels := append(append([]string{}, keys...), "Exit")
		index, err := s.choose(ctx, "MiWu", labels)
		if err != nil {
			return err
		}
		if index == len(keys) {
			return nil
		}
		cmd := s.commands[keys[index]]
		cmd.SetContext(ctx)
		if err = cmd.RunE(cmd, nil); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
				return err
			}
			s.report(err)
		}
	}
}
