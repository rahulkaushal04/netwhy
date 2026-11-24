package probe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/rahulkaushal04/netwhy/internal/evidence"
)

var (
	errNoGateway   = errors.New("default route has no gateway")
	errUnsupported = errors.New("not supported on this OS")
)

const neighbourWait = time.Second

var _ Probe = Gateway{}

// Gateway checks that the IPv4 default gateway has a resolved ARP entry. An
// unresolved entry is a Layer 2 problem, not "the internet is down". Linux only:
// other systems hide the table from unsigned programs, so it passes there.
// The probe sends one byte to the gateway's UDP discard port to trigger ARP.
type Gateway struct{}

func (Gateway) Name() string { return "gateway" }

func (Gateway) Deps() []string { return []string{"system"} }

func (Gateway) Run(ctx context.Context) Result {
	res := Result{Name: "gateway"}
	start := time.Now()
	ev := evidence.Evidence{Sent: "find the default gateway"}

	gw, iface, err := defaultGateway(ctx)
	switch {
	case errors.Is(err, errNoGateway):
		res.Status = Pass
		res.Summary = "default route is directly connected (VPN or point-to-point), no gateway to check"
	case errors.Is(err, errUnsupported):
		res.Status = Pass
		res.Summary = "gateway ARP check is only available on Linux, skipped"
	case err != nil:
		ev.Error = err.Error()
		res.Status = Warn
		res.Summary = "could not determine the default gateway"
	}
	if res.Status != "" {
		ev.Duration = time.Since(start)
		res.Evidence = ev
		return res
	}

	poke(gw)
	state, mac, err := settle(ctx, gw, iface)
	ev.Sent = fmt.Sprintf("neighbour table lookup for %s on %s", gw, iface)
	ev.Received = fmt.Sprintf("state %s %s", state, mac)
	ev.Duration = time.Since(start)
	res.Evidence = ev
	if err != nil {
		ev.Error = err.Error()
		res.Evidence = ev
		res.Status = Warn
		res.Summary = "could not read the neighbour table"
		return res
	}
	res.Status, res.Summary = gatewayResult(gw, iface, state, mac)
	return res
}

func gatewayResult(gw netip.Addr, iface, state, mac string) (Status, string) {
	if neighbourHealthy(state) {
		return Pass, fmt.Sprintf("gateway %s on %s is reachable (%s, %s)", gw, iface, state, mac)
	}
	return Fail, fmt.Sprintf("gateway %s on %s did not answer ARP (state %s): a Layer 2 problem between you and your router, not the internet", gw, iface, state)
}

// STALE and DELAY are normal for idle entries.
func neighbourHealthy(state string) bool {
	switch state {
	case "reachable", "stale", "delay", "probe", "permanent", "noarp", "present":
		return true
	}
	return false
}

func settle(ctx context.Context, gw netip.Addr, iface string) (state, mac string, err error) {
	deadline := time.Now().Add(neighbourWait)
	for {
		state, mac, err = neighbour(ctx, gw, iface)
		if err != nil || neighbourHealthy(state) || time.Now().After(deadline) {
			return state, mac, err
		}
		select {
		case <-ctx.Done():
			return state, mac, err
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func poke(gw netip.Addr) {
	c, err := net.DialTimeout("udp", netip.AddrPortFrom(gw, 9).String(), time.Second)
	if err != nil {
		return
	}
	c.Write([]byte{0})
	c.Close()
}
