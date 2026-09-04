package main

import (
	"context"

	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:          "ploi-tui",
		Short:        "lazygit-style terminal UI for managing ploi.io servers",
		Long:         "ploi-tui is a terminal UI for managing your ploi.io servers, sites, deployments and certificates.\n\nRun it without arguments to launch the interactive UI. If no API token is configured yet, the connect wizard starts automatically.",
		Version:      version,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUI()
		},
	}

	root.AddCommand(
		newConnectCmd(),
		newLogoutCmd(),
		newVersionCmd(),
	)

	return root
}

func Execute() error {
	return newRootCmd().ExecuteContext(context.Background())
}
