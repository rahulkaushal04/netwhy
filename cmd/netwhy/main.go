// Command netwhy finds out why you can't reach something.
//
// It tests each layer in dependency order (system, gateway, DNS, TCP, TLS, HTTP),
// names the lowest layer that failed, and prints the evidence.
//
// Usage:
//
//	netwhy [--timeout 5s] <target>
//
// The target is a URL, host:port, a hostname or an IP. A bare host is tested as
// https on port 443; host:port stops after the TCP check.
//
// Exit codes: 0 no layer failed, 1 a layer failed, 2 invalid input.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"

	"github.com/rahulkaushal04/netwhy/internal/plan"
	"github.com/rahulkaushal04/netwhy/internal/render"
	"github.com/rahulkaushal04/netwhy/internal/verdict"
)

// version is set at release time with -ldflags "-X main.version=<tag>".
var version = "dev"

// errFailure means a layer failed. The report was already printed.
var errFailure = errors.New("a layer failed")

func main() {
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:           "netwhy <target>",
		Version:       version,
		Short:         "Find out why you can't reach something",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := plan.ParseTarget(args[0])
			if err != nil {
				return err
			}
			results := plan.Run(cmd.Context(), plan.Build(t), timeout)
			v := verdict.Decide(results)
			render.Human(cmd.OutOrStdout(), t.Raw, v)
			if !v.OK {
				return errFailure
			}
			return nil
		},
	}
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Second, "time budget for each probe")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := cmd.ExecuteContext(ctx); err != nil {
		if errors.Is(err, errFailure) {
			stop()
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		stop()
		os.Exit(2)
	}
}
