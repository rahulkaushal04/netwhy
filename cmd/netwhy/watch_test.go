package main

import (
	"strings"
	"testing"
	"time"

	"github.com/rahulkaushal04/netwhy/internal/probe"
	"github.com/rahulkaushal04/netwhy/internal/verdict"
)

func decide(statuses ...probe.Status) verdict.Verdict {
	names := []string{"system", "dns", "tcp"}
	var results []probe.Result
	for i, s := range statuses {
		results = append(results, probe.Result{Name: names[i], Status: s, Summary: names[i] + " " + string(s)})
	}
	return verdict.Decide(results)
}

func TestWatchLine(t *testing.T) {
	now := time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)

	sig, line := watchLine("", decide(probe.Pass, probe.Pass, probe.Pass), now)
	if line != "15:04:05 all checks passed" {
		t.Errorf("first check: line = %q", line)
	}

	if _, line := watchLine(sig, decide(probe.Pass, probe.Pass, probe.Pass), now); line != "" {
		t.Errorf("unchanged result printed %q", line)
	}

	_, line = watchLine(sig, decide(probe.Pass, probe.Fail, probe.Skipped), now)
	if !strings.Contains(line, "the dns layer failed") {
		t.Errorf("change to failure: line = %q", line)
	}
}
