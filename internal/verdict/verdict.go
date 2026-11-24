// Package verdict turns probe results into one diagnosis.
package verdict

import (
	"fmt"

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
			return Verdict{
				Layer:    r.Name,
				Summary:  r.Summary,
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
