package probe

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/http/httpproxy"

	"github.com/rahulkaushal04/netwhy/internal/evidence"
)

// bodySample is how much of the response body to read.
const bodySample = 64 << 10

var _ Probe = HTTP{}

// HTTP makes one GET and reads the first 64 KiB of the body. Redirects are
// reported, not followed. Proxy environment variables are not used, so the
// result reflects the direct path; a configured proxy is reported as a warning.
type HTTP struct {
	URL string
	// Dep is the probe that must work first: tls for https, else tcp.
	Dep string
}

func (HTTP) Name() string { return "http" }

func (p HTTP) Deps() []string { return []string{p.Dep} }

func (p HTTP) Run(ctx context.Context) Result {
	res := Result{Name: "http"}
	ev := evidence.Evidence{Sent: "GET " + p.URL}

	var tm timings
	req, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, tm.trace()), http.MethodGet, p.URL, nil)
	if err != nil {
		ev.Error = err.Error()
		res.Status, res.Summary, res.Evidence = Fail, "invalid request URL", ev
		return res
	}
	req.Header.Set("User-Agent", "netwhy")

	client := &http.Client{
		Transport:     &http.Transport{Proxy: nil, DisableKeepAlives: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	start := time.Now()
	resp, err := client.Do(req)
	ev.Duration = time.Since(start)

	if err != nil {
		ev.Error = err.Error()
		res.Status, res.Summary, res.Evidence = Fail, "HTTP request failed even though the connection works", ev
		return res
	}
	defer resp.Body.Close()

	// Headers are small. Reading some body is what exposes a PMTU black hole:
	// large packets are dropped, so the transfer stalls after the first ones.
	n, readErr := io.CopyN(io.Discard, resp.Body, bodySample)
	ev.Duration = time.Since(start)
	ev.Received = fmt.Sprintf("%s, %d body bytes\n%s", resp.Status, n, tm.breakdown(start, ev.Duration))
	res.Evidence = ev

	res.Status = Pass
	res.Summary = "server answered " + resp.Status
	switch {
	case readErr != nil && !errors.Is(readErr, io.EOF):
		res.Status = Warn
		res.Summary = fmt.Sprintf("the response stopped after %d bytes (%v): large packets may be dropped, see the pmtu result", n, readErr)
	case resp.StatusCode >= 500:
		res.Status = Warn
		res.Summary = "server answered with an error: " + resp.Status
	}
	if proxy := proxyFor(p.URL); proxy != "" && res.Status == Pass {
		res.Status = Warn
		res.Summary += "; a proxy (" + proxy + ") is set in the environment and applications will use it, but netwhy tested the direct path"
	}
	return res
}

// proxyFor returns the proxy host the environment configures for rawURL, or "".
func proxyFor(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	p, err := httpproxy.FromEnvironment().ProxyFunc()(u)
	if err != nil || p == nil {
		return ""
	}
	return p.Host
}

// timings is locked because httptrace hooks can run on different goroutines.
type timings struct {
	mu                           sync.Mutex
	dnsStart, dnsDone            time.Time
	connStart, connDone          time.Time
	tlsStart, tlsDone, firstByte time.Time
}

func (t *timings) trace() *httptrace.ClientTrace {
	set := func(dst *time.Time) {
		t.mu.Lock()
		*dst = time.Now()
		t.mu.Unlock()
	}
	return &httptrace.ClientTrace{
		DNSStart:             func(httptrace.DNSStartInfo) { set(&t.dnsStart) },
		DNSDone:              func(httptrace.DNSDoneInfo) { set(&t.dnsDone) },
		ConnectStart:         func(string, string) { set(&t.connStart) },
		ConnectDone:          func(string, string, error) { set(&t.connDone) },
		TLSHandshakeStart:    func() { set(&t.tlsStart) },
		TLSHandshakeDone:     func(tls.ConnectionState, error) { set(&t.tlsDone) },
		GotFirstResponseByte: func() { set(&t.firstByte) },
	}
}

func (t *timings) breakdown(start time.Time, total time.Duration) string {
	t.mu.Lock()
	defer t.mu.Unlock()

	var parts []string
	add := func(name string, from, to time.Time) {
		if !from.IsZero() && !to.IsZero() {
			parts = append(parts, fmt.Sprintf("%s %s", name, to.Sub(from).Round(time.Millisecond)))
		}
	}
	add("dns", t.dnsStart, t.dnsDone)
	add("connect", t.connStart, t.connDone)
	add("tls", t.tlsStart, t.tlsDone)

	ready := start
	for _, ts := range []time.Time{t.dnsDone, t.connDone, t.tlsDone} {
		if ts.After(ready) {
			ready = ts
		}
	}
	add("server wait", ready, t.firstByte)
	parts = append(parts, "total "+total.Round(time.Millisecond).String())
	return strings.Join(parts, ", ")
}
