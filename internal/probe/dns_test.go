package probe

import (
	"context"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// startDNS runs a UDP DNS server that answers 192.0.2.1 for ok.test and
// NXDOMAIN for every other name. It returns the server's address.
func startDNS(t *testing.T) string {
	return startDNSWith(t, func(name string) string {
		if name == "ok.test." {
			return "192.0.2.1"
		}
		return ""
	})
}

// startDNSWith runs a UDP DNS server. answer maps a name to the IPv4 address
// to return, or "" for NXDOMAIN. It returns the server's address.
func startDNSWith(t *testing.T, answer func(name string) string) string {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	srv := &dns.Server{
		PacketConn:        pc,
		NotifyStartedFunc: func() { close(started) },
		Handler: dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
			m := new(dns.Msg)
			m.SetReply(r)
			q := r.Question[0]
			ip := answer(q.Name)
			switch {
			case ip == "":
				m.Rcode = dns.RcodeNameError
			case q.Qtype == dns.TypeA:
				m.Answer = []dns.RR{&dns.A{
					Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
					A:   net.ParseIP(ip),
				}}
			}
			w.WriteMsg(m)
		}),
	}
	go srv.ActivateAndServe()
	<-started
	t.Cleanup(func() { srv.Shutdown() })
	return pc.LocalAddr().String()
}

// deadResolver returns the address of a UDP port nobody listens on.
func deadResolver(t *testing.T) string {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := pc.LocalAddr().String()
	pc.Close()
	return addr
}

func noSystem(context.Context, string) []netip.Addr { return nil }

func runDNS(t *testing.T, p DNS) Result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return p.Run(ctx)
}

func TestDNSResolves(t *testing.T) {
	path := &Path{}
	got := runDNS(t, DNS{Host: "ok.test", Resolvers: []string{startDNS(t)}, Path: path, lookup: noSystem})

	if got.Status != Pass {
		t.Fatalf("status = %s (%s), want pass", got.Status, got.Summary)
	}
	if want := []netip.Addr{netip.MustParseAddr("192.0.2.1")}; !slices.Equal(path.Addrs, want) {
		t.Errorf("Path.Addrs = %v, want %v", path.Addrs, want)
	}
}

func TestDNSNXDOMAIN(t *testing.T) {
	got := runDNS(t, DNS{Host: "missing.test", Resolvers: []string{startDNS(t)}, Path: &Path{}, lookup: noSystem})

	if got.Status != Fail || !strings.Contains(got.Summary, "NXDOMAIN") {
		t.Errorf("got %s %q, want fail mentioning NXDOMAIN", got.Status, got.Summary)
	}
}

func TestDNSWarnsWhenOneResolverIsDead(t *testing.T) {
	path := &Path{}
	got := runDNS(t, DNS{Host: "ok.test", Resolvers: []string{deadResolver(t), startDNS(t)}, Path: path, lookup: noSystem})

	if got.Status != Warn || len(path.Addrs) != 1 {
		t.Errorf("got %s %q with %d addrs, want warn with 1", got.Status, got.Summary, len(path.Addrs))
	}
}

func TestDNSAllResolversDead(t *testing.T) {
	got := runDNS(t, DNS{Host: "ok.test", Resolvers: []string{deadResolver(t)}, Path: &Path{}, lookup: noSystem})

	if got.Status != Fail || !strings.Contains(got.Summary, "no DNS resolver answered") {
		t.Errorf("got %s %q, want fail: no resolver answered", got.Status, got.Summary)
	}
}

func TestDNSFallsBackToSystemResolver(t *testing.T) {
	path := &Path{}
	want := netip.MustParseAddr("198.51.100.7")
	lookup := func(context.Context, string) []netip.Addr { return []netip.Addr{want} }

	got := runDNS(t, DNS{Host: "missing.test", Resolvers: []string{startDNS(t)}, Path: path, lookup: lookup})

	if got.Status != Warn || !slices.Equal(path.Addrs, []netip.Addr{want}) {
		t.Errorf("got %s %q addrs %v, want warn with %v", got.Status, got.Summary, path.Addrs, want)
	}
}

func TestDNSIPNeedsNoLookup(t *testing.T) {
	path := &Path{}
	got := runDNS(t, DNS{Host: "192.0.2.9", IsIP: true, Path: path})

	if got.Status != Pass || !slices.Equal(path.Addrs, []netip.Addr{netip.MustParseAddr("192.0.2.9")}) {
		t.Errorf("got %s, addrs %v", got.Status, path.Addrs)
	}
}

func TestSystemResolvers(t *testing.T) {
	file := filepath.Join(t.TempDir(), "resolv.conf")
	if err := os.WriteFile(file, []byte("nameserver 192.0.2.53\nnameserver 2001:db8::53\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := systemResolvers(file)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"192.0.2.53:53", "[2001:db8::53]:53"}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSystemResolversNone(t *testing.T) {
	file := filepath.Join(t.TempDir(), "resolv.conf")
	if err := os.WriteFile(file, []byte("search example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := systemResolvers(file); err == nil {
		t.Error("want an error when no nameserver is configured")
	}
}

func TestDNSDetectsHijacking(t *testing.T) {
	hijacker := startDNSWith(t, func(string) string { return "192.0.2.1" })

	got := runDNS(t, DNS{Host: "ok.test", Resolvers: []string{hijacker}, Path: &Path{}, lookup: noSystem})

	if got.Status != Warn || !strings.Contains(got.Summary, "NXDOMAIN hijacking") {
		t.Errorf("got %s %q, want warn: NXDOMAIN hijacking", got.Status, got.Summary)
	}
}

func TestDNSDetectsBlockedName(t *testing.T) {
	blocker := startDNSWith(t, func(name string) string {
		if name == "ok.test." {
			return "0.0.0.0"
		}
		return ""
	})

	got := runDNS(t, DNS{Host: "ok.test", Resolvers: []string{blocker}, Path: &Path{}, lookup: noSystem})

	if got.Status != Warn || !strings.Contains(got.Summary, "blocking the name") {
		t.Errorf("got %s %q, want warn: blocking the name", got.Status, got.Summary)
	}
}

func TestDNSDetectsPrivateAnswerFromOneResolver(t *testing.T) {
	good := startDNS(t)
	poisoned := startDNSWith(t, func(name string) string {
		if name == "ok.test." {
			return "10.0.0.5"
		}
		return ""
	})

	got := runDNS(t, DNS{Host: "ok.test", Resolvers: []string{good, poisoned}, Path: &Path{}, lookup: noSystem})

	if got.Status != Warn || !strings.Contains(got.Summary, "only private addresses") {
		t.Errorf("got %s %q, want warn: only private addresses", got.Status, got.Summary)
	}
}

func TestDNSAllPrivateIsFine(t *testing.T) {
	internal := startDNSWith(t, func(name string) string {
		if name == "ok.test." {
			return "10.0.0.5"
		}
		return ""
	})

	got := runDNS(t, DNS{Host: "ok.test", Resolvers: []string{internal}, Path: &Path{}, lookup: noSystem})

	if got.Status != Pass {
		t.Errorf("got %s %q, want pass for an internal name", got.Status, got.Summary)
	}
}
