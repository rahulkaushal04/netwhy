// Package verdict turns probe results into one diagnosis.
package verdict

import (
	"github.com/rahulkaushal04/netwhy/internal/evidence"
	"github.com/rahulkaushal04/netwhy/internal/probe"
)

type Verdict struct {
	OK       bool              `json:"ok"`
	Layer    string            `json:"layer,omitempty"`
	Summary  string            `json:"summary"`
	Evidence evidence.Evidence `json:"evidence"`
	Results  []probe.Result    `json:"results"`
}

func Decide(results []probe.Result) Verdict {
	for _, r := range results {
		if r.Status == probe.Fail {
			return Verdict{
				Layer:    r.Name,
				Summary:  r.Summary,
				Evidence: r.Evidence,
				Results:  results,
			}
		}
	}
	return Verdict{OK: true, Summary: "all checks passed", Results: results}
}