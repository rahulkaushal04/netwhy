package plan

import (
	"context"
	"time"

	"github.com/rahulkaushal04/netwhy/internal/probe"
)

func Build(t Target) []probe.Probe {
	return []probe.Probe{
		probe.System{},
		probe.DNS{Host: t.Host, IsIP: t.IsIP},
		probe.TCP{Host: t.Host, Port: t.Port},
	}
}

func Run(ctx context.Context, probes []probe.Probe, perProbe time.Duration) []probe.Result {
	results := make([]probe.Result, 0, len(probes))
	status := map[string]probe.Status{}

	for _, p := range probes {
		var res probe.Result
		if blocker := blockedBy(p, status); blocker != "" {
			res = probe.Result{
				Name:    p.Name(),
				Status:  probe.Skipped,
				Summary: "skipped because " + blocker + " did not pass",
			}
		} else {
			pctx, cancel := context.WithTimeout(ctx, perProbe)
			res = p.Run(pctx)
			cancel()
		}
		status[p.Name()] = res.Status
		results = append(results, res)
	}
	return results
}

func blockedBy(p probe.Probe, status map[string]probe.Status) string {
	for _, d := range p.Deps() {
		if status[d] != probe.Pass {
			return d
		}
	}
	return ""
}
