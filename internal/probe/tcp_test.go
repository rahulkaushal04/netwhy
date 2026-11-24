package probe

import (
	"context"
	"net"
	"net/netip"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return false }

func TestClassifyDial(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"refused", &net.OpError{Op: "dial", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}, "connection refused (RST)"},
		{"network unreachable", &net.OpError{Op: "dial", Err: os.NewSyscallError("connect", syscall.ENETUNREACH)}, "no route to host"},
		{"host unreachable", &net.OpError{Op: "dial", Err: os.NewSyscallError("connect", syscall.EHOSTUNREACH)}, "no route to host"},
		{"timeout", &net.OpError{Op: "dial", Err: timeoutErr{}}, "connection timed out"},
		{"other", os.ErrInvalid, "connection failed"},
	}
	for _, tc := range tests {
		if kind, _ := classifyDial(tc.err, 443); kind != tc.want {
			t.Errorf("%s: kind = %q, want %q", tc.name, kind, tc.want)
		}
	}
}

// listen opens a TCP listener on loopback and returns its port.
func listen(t *testing.T) (net.Listener, int) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	return l, l.Addr().(*net.TCPAddr).Port
}

// closedPort returns a loopback port with nothing listening.
func closedPort(t *testing.T) int {
	t.Helper()
	l, port := listen(t)
	l.Close()
	return port
}

func runTCP(p TCP) Result {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return p.Run(ctx)
}

func TestTCPConnects(t *testing.T) {
	_, port := listen(t)
	path := &Path{Addrs: []netip.Addr{netip.MustParseAddr("127.0.0.1")}}

	got := runTCP(TCP{Port: port, Path: path})

	if got.Status != Pass {
		t.Fatalf("status = %s (%s), want pass", got.Status, got.Summary)
	}
	if want := netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), uint16(port)); path.Conn != want {
		t.Errorf("Path.Conn = %v, want %v", path.Conn, want)
	}
}

func TestTCPRefused(t *testing.T) {
	path := &Path{Addrs: []netip.Addr{netip.MustParseAddr("127.0.0.1")}}

	got := runTCP(TCP{Port: closedPort(t), Path: path})

	if got.Status != Fail || !strings.Contains(got.Summary, "refused") {
		t.Errorf("got %s %q, want fail: refused", got.Status, got.Summary)
	}
}

func TestTCPWarnsWhenOneFamilyFails(t *testing.T) {
	_, port := listen(t) // listens on IPv4 only
	path := &Path{Addrs: []netip.Addr{netip.MustParseAddr("127.0.0.1"), netip.MustParseAddr("::1")}}

	got := runTCP(TCP{Port: port, Path: path})

	if got.Status != Warn || !strings.Contains(got.Summary, "IPv6 failed") {
		t.Errorf("got %s %q, want warn: IPv6 failed", got.Status, got.Summary)
	}
	if !path.Conn.IsValid() {
		t.Error("Path.Conn should hold the working IPv4 address")
	}
}

func TestTCPNoAddresses(t *testing.T) {
	got := runTCP(TCP{Port: 443, Path: &Path{}})

	if got.Status != Fail || !strings.Contains(got.Summary, "no addresses") {
		t.Errorf("got %s %q, want fail: no addresses", got.Status, got.Summary)
	}
}
