package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pdaether/ploi-tui/internal/api"
	"github.com/pdaether/ploi-tui/internal/config"
	cache "github.com/pdaether/ploi-tui/internal/store"
	"github.com/pdaether/ploi-tui/internal/ui"
)

func runTUI() error {
	store, err := newStore()
	if err != nil {
		return err
	}
	stored, err := store.LoadToken()
	if errors.Is(err, config.ErrTokenNotFound) {
		return runConnectFlow(context.Background(), os.Stdin, os.Stdout, store)
	}
	if err != nil {
		return err
	}

	opts := []api.Option{}
	if baseURL, ok := api.BaseURLFromEnv(); ok {
		opts = append(opts, api.WithBaseURL(baseURL))
	}
	client := api.New(stored.Token, opts...)
	cache, err := cache.New()
	if err != nil {
		return fmt.Errorf("create cache: %w", err)
	}
	model := ui.NewApp(client,
		ui.WithCache(cache),
	)
	program := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("run TUI: %w", err)
	}
	return nil
}
