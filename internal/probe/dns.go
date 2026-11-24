package probe

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/rahulkaushal04/netwhy/internal/evidence"
)

var _ Probe = DNS{}

type DNS struct {
	Host string
	IsIP bool
}

func (DNS) Name() string   { return "dns" }
func (DNS) Deps() []string { return []string{"system"} }

func (p DNS) Run(ctx context.Context) Result {
	res := Result{Name: "dns"}

	if p.IsIP {
		res.Status = Pass
		res.Summary = "target is an IP address, no lookup needed"
		res.Evidence = evidence.Evidence{Sent: "none", Received: p.Host}
		return res
	}

	start := time.Now()
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, p.Host)
	ev := evidence.Evidence{
		Sent:     "lookup " + p.Host + " via system resolver",
		Duration: time.Since(start),
	}

	if err != nil {
		ev.Error = err.Error()

		res.Status = Fail
		res.Evidence = ev

		var dnsErr *net.DNSError
		switch {
		case errors.As(err, &dnsErr) && dnsErr.IsNotFound:
			res.Summary = p.Host + " does not exist (no such host)"
		case errors.As(err, &dnsErr) && dnsErr.IsTimeout:
			res.Summary = "DNS resolver did not answer in time"
		default:
			res.Summary = "DNS lookup failed"
		}
		return res
	}

	ips := make([]string, len(addrs))
	for i, a := range addrs {
		ips[i] = a.IP.String()
	}
	ev.Received = strings.Join(ips, ", ")
	res.Status = Pass
	res.Summary = p.Host + " resolved to " + ev.Received
	res.Evidence = ev
	return res
}
