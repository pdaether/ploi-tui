package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/pdaether/ploi-tui/internal/config"
)

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the stored ploi.io credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := newStore()
			if err != nil {
				return err
			}
			return runLogout(cmd.OutOrStdout(), store)
		},
	}
}

func runLogout(out io.Writer, store *config.Store) error {
	if _, err := store.LoadToken(); errors.Is(err, config.ErrTokenNotFound) {
		_, err := fmt.Fprintln(out, "No stored credentials found.")
		return err
	}

	if err := store.DeleteToken(); err != nil {
		return fmt.Errorf("remove credentials: %w", err)
	}
	_, err := fmt.Fprintln(out, "✓ Credentials removed.")
	return err
}
