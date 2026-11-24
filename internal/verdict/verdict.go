// Package verdict turns probe results into one diagnosis.
package verdict

import (
	"fmt"
	"slices"

	"github.com/rahulkaushal04/netwhy/internal/evidence"
	"github.com/rahulkaushal04/netwhy/internal/probe"
)

type Verdict struct {
	OK       bool              `json:"ok"`
	Layer    string            `json:"layer,omitempty"`
	Summary  string            `json:"summary"`
	Evidence evidence.Evidence `json:"evidence"`
	Warnings []probe.Result    `json:"warnings,omitempty"`
	Results  []probe.Result    `json:"results"`
}

// Decide returns the first failure as the verdict. Results must be in dependency
// order with later probes Skipped, as plan.Run guarantees, so it is the root cause.
func Decide(results []probe.Result) Verdict {
	var warnings []probe.Result
	for _, r := range results {
		if r.Status == probe.Warn {
			warnings = append(warnings, r)
		}
	}
	for _, r := range results {
		if r.Status == probe.Fail {
			summary := r.Summary
			// A black hole drops the large handshake packets, so it shows up as
			// a TLS or HTTP failure. The pmtu warning is the real clue.
			if (r.Name == "tls" || r.Name == "http") && slices.ContainsFunc(warnings, func(w probe.Result) bool { return w.Name == "pmtu" }) {
				summary += "; the pmtu warning suggests large packets are dropped, so a path MTU black hole may be the cause"
			}
			return Verdict{
				Layer:    r.Name,
				Summary:  summary,
				Evidence: r.Evidence,
				Warnings: warnings,
				Results:  results,
			}
		}
	}
	summary := "all checks passed"
	if len(warnings) > 0 {
		summary = fmt.Sprintf("reachable, with %d warning(s)", len(warnings))
	}
	return Verdict{OK: true, Summary: summary, Warnings: warnings, Results: results}
}
