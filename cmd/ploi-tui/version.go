package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = "dev"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the ploi-tui version",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "ploi-tui %s\n", version)
			return err
		},
	}
}
