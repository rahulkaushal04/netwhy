package render

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/rahulkaushal04/netwhy/internal/evidence"
	"github.com/rahulkaushal04/netwhy/internal/probe"
	"github.com/rahulkaushal04/netwhy/internal/verdict"
)

func TestHumanFailure(t *testing.T) {
	v := verdict.Decide([]probe.Result{
		{Name: "system", Status: probe.Pass, Summary: "up"},
		{Name: "dns", Status: probe.Fail, Summary: "no such host", Evidence: evidence.Evidence{
			Sent: "query", Received: "first\nsecond", Error: "boom", Duration: 12 * time.Millisecond,
		}},
		{Name: "tcp", Status: probe.Skipped, Summary: "skipped"},
	})

	var b bytes.Buffer
	Human(&b, "example.com", v)
	got := b.String()

	for _, want := range []string{
		"✓ system",
		"✗ dns",
		"- tcp",
		"Verdict: the dns layer failed",
		"  received: first\n",
		"\n" + strings.Repeat(" ", 12) + "second\n",
		"  error:    boom\n",
		"  took:     12ms\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output is missing %q:\n%s", want, got)
		}
	}
}

func TestHumanWarning(t *testing.T) {
	v := verdict.Decide([]probe.Result{
		{Name: "tcp", Status: probe.Warn, Summary: "IPv6 failed", Evidence: evidence.Evidence{Sent: "connect"}},
	})

	var b bytes.Buffer
	Human(&b, "example.com", v)
	got := b.String()

	for _, want := range []string{"! tcp", "Verdict: reachable, with 1 warning(s)", "Warning (tcp): IPv6 failed"} {
		if !strings.Contains(got, want) {
			t.Errorf("output is missing %q:\n%s", want, got)
		}
	}
}
