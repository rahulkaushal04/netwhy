package probe

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/rahulkaushal04/netwhy/internal/evidence"
)

const (
	echoOverhead = 28 // IPv4 header (20) plus ICMP header (8)
	smallPayload = 56
	mtuStep      = 8 // the search stops when the answer is this precise
)

var _ Probe = PMTU{}

// PMTU sends ICMP echoes with the don't-fragment bit set and searches for the
// largest size that gets a reply. A path MTU below the interface MTU means a
// tunnel or PPPoE link, or a black hole that drops "fragmentation needed"
// messages. It covers IPv4 only and needs unprivileged ICMP sockets; when
// those are unavailable or the target ignores echo, it passes and says so.
type PMTU struct {
	Path *Path

	// open and ifaceMTU replace the real socket and interface lookup in tests.
	open     func(dst netip.Addr) (echoer, error)
	ifaceMTU func(dst netip.Addr) (int, error)
}

// echoer sends one echo request with the given payload size and reports
// whether the matching reply came back in time.
type echoer interface {
	ping(ctx context.Context, payload int) bool
	Close() error
}

func (PMTU) Name() string { return "pmtu" }

func (PMTU) Deps() []string { return []string{"tcp"} }

func (p PMTU) Run(ctx context.Context) Result {
	res := Result{Name: "pmtu", Status: Pass}
	// Use an IPv4 address even when IPv6 connected fastest, so the result does
	// not flip between runs.
	var dst netip.Addr
	for _, a := range append([]netip.Addr{p.Path.Conn.Addr()}, p.Path.Addrs...) {
		if a.Is4() {
			dst = a
			break
		}
	}
	if !dst.IsValid() {
		res.Summary = "path MTU is only checked for IPv4"
		return res
	}
	open, ifaceMTU := p.open, p.ifaceMTU
	if open == nil {
		open = openEchoer
	}
	if ifaceMTU == nil {
		ifaceMTU = interfaceMTU
	}

	start := time.Now()
	mtu, err := ifaceMTU(dst)
	if err != nil {
		res.Summary = "path MTU not measured: could not find the interface MTU"
		res.Evidence.Error = err.Error()
		return res
	}
	e, err := open(dst)
	if err != nil {
		res.Summary = "path MTU not measured: ICMP echo is not available (on Linux, allow it with net.ipv4.ping_group_range)"
		res.Evidence.Error = err.Error()
		return res
	}
	defer e.Close()

	if !e.ping(ctx, smallPayload) {
		res.Summary = "path MTU not measured: the target does not answer ICMP echo"
		res.Evidence = evidence.Evidence{Sent: "ICMP echo to " + dst.String(), Duration: time.Since(start)}
		return res
	}

	limit := mtu - echoOverhead
	best := largestPassing(smallPayload, limit, func(n int) bool { return e.ping(ctx, n) })
	res.Evidence = evidence.Evidence{
		Sent:     fmt.Sprintf("ICMP echo to %s with DF set, payloads up to %d bytes", dst, limit),
		Received: fmt.Sprintf("largest answered payload %d bytes (path MTU about %d), interface MTU %d", best, best+echoOverhead, mtu),
		Duration: time.Since(start),
	}

	switch {
	case ctx.Err() != nil:
		res.Summary = "path MTU not measured: ran out of time"
	case best >= limit:
		res.Summary = fmt.Sprintf("path MTU is at least the interface MTU (%d)", mtu)
	default:
		res.Status = Warn
		res.Summary = fmt.Sprintf("packets above about %d bytes get no reply, below the interface MTU of %d: a tunnel, PPPoE, or a black hole that drops ICMP fragmentation-needed. If large transfers stall, lower the MTU or clamp MSS", best+echoOverhead, mtu)
	}
	return res
}

// largestPassing returns the largest n in [lo, hi] for which ok(n) is true,
// to within mtuStep, assuming ok(lo) is true and ok is true up to a threshold.
// The top size is tried twice so one lost packet does not lower the answer.
func largestPassing(lo, hi int, ok func(int) bool) int {
	if ok(hi) || ok(hi) {
		return hi
	}
	for hi-lo > mtuStep {
		mid := (lo + hi) / 2
		if ok(mid) {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo
}

// interfaceMTU returns the MTU of the interface the kernel uses to reach dst.
func interfaceMTU(dst netip.Addr) (int, error) {
	c, err := net.Dial("udp", netip.AddrPortFrom(dst, 9).String())
	if err != nil {
		return 0, err
	}
	local := c.LocalAddr().(*net.UDPAddr).IP
	c.Close()

	ifaces, err := net.Interfaces()
	if err != nil {
		return 0, err
	}
	for _, ifc := range ifaces {
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok && ipn.IP.Equal(local) {
				return ifc.MTU, nil
			}
		}
	}
	return 0, fmt.Errorf("no interface has address %s", local)
}
