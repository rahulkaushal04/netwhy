package plan

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/rahulkaushal04/netwhy/internal/probe"
)

type fake struct {
	name   string
	deps   []string
	status probe.Status
	ran    *bool
}

func (f fake) Name() string   { return f.name }
func (f fake) Deps() []string { return f.deps }
func (f fake) Run(context.Context) probe.Result {
	if f.ran != nil {
		*f.ran = true
	}
	return probe.Result{Name: f.name, Status: f.status}
}

func TestRunSkipsAfterFailure(t *testing.T) {
	var ran bool
	probes := []probe.Probe{
		fake{name: "a", status: probe.Fail},
		fake{name: "b", deps: []string{"a"}, status: probe.Pass, ran: &ran},
	}

	got := Run(context.Background(), probes, time.Second)

	if got[0].Status != probe.Fail || got[1].Status != probe.Skipped {
		t.Errorf("statuses = %s, %s; want fail, skipped", got[0].Status, got[1].Status)
	}
	if ran {
		t.Error("probe b ran although its dependency failed")
	}
}

func TestRunWarnDoesNotBlock(t *testing.T) {
	var ran bool
	probes := []probe.Probe{
		fake{name: "a", status: probe.Warn},
		fake{name: "b", deps: []string{"a"}, status: probe.Pass, ran: &ran},
	}

	got := Run(context.Background(), probes, time.Second)

	if !ran || got[1].Status != probe.Pass {
		t.Errorf("b should run after a warning, got %s (ran=%v)", got[1].Status, ran)
	}
}

func TestRunSkipsOnUnknownDependency(t *testing.T) {
	probes := []probe.Probe{fake{name: "a", deps: []string{"missing"}, status: probe.Pass}}

	if got := Run(context.Background(), probes, time.Second); got[0].Status != probe.Skipped {
		t.Errorf("status = %s, want skipped", got[0].Status)
	}
}

func TestBuild(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"https://example.com", []string{"system", "gateway", "dns", "tcp", "tls", "http"}},
		{"http://example.com", []string{"system", "gateway", "dns", "tcp", "http"}},
		{"example.com:22", []string{"system", "gateway", "dns", "tcp"}},
	}
	for _, tc := range tests {
		target, err := ParseTarget(tc.in)
		if err != nil {
			t.Fatal(err)
		}
		probes := Build(target)

		var names []string
		for _, p := range probes {
			for _, d := range p.Deps() {
				if !slices.Contains(names, d) {
					t.Errorf("%s: %s depends on %s, which is not before it", tc.in, p.Name(), d)
				}
			}
			names = append(names, p.Name())
		}
		if !slices.Equal(names, tc.want) {
			t.Errorf("%s: probes = %v, want %v", tc.in, names, tc.want)
		}
	}
}
