// Package plan parses the target, picks the probes that apply, and runs them.
package plan

import (
	"context"
	"time"

	"github.com/rahulkaushal04/netwhy/internal/probe"
)

// Build returns the probes in dependency order: system, gateway, dns, portal,
// tcp, pmtu, then tls for https and http for http and https. A bare host:port
// stops after pmtu. The portal check is the only request to a host the user
// did not name, so it is left out unless portal is true.
func Build(t Target, portal bool) []probe.Probe {
	path := &probe.Path{}
	probes := []probe.Probe{
		probe.System{},
		probe.Gateway{},
		probe.DNS{Host: t.Host, IsIP: t.IsIP, Path: path},
	}
	tcpDep := "dns"
	if portal {
		probes = append(probes, probe.Portal{})
		tcpDep = "portal"
	}
	probes = append(probes, probe.TCP{Port: t.Port, Path: path, Dep: tcpDep}, probe.PMTU{Path: path})
	last := "tcp"
	if t.Scheme == "https" {
		probes = append(probes, probe.TLS{Host: t.Host, Path: path})
		last = "tls"
	}
	if t.Scheme == "http" || t.Scheme == "https" {
		probes = append(probes, probe.HTTP{URL: t.URL(), Dep: last})
	}
	return probes
}

// Run executes probes in order. A probe whose dependency did not work is skipped,
// so one fault gives one failure. Probes must come after their dependencies.
func Run(ctx context.Context, probes []probe.Probe, perProbe time.Duration) []probe.Result {
	results := make([]probe.Result, 0, len(probes))
	status := map[string]probe.Status{}

	for _, p := range probes {
		var res probe.Result
		if blocker := blockedBy(p, status); blocker != "" {
			res = probe.Result{
				Name:    p.Name(),
				Status:  probe.Skipped,
				Summary: "skipped because " + blocker + " did not work",
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
		if !status[d].Works() {
			return d
		}
	}
	return ""
}
