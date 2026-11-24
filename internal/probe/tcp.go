package probe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rahulkaushal04/netwhy/internal/evidence"
)

// maxPerFamily keeps a host with many records from using up the time budget.
const maxPerFamily = 3

var _ Probe = TCP{}

// TCP connects to every address DNS found, IPv4 and IPv6 at once. It warns when
// one family works and the other does not, because clients that prefer the
// broken one become slow.
type TCP struct {
	Port int
	Path *Path
}

func (TCP) Name() string { return "tcp" }

func (TCP) Deps() []string { return []string{"dns"} }

type attempt struct {
	addr netip.AddrPort
	rtt  time.Duration
	err  error
}

type family struct {
	name  string
	tries []attempt
}

func (p TCP) Run(ctx context.Context) Result {
	res := Result{Name: "tcp"}

	var v4, v6 []netip.Addr
	for _, a := range p.Path.Addrs {
		if a.Is4() {
			v4 = append(v4, a)
		} else {
			v6 = append(v6, a)
		}
	}

	start := time.Now()
	families := []family{
		{name: "IPv4", tries: dialAll(ctx, firstN(v4, maxPerFamily), p.Port)},
		{name: "IPv6", tries: dialAll(ctx, firstN(v6, maxPerFamily), p.Port)},
	}

	var lines, working []string
	var failed []family
	var best attempt
	for _, f := range families {
		if len(f.tries) == 0 {
			continue
		}
		ok := false
		for _, t := range f.tries {
			if t.err != nil {
				lines = append(lines, fmt.Sprintf("%s: failed: %v", t.addr, t.err))
				continue
			}
			lines = append(lines, fmt.Sprintf("%s: connected in %s", t.addr, t.rtt.Round(time.Millisecond)))
			if !best.addr.IsValid() || t.rtt < best.rtt {
				best = t
			}
			ok = true
		}
		if ok {
			working = append(working, f.name)
		} else {
			failed = append(failed, f)
		}
	}

	res.Evidence = evidence.Evidence{
		Sent:     fmt.Sprintf("TCP connect to port %d of %s", p.Port, joinAddrs(p.Path.Addrs)),
		Received: strings.Join(lines, "\n"),
		Duration: time.Since(start),
	}

	switch {
	case len(working) == 0:
		res.Status = Fail
		if len(failed) == 0 {
			res.Summary = "no addresses to connect to"
			break
		}
		res.Evidence.Error = failed[0].tries[0].err.Error()
		parts := make([]string, len(failed))
		for i, f := range failed {
			kind, hint := classifyDial(f.tries[0].err, p.Port)
			parts[i] = fmt.Sprintf("%s %s: %s", f.name, kind, hint)
		}
		res.Summary = strings.Join(parts, "; ")
	case len(failed) > 0:
		res.Status = Warn
		kind, _ := classifyDial(failed[0].tries[0].err, p.Port)
		res.Summary = fmt.Sprintf("%s failed (%s) but %s works", failed[0].name, kind, working[0])
		if failed[0].name == "IPv6" {
			res.Summary += "; clients that try IPv6 first will be slow"
		}
		p.Path.Conn = best.addr
	default:
		res.Status = Pass
		res.Summary = "connected over " + strings.Join(working, " and ")
		p.Path.Conn = best.addr
	}
	return res
}

func dialAll(ctx context.Context, addrs []netip.Addr, port int) []attempt {
	out := make([]attempt, len(addrs))
	var wg sync.WaitGroup
	for i, a := range addrs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ap := netip.AddrPortFrom(a, uint16(port))
			start := time.Now()
			var d net.Dialer
			conn, err := d.DialContext(ctx, "tcp", ap.String())
			out[i] = attempt{addr: ap, rtt: time.Since(start), err: err}
			if err == nil {
				conn.Close()
			}
		}()
	}
	wg.Wait()
	return out
}

func firstN(a []netip.Addr, n int) []netip.Addr {
	if len(a) > n {
		return a[:n]
	}
	return a
}

// Specific system errors are checked before the generic timeout.
func classifyDial(err error, port int) (kind, hint string) {
	var ne net.Error
	switch {
	case errors.Is(err, syscall.ECONNREFUSED):
		return "connection refused (RST)", fmt.Sprintf("the host is reachable but nothing accepts connections on port %d, or a firewall is rejecting", port)
	case errors.Is(err, syscall.ENETUNREACH), errors.Is(err, syscall.EHOSTUNREACH):
		return "no route to host", "the network or host is unreachable"
	case errors.As(err, &ne) && ne.Timeout():
		return "connection timed out", "packets are being silently dropped (firewall DROP or a routing problem)"
	default:
		return "connection failed", err.Error()
	}
}
