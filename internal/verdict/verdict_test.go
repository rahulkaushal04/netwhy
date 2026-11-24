package verdict

import (
	"testing"

	"github.com/rahulkaushal04/netwhy/internal/probe"
)

func res(name string, s probe.Status) probe.Result {
	return probe.Result{Name: name, Status: s, Summary: name + " " + string(s)}
}

func TestDecide(t *testing.T) {
	tests := []struct {
		name      string
		results   []probe.Result
		wantOK    bool
		wantLayer string
	}{
		{
			name:    "all pass",
			results: []probe.Result{res("system", probe.Pass), res("dns", probe.Pass), res("tcp", probe.Pass)},
			wantOK:  true,
		},
		{
			name:      "system fails, rest skipped",
			results:   []probe.Result{res("system", probe.Fail), res("dns", probe.Skipped), res("tcp", probe.Skipped)},
			wantLayer: "system",
		},
		{
			name:      "dns fails, tcp skipped",
			results:   []probe.Result{res("system", probe.Pass), res("dns", probe.Fail), res("tcp", probe.Skipped)},
			wantLayer: "dns",
		},
		{
			name:      "tcp fails",
			results:   []probe.Result{res("system", probe.Pass), res("dns", probe.Pass), res("tcp", probe.Fail)},
			wantLayer: "tcp",
		},
		{
			name:      "first failure wins",
			results:   []probe.Result{res("system", probe.Pass), res("dns", probe.Fail), res("tcp", probe.Fail)},
			wantLayer: "dns",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Decide(tc.results)
			if got.OK != tc.wantOK {
				t.Errorf("OK = %v, want %v", got.OK, tc.wantOK)
			}
			if got.Layer != tc.wantLayer {
				t.Errorf("Layer = %q, want %q", got.Layer, tc.wantLayer)
			}
		})
	}
}
