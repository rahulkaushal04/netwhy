// Command netwhy finds out why you can't reach something.
//
// It tests each layer in dependency order (system, gateway, DNS, captive portal,
// TCP, path MTU, TLS, HTTP), names the lowest layer that failed, and prints the
// evidence.
//
// Usage:
//
//	netwhy [--timeout 5s] [--json | --markdown] [--no-portal] <target>
//	netwhy watch [--interval 30s] <target>
//
// The target is a URL, host:port, a hostname or an IP. A bare host is tested as
// https on port 443; host:port stops after the TCP check.
//
// watch repeats the check and prints a line only when the result changes.
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
	"github.com/rahulkaushal04/netwhy/internal/probe"
	"github.com/rahulkaushal04/netwhy/internal/render"
	"github.com/rahulkaushal04/netwhy/internal/verdict"
)

// version is set at release time with -ldflags "-X main.version=<tag>".
var version = "dev"

// errFailure means a layer failed. The report was already printed.
var errFailure = errors.New("a layer failed")

type options struct {
	timeout  time.Duration
	noPortal bool
	json     bool
	markdown bool
	interval time.Duration
}

// diagnose runs every applicable probe against target and decides the verdict.
func diagnose(ctx context.Context, target string, o *options) (verdict.Verdict, error) {
	t, err := plan.ParseTarget(target)
	if err != nil {
		return verdict.Verdict{}, err
	}
	return verdict.Decide(plan.Run(ctx, plan.Build(t, !o.noPortal), o.timeout)), nil
}

func main() {
	var o options

	cmd := &cobra.Command{
		Use:           "netwhy <target>",
		Version:       version,
		Short:         "Find out why you can't reach something",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			v, err := diagnose(cmd.Context(), args[0], &o)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			switch {
			case o.json:
				err = render.JSON(out, args[0], v)
			case o.markdown:
				render.Markdown(out, args[0], v)
			default:
				render.Human(out, args[0], v)
			}
			if err != nil {
				return err
			}
			if !v.OK {
				return errFailure
			}
			return nil
		},
	}
	cmd.PersistentFlags().DurationVar(&o.timeout, "timeout", 5*time.Second, "time budget for each probe")
	cmd.PersistentFlags().BoolVar(&o.noPortal, "no-portal", false, "skip the captive-portal check, which requests "+probe.PortalURL)
	cmd.Flags().BoolVar(&o.json, "json", false, "print the verdict as JSON")
	cmd.Flags().BoolVar(&o.markdown, "markdown", false, "print the verdict as Markdown")
	cmd.MarkFlagsMutuallyExclusive("json", "markdown")
	cmd.AddCommand(watchCmd(&o))

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
