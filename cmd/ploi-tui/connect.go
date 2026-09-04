package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/pdaether/ploi-tui/internal/api"
	"github.com/pdaether/ploi-tui/internal/config"
)

const apiKeyURL = "https://ploi.io/profile/api-keys"

func newConnectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "connect",
		Short: "Validate and store your ploi.io API token",
		Long:  "Prompts for an API token (create one at https://ploi.io/profile/api-keys), validates it against the ploi API and stores it in the OS keyring with a config-file fallback.",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := newStore()
			if err != nil {
				return err
			}
			return runConnectFlow(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), store)
		},
	}
}

var newStore = func() (*config.Store, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return config.NewStore(cfg), nil
}

func runConnectFlow(ctx context.Context, in io.Reader, out io.Writer, store *config.Store) error {
	if _, err := fmt.Fprintf(out, "Welcome to ploi-tui\n\nCreate an API token at:\n%s\nRequired scopes: read servers/sites/certificates (+ write servers for restart actions)\n\n", apiKeyURL); err != nil {
		return fmt.Errorf("write connect prompt: %w", err)
	}

	token, err := readToken(in, out)
	if err != nil {
		return fmt.Errorf("read token: %w", err)
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("no token entered; get one at " + apiKeyURL)
	}

	opts := []api.Option{}
	if u, ok := api.BaseURLFromEnv(); ok {
		opts = append(opts, api.WithBaseURL(u))
	}
	client := api.New(token, opts...)

	user, err := client.User(ctx)
	if errors.Is(err, api.ErrUnauthorized) {
		return fmt.Errorf("token rejected by ploi (%v)\ncreate a new one at %s", err, apiKeyURL)
	}
	if err != nil {
		return fmt.Errorf("validate token: %w", err)
	}

	loc, err := store.SaveToken(token)
	if err != nil {
		return fmt.Errorf("store token: %w", err)
	}

	result := fmt.Sprintf("✓ Token valid — %s\n", user.Email)
	if user.Name != "" && user.Name != user.Email {
		result += fmt.Sprintf("  Account: %s (%s plan)\n", user.Name, user.Plan)
	} else if user.Plan != "" {
		result += fmt.Sprintf("  Plan: %s\n", user.Plan)
	}
	result += fmt.Sprintf("✓ Stored in %s\n\nAll set! Run `ploi-tui` to launch the UI.\n", loc)
	if _, err := fmt.Fprint(out, result); err != nil {
		return fmt.Errorf("write connect result: %w", err)
	}
	return nil
}

func readToken(in io.Reader, out io.Writer) (string, error) {
	if f, ok := in.(*os.File); ok && isTerminal(f) {
		if _, err := fmt.Fprint(out, "API token › "); err != nil {
			return "", err
		}
		b, err := term.ReadPassword(int(f.Fd()))
		if _, writeErr := fmt.Fprintln(out); writeErr != nil {
			return "", writeErr
		}
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
