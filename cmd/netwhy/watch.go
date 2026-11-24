package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rahulkaushal04/netwhy/internal/verdict"
)

func watchCmd(o *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "watch <target>",
		Short:         "Re-check on an interval and print only when the result changes",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "watching %s every %s (Ctrl-C to stop)\n", args[0], o.interval)

			var last string
			for {
				v, err := diagnose(cmd.Context(), args[0], o)
				if err != nil {
					return err
				}
				var line string
				if last, line = watchLine(last, v, time.Now()); line != "" {
					fmt.Fprintln(out, line)
				}
				select {
				case <-cmd.Context().Done():
					return nil
				case <-time.After(o.interval):
				}
			}
		},
	}
	cmd.Flags().DurationVar(&o.interval, "interval", 30*time.Second, "time between checks")
	return cmd
}

// watchLine returns the new signature of v and, when it differs from prev, a
// timestamped line describing it. An empty line means nothing changed.
func watchLine(prev string, v verdict.Verdict, now time.Time) (sig, line string) {
	parts := make([]string, len(v.Results))
	for i, r := range v.Results {
		parts[i] = r.Name + "=" + string(r.Status)
	}
	sig = strings.Join(parts, " ")
	if sig == prev {
		return sig, ""
	}

	desc := v.Summary
	if !v.OK {
		desc = fmt.Sprintf("the %s layer failed: %s", v.Layer, v.Summary)
	}
	return sig, now.Format("15:04:05") + " " + desc
}
