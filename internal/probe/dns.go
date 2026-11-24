package probe

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"

	"github.com/rahulkaushal04/netwhy/internal/evidence"
)

const resolvConf = "/etc/resolv.conf"

var _ Probe = DNS{}

// DNS asks every configured resolver directly, bypassing libc, so one broken
// resolver cannot hide behind a working one. Direct queries skip search domains,
// /etc/hosts and mDNS, so if none answers but the system resolver does, it warns.
type DNS struct {
	Host string
	IsIP bool
	// Resolvers are host:port; empty means read /etc/resolv.conf.
	Resolvers []string
	Path      *Path

	// lookup replaces the system-resolver fallback in tests.
	lookup func(ctx context.Context, host string) []netip.Addr
}

func (DNS) Name() string { return "dns" }

func (DNS) Deps() []string { return []string{"gateway"} }

func (p DNS) Run(ctx context.Context) Result {
	res := Result{Name: "dns"}

	if p.IsIP {
		addr, err := netip.ParseAddr(p.Host)
		if err != nil {
			res.Status = Fail
			res.Summary = "target is not a valid IP address"
			res.Evidence = evidence.Evidence{Sent: "parse " + p.Host, Error: err.Error()}
			return res
		}
		p.Path.Addrs = []netip.Addr{addr.Unmap()}
		res.Status = Pass
		res.Summary = "target is an IP address, no lookup needed"
		res.Evidence = evidence.Evidence{Sent: "none", Received: p.Host}
		return res
	}

	resolvers := p.Resolvers
	if len(resolvers) == 0 {
		var err error
		if resolvers, err = systemResolvers(resolvConf); err != nil {
			res.Status = Fail
			res.Summary = "no DNS resolver is configured"
			res.Evidence = evidence.Evidence{Sent: "read " + resolvConf, Error: err.Error()}
			return res
		}
	}

	start := time.Now()
	answers := queryAll(ctx, resolvers, p.Host)
	status, summary, addrs := summarize(p.Host, answers)
	ev := evidence.Evidence{
		Sent:     "A and AAAA for " + p.Host + " to " + strings.Join(resolvers, ", "),
		Received: describe(answers),
	}

	if len(addrs) == 0 {
		lookup := p.lookup
		if lookup == nil {
			lookup = lookupSystem
		}
		if sys := lookup(ctx, p.Host); len(sys) > 0 {
			addrs = sys
			status = Warn
			summary = p.Host + " was found by the system resolver but not by the configured resolvers (hosts file, search domain or mDNS?)"
			ev.Received += "\nsystem resolver: " + joinAddrs(sys)
		}
	}

	if status != Fail {
		if issues := resolverIssues(ctx, answers); len(issues) > 0 {
			joined := strings.Join(issues, "; ")
			if status == Warn {
				summary += "; " + joined
			} else {
				summary = joined
			}
			status = Warn
			ev.Received += "\n" + strings.Join(issues, "\n")
		}
	}

	ev.Duration = time.Since(start)
	res.Status, res.Summary, res.Evidence = status, summary, ev
	if status != Fail {
		p.Path.Addrs = addrs
	}
	return res
}

// resolverIssues reports resolvers that lie or filter: they answer for a name
// that cannot exist (NXDOMAIN hijacking), answer 0.0.0.0 (a DNS blocker), or
// give private addresses where another resolver gives public ones.
func resolverIssues(ctx context.Context, answers []answer) []string {
	var issues []string
	for _, r := range hijackers(ctx, answers) {
		issues = append(issues, r+" answers for names that do not exist (NXDOMAIN hijacking, typos land on its page)")
	}

	var public, private []string
	for _, a := range answers {
		switch {
		case len(a.addrs) == 0:
		case slices.ContainsFunc(a.addrs, netip.Addr.IsUnspecified):
			issues = append(issues, a.resolver+" answered 0.0.0.0 or ::, so it is blocking the name (DNS filter?)")
		case !slices.ContainsFunc(a.addrs, func(ip netip.Addr) bool { return !isPrivate(ip) }):
			private = append(private, a.resolver)
		default:
			public = append(public, a.resolver)
		}
	}
	if len(public) > 0 {
		for _, r := range private {
			issues = append(issues, r+" returns only private addresses while "+public[0]+" returns public ones")
		}
	}
	return issues
}

func isPrivate(ip netip.Addr) bool {
	return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()
}

// hijackers returns the resolvers that answer with an address for a random
// name under .com, which cannot exist.
func hijackers(ctx context.Context, answers []answer) []string {
	name := fmt.Sprintf("netwhy-%08x.com.", rand.Uint32())
	hijacked := make([]bool, len(answers))
	var wg sync.WaitGroup
	for i, a := range answers {
		if a.err != nil {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, _, err := exchange(ctx, a.resolver, name, dns.TypeA)
			hijacked[i] = err == nil && r.Rcode == dns.RcodeSuccess && len(extractAddrs(r)) > 0
		}()
	}
	wg.Wait()

	var out []string
	for i, h := range hijacked {
		if h {
			out = append(out, answers[i].resolver)
		}
	}
	return out
}

