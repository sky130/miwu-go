package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"miwu-go/internal/app"
	"miwu-go/internal/bootstrap"
	"miwu-go/internal/cli"

	"golang.org/x/term"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	fd := int(os.Stdin.Fd())
	initial, _ := term.GetState(fd)
	restore := func() {
		if initial != nil {
			_ = term.Restore(fd, initial)
		}
	}
	code := cli.Run(ctx, os.Args[1:], cli.Options{
		In: os.Stdin, Out: os.Stdout, Err: os.Stderr,
		Terminal:       term.IsTerminal(fd) && term.IsTerminal(int(os.Stderr.Fd())),
		ReadPassword:   func() ([]byte, error) { return term.ReadPassword(fd) },
		InterruptInput: func() { restore(); _ = os.Stdin.Close() },
		Open: func(settings cli.Settings) (*app.App, error) {
			options := bootstrap.Options{
				ConfigDir:      settings.ConfigDir,
				DataDir:        settings.DataDir,
				RequestTimeout: settings.Timeout,
			}
			if settings.Verbose {
				options.Diagnostics = os.Stderr
			}
			return bootstrap.Open(options)
		},
	})
	restore()
	stop()
	os.Exit(code)
}
