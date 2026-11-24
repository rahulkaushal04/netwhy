package probe

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"
)

// fakeEcho answers every echo up to max payload bytes, like a path whose MTU
// is max plus the headers.
type fakeEcho struct{ max int }

func (f fakeEcho) ping(_ context.Context, n int) bool { return n <= f.max }
func (f fakeEcho) Close() error                       { return nil }

func runPMTU(addr string, e echoer, openErr error, mtu int) Result {
	p := PMTU{
		Path:     &Path{Conn: netip.MustParseAddrPort(addr)},
		open:     func(netip.Addr) (echoer, error) { return e, openErr },
		ifaceMTU: func(netip.Addr) (int, error) { return mtu, nil },
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return p.Run(ctx)
}

func TestPMTU(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		echo    echoer
		openErr error
		want    Status
		in      string
	}{
		{"full MTU", "192.0.2.1:443", fakeEcho{max: 1472}, nil, Pass, "at least the interface MTU"},
		{"black hole at 1400", "192.0.2.1:443", fakeEcho{max: 1372}, nil, Warn, "below the interface MTU of 1500"},
		{"target ignores ping", "192.0.2.1:443", fakeEcho{max: 0}, nil, Pass, "does not answer ICMP echo"},
		{"no ICMP sockets", "192.0.2.1:443", nil, errors.New("permission denied"), Pass, "not available"},
		{"IPv6 is not checked", "[2001:db8::1]:443", fakeEcho{max: 1472}, nil, Pass, "only checked for IPv4"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := runPMTU(tc.addr, tc.echo, tc.openErr, 1500)
			if got.Status != tc.want || !strings.Contains(got.Summary, tc.in) {
				t.Errorf("got %s %q, want %s containing %q", got.Status, got.Summary, tc.want, tc.in)
			}
		})
	}
}

func TestLargestPassing(t *testing.T) {
	const lo, hi = 56, 1472
	for threshold := lo; threshold < hi; threshold += 37 {
		got := largestPassing(lo, hi, func(n int) bool { return n <= threshold })
		if got > threshold || threshold-got >= mtuStep {
			t.Errorf("threshold %d: got %d, want within %d below it", threshold, got, mtuStep)
		}
	}
	if got := largestPassing(lo, hi, func(int) bool { return true }); got != hi {
		t.Errorf("everything passes: got %d, want %d", got, hi)
	}
}

func TestLargestPassingRetriesTheTopSize(t *testing.T) {
	calls := 0
	got := largestPassing(56, 1472, func(n int) bool {
		calls++
		return calls == 2 // the first try of the top size is lost, the second works
	})
	if got != 1472 {
		t.Errorf("got %d, want 1472: one lost packet must not lower the answer", got)
	}
}

func TestPMTUUsesIPv4WhenIPv6ConnectedFirst(t *testing.T) {
	p := PMTU{
		Path:     &Path{Conn: netip.MustParseAddrPort("[2001:db8::1]:443"), Addrs: []netip.Addr{netip.MustParseAddr("2001:db8::1"), netip.MustParseAddr("192.0.2.1")}},
		open:     func(netip.Addr) (echoer, error) { return fakeEcho{max: 1372}, nil },
		ifaceMTU: func(netip.Addr) (int, error) { return 1500, nil },
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if got := p.Run(ctx); got.Status != Warn {
		t.Errorf("got %s %q, want the IPv4 address to be measured", got.Status, got.Summary)
	}
}
