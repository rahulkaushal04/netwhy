package probe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func runHTTP(url string) Result {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return HTTP{URL: url, Dep: "tcp"}.Run(ctx)
}

func TestHTTPStatuses(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    Status
		in      string
	}{
		{"ok", func(w http.ResponseWriter, _ *http.Request) {}, Pass, "200"},
		{"not found is still reachable", func(w http.ResponseWriter, _ *http.Request) { http.NotFound(w, nil) }, Pass, "404"},
		{"redirect is not followed", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/elsewhere", http.StatusFound) }, Pass, "302"},
		{"server error warns", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }, Warn, "503"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()

			got := runHTTP(srv.URL)

			if got.Status != tc.want || !strings.Contains(got.Evidence.Received, tc.in) {
				t.Errorf("got %s, received %q; want %s containing %s", got.Status, got.Evidence.Received, tc.want, tc.in)
			}
		})
	}
}

func TestHTTPBreakdownIsReported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()

	got := runHTTP(srv.URL)

	for _, want := range []string{"connect ", "server wait ", "total "} {
		if !strings.Contains(got.Evidence.Received, want) {
			t.Errorf("breakdown is missing %q: %s", want, got.Evidence.Received)
		}
	}
}

func TestHTTPConnectionRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	if got := runHTTP(url); got.Status != Fail {
		t.Errorf("status = %s, want fail", got.Status)
	}
}

func TestHTTPStalledBodyWarns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000000")
		w.Write(make([]byte, 100))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	got := HTTP{URL: srv.URL, Dep: "tcp"}.Run(ctx)

	if got.Status != Warn || !strings.Contains(got.Summary, "stopped after 100 bytes") {
		t.Errorf("got %s %q, want warn: stopped after 100 bytes", got.Status, got.Summary)
	}
}

func TestProxyFor(t *testing.T) {
	for _, k := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "NO_PROXY", "no_proxy"} {
		t.Setenv(k, "")
	}
	if got := proxyFor("https://example.com/"); got != "" {
		t.Errorf("no proxy configured, got %q", got)
	}

	t.Setenv("HTTPS_PROXY", "http://user:secret@proxy.example:3128")
	if got := proxyFor("https://example.com/"); got != "proxy.example:3128" {
		t.Errorf("got %q, want the proxy host without credentials", got)
	}

	t.Setenv("NO_PROXY", "example.com")
	if got := proxyFor("https://example.com/"); got != "" {
		t.Errorf("NO_PROXY should bypass the proxy, got %q", got)
	}
}