type answer struct {
	resolver string
	addrs    []netip.Addr
	rcode    int // DNS response code, -1 when no reply arrived
	rtt      time.Duration
	err      error // set only when no reply arrived at all
}

func systemResolvers(path string) ([]string, error) {
	cfg, err := dns.ClientConfigFromFile(path)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, s := range cfg.Servers {
		out = append(out, net.JoinHostPort(s, cfg.Port))
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no nameserver in %s", path)
	}
	return out, nil
}

func queryAll(ctx context.Context, resolvers []string, host string) []answer {
	out := make([]answer, len(resolvers))
	var wg sync.WaitGroup
	for i, r := range resolvers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i] = queryResolver(ctx, r, dns.Fqdn(host))
		}()
	}
	wg.Wait()
	return out
}

// queryResolver sets err only when both the A and AAAA queries got no reply.
func queryResolver(ctx context.Context, server, name string) answer {
	a := answer{resolver: server, rcode: -1}
	failures := 0
	for _, qtype := range []uint16{dns.TypeA, dns.TypeAAAA} {
		r, rtt, err := exchange(ctx, server, name, qtype)
		a.rtt += rtt
		if err != nil {
			failures++
			a.err = err
			continue
		}
		if a.rcode == -1 || r.Rcode != dns.RcodeSuccess {
			a.rcode = r.Rcode
		}
		a.addrs = append(a.addrs, extractAddrs(r)...)
	}
	if failures < 2 {
		a.err = nil
	}
	return a
}

// exchange queries over UDP with EDNS0 and retries over TCP when truncated.
func exchange(ctx context.Context, server, name string, qtype uint16) (*dns.Msg, time.Duration, error) {
	m := new(dns.Msg)
	m.SetQuestion(name, qtype)
	m.SetEdns0(1232, false)

	r, rtt, err := (&dns.Client{}).ExchangeContext(ctx, m, server)
	if err == nil && r.Truncated {
		var rtt2 time.Duration
		r, rtt2, err = (&dns.Client{Net: "tcp"}).ExchangeContext(ctx, m, server)
		rtt += rtt2
	}
	return r, rtt, err
}

func extractAddrs(r *dns.Msg) []netip.Addr {
	var out []netip.Addr
	for _, rr := range r.Answer {
		var ip net.IP
		switch v := rr.(type) {
		case *dns.A:
			ip = v.A
		case *dns.AAAA:
			ip = v.AAAA
		default:
			continue
		}
		if a, ok := netip.AddrFromSlice(ip); ok {
			out = append(out, a.Unmap())
		}
	}
	return out
}

func summarize(host string, answers []answer) (Status, string, []netip.Addr) {
	var addrs []netip.Addr
	seen := map[netip.Addr]bool{}
	var bad []string // resolvers that gave no usable answer
	nxdomain, silent, empty := 0, 0, 0

	for _, a := range answers {
		for _, ip := range a.addrs {
			if !seen[ip] {
				seen[ip] = true
				addrs = append(addrs, ip)
			}
		}
		switch {
		case a.err != nil:
			silent++
			bad = append(bad, a.resolver)
		case len(a.addrs) > 0:
		case a.rcode == dns.RcodeNameError:
			nxdomain++
			bad = append(bad, a.resolver)
		case a.rcode == dns.RcodeSuccess:
			empty++
		default:
			bad = append(bad, a.resolver)
		}
	}

	switch {
	case len(addrs) > 0 && len(bad) > 0:
		return Warn, fmt.Sprintf("%s resolved, but %s gave no usable answer", host, strings.Join(bad, ", ")), addrs
	case len(addrs) > 0:
		return Pass, host + " resolved to " + joinAddrs(addrs), addrs
	case nxdomain == len(answers):
		return Fail, host + " does not exist (NXDOMAIN from every resolver)", nil
	case silent == len(answers):
		return Fail, "no DNS resolver answered (timeout or unreachable)", nil
	case empty > 0 && len(bad) == 0:
		return Fail, host + " has no A or AAAA records", nil
	default:
		return Fail, "DNS resolvers returned errors (SERVFAIL or REFUSED) and no address", nil
	}
}

func describe(answers []answer) string {
	lines := make([]string, len(answers))
	for i, a := range answers {
		if a.err != nil {
			lines[i] = fmt.Sprintf("%s: no reply (%v)", a.resolver, a.err)
			continue
		}
		lines[i] = fmt.Sprintf("%s: %s, %d address(es), %s", a.resolver, dns.RcodeToString[a.rcode], len(a.addrs), a.rtt.Round(time.Millisecond))
	}
	return strings.Join(lines, "\n")
}

func lookupSystem(ctx context.Context, host string) []netip.Addr {
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil
	}
	out := make([]netip.Addr, len(ips))
	for i, ip := range ips {
		out[i] = ip.Unmap()
	}
	return out
}

func joinAddrs(addrs []netip.Addr) string {
	s := make([]string, len(addrs))
	for i, a := range addrs {
		s[i] = a.String()
	}
	return strings.Join(s, ", ")
}
