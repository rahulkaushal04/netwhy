package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/rahulkaushal04/netwhy/internal/verdict"
)

// Markdown writes the verdict as Markdown, ready to paste into a ticket or
// an incident channel.
func Markdown(w io.Writer, target string, v verdict.Verdict) {
	if v.OK {
		fmt.Fprintf(w, "**netwhy** `%s`: %s\n\n", target, v.Summary)
	} else {
		fmt.Fprintf(w, "**netwhy** `%s`: the **%s** layer failed\n\n", target, v.Layer)
	}

	fmt.Fprintln(w, "| Layer | Status | Result |")
	fmt.Fprintln(w, "|---|---|---|")
	for _, r := range v.Results {
		fmt.Fprintf(w, "| %s | %s | %s |\n", r.Name, r.Status, strings.ReplaceAll(r.Summary, "|", `\|`))
	}

	if !v.OK {
		fmt.Fprintf(w, "\nEvidence for %s:\n\n```\n", v.Layer)
		writeEvidence(w, v.Evidence)
		fmt.Fprintln(w, "```")
	}
	for _, r := range v.Warnings {
		fmt.Fprintf(w, "\nWarning from %s:\n\n```\n", r.Name)
		writeEvidence(w, r.Evidence)
		fmt.Fprintln(w, "```")
	}
}
