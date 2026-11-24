package plan

import (
	"context"
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

func TestBuildOrder(t *testing.T) {
	probes := Build(Target{Host: "example.com", Port: 443})
	want := []string{"system", "dns", "tcp"}
	for i, p := range probes {
		if p.Name() != want[i] {
			t.Errorf("probe %d = %s, want %s", i, p.Name(), want[i])
		}
	}
}