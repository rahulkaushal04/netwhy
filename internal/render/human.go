// Package render prints a verdict for people.
package render

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/rahulkaushal04/netwhy/internal/evidence"
	"github.com/rahulkaushal04/netwhy/internal/probe"
	"github.com/rahulkaushal04/netwhy/internal/verdict"
)

func Human(w io.Writer, target string, v verdict.Verdict) {
	fmt.Fprintf(w, "netwhy: %s\n\n", target)
	for _, r := range v.Results {
		fmt.Fprintf(w, "  %s %-8s %s\n", mark(r.Status), r.Name, r.Summary)
	}
	fmt.Fprintln(w)

	if v.OK {
		fmt.Fprintf(w, "Verdict: %s\n", v.Summary)
	} else {
		fmt.Fprintf(w, "Verdict: the %s layer failed\n  %s\n\nEvidence:\n", v.Layer, v.Summary)
		writeEvidence(w, v.Evidence)
	}

	for _, r := range v.Warnings {
		fmt.Fprintf(w, "\nWarning (%s): %s\n", r.Name, r.Summary)
		writeEvidence(w, r.Evidence)
	}
}

func writeEvidence(w io.Writer, ev evidence.Evidence) {
	field(w, "sent", ev.Sent)
	field(w, "received", ev.Received)
	field(w, "error", ev.Error)
	field(w, "took", ev.Duration.Round(time.Millisecond).String())
}

func field(w io.Writer, label, value string) {
	if value == "" {
		return
	}
	lines := strings.Split(value, "\n")
	fmt.Fprintf(w, "  %-9s %s\n", label+":", lines[0])
	for _, l := range lines[1:] {
		fmt.Fprintf(w, "  %-9s %s\n", "", l)
	}
}

func mark(s probe.Status) string {
	switch s {
	case probe.Pass:
		return "✓"
	case probe.Warn:
		return "!"
	case probe.Fail:
		return "✗"
	default:
		return "-"
	}
}
