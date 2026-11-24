package render

import (
	"fmt"
	"io"
	"time"

	"github.com/rahulkaushal04/netwhy/internal/probe"
	"github.com/rahulkaushal04/netwhy/internal/verdict"
)

func Human(w io.Writer, target string, v verdict.Verdict) {
	fmt.Fprintf(w, "netwhy: %s\n\n", target)
	for _, r := range v.Results {
		fmt.Fprintf(w, "  %s %-7s %s\n", mark(r.Status), r.Name, r.Summary)
	}
	fmt.Fprintln(w)

	if v.OK {
		fmt.Fprintf(w, "Verdict: %s\n", v.Summary)
		return
	}

	fmt.Fprintf(w, "Verdict: the %s layer failed\n  %s\n\n", v.Layer, v.Summary)
	fmt.Fprintln(w, "Evidence:")
	fmt.Fprintf(w, "  sent:     %s\n", v.Evidence.Sent)
	if v.Evidence.Received != "" {
		fmt.Fprintf(w, "  received: %s\n", v.Evidence.Received)
	}
	if v.Evidence.Error != "" {
		fmt.Fprintf(w, "  error:    %s\n", v.Evidence.Error)
	}
	fmt.Fprintf(w, "  took:     %s\n", v.Evidence.Duration.Round(time.Millisecond))
}

func mark(s probe.Status) string {
	switch s {
	case probe.Pass:
		return "✓"
	case probe.Fail:
		return "✗"
	default:
		return "-"
	}
}
