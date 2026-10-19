// Package cli implements Quotary's Cobra command tree.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/juanrubio/quotary/internal/backend/demo"
	"github.com/juanrubio/quotary/internal/config"
	"github.com/juanrubio/quotary/internal/tui"
)

// Version is set at release time via -ldflags.
var Version = "dev"

type options struct {
	demo     bool
	scenario string
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
			if !opts.demo {
				return tui.Run(cmd.Context(), tui.Options{})
			}
			runtime, err := demo.NewRuntime(demo.Scenario(opts.scenario), demoDeps())
			if err != nil {
				return err
			}
			return tui.Run(cmd.Context(), tui.Options{Demo: true, Session: runtime})
		},
	}
	cmd.Flags().BoolVar(&opts.demo, "demo", false, "run the offline fictional demonstration")
	cmd.Flags().StringVar(&opts.scenario, "scenario", string(demo.ScenarioSubscription), "demo scenario: subscription, attention, catalog, empty, setup")
	return cmd
}

func demoDeps() demo.Deps {
	return demo.Deps{
		Store:    config.NewMemoryStore(config.Defaults()),
		Fixtures: demo.FixtureFS(),
		Runner:   demo.OfflineRunner{},
	}
}

// Execute runs the root command and returns its error.
func Execute() error {
	return NewRootCmd().Execute()
}
