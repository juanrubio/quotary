// Package cli implements Quotary's Cobra command tree.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/juanrubio/quotary/internal/tui"
)

// Version is set at release time via -ldflags.
var Version = "dev"

type options struct {
	demo bool
}

// NewRootCmd builds the root command. It is exported for tests.
func NewRootCmd() *cobra.Command {
	opts := &options{}
	cmd := &cobra.Command{
		Use:           "quotary",
		Short:         "AI subscription capacity, resets, and history in the terminal",
		Long:          "Quotary shows the capacity, reset times, plans, and usage history of the AI services you use. Run with --demo for an offline fictional demonstration.",
		Version:       Version,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !term.IsTerminal(int(os.Stdout.Fd())) {
				return fmt.Errorf("quotary needs an interactive terminal; use --version for non-interactive output")
			}
			return tui.Run(cmd.Context(), tui.Options{Demo: opts.demo})
		},
	}
	cmd.Flags().BoolVar(&opts.demo, "demo", false, "run the offline fictional demonstration")
	return cmd
}

// Execute runs the root command and returns its error.
func Execute() error {
	return NewRootCmd().Execute()
}
