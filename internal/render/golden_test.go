package render

import (
	"bytes"
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rahulkaushal04/netwhy/internal/evidence"
	"github.com/rahulkaushal04/netwhy/internal/probe"
	"github.com/rahulkaushal04/netwhy/internal/verdict"
)

var update = flag.Bool("update", false, "rewrite the golden files")

func result(name string, s probe.Status, summary string, ev evidence.Evidence) probe.Result {
	return probe.Result{Name: name, Status: s, Summary: summary, Evidence: ev}
}

// scenarios are fixed results, so every renderer's output is reproducible.
var scenarios = map[string][]probe.Result{
	"pass": {
		result("system", probe.Pass, "interface up", evidence.Evidence{Sent: "route lookup", Duration: time.Millisecond}),
		result("dns", probe.Pass, "example.com resolved to 192.0.2.1", evidence.Evidence{Sent: "A and AAAA for example.com", Received: "192.0.2.53:53: NOERROR, 1 address(es), 12ms", Duration: 12 * time.Millisecond}),
		result("tcp", probe.Pass, "connected over IPv4", evidence.Evidence{Duration: 20 * time.Millisecond}),
	},
	"dns-fail": {
		result("system", probe.Pass, "interface up", evidence.Evidence{}),
		result("dns", probe.Fail, "no DNS resolver answered (timeout or unreachable)", evidence.Evidence{
			Sent: "A and AAAA for example.com to 192.0.2.53:53, 192.0.2.54:53", Received: "192.0.2.53:53: no reply (i/o timeout)\n192.0.2.54:53: no reply (i/o timeout)",
			Duration: 5 * time.Second, Error: "read udp: i/o timeout",
		}),
		result("tcp", probe.Skipped, "skipped because dns did not work", evidence.Evidence{}),
	},
	"tcp-warn": {
		result("system", probe.Pass, "interface up", evidence.Evidence{}),
		result("dns", probe.Pass, "example.com resolved to 192.0.2.1, 2001:db8::1", evidence.Evidence{}),
		result("tcp", probe.Warn, "IPv6 failed (connection timed out) but IPv4 works; clients that try IPv6 first will be slow", evidence.Evidence{
			Sent: "TCP connect to port 443", Received: "192.0.2.1:443: connected in 20ms\n[2001:db8::1]:443: failed: i/o timeout", Duration: 5 * time.Second,
		}),
	},
}

var formats = []struct {
	ext string
	fn  func(io.Writer, string, verdict.Verdict) error
}{
	{"txt", func(w io.Writer, t string, v verdict.Verdict) error { Human(w, t, v); return nil }},
	{"md", func(w io.Writer, t string, v verdict.Verdict) error { Markdown(w, t, v); return nil }},
	{"json", JSON},
}

func TestGolden(t *testing.T) {
	for name, results := range scenarios {
		v := verdict.Decide(results)
		for _, f := range formats {
			t.Run(name+"."+f.ext, func(t *testing.T) {
				var got bytes.Buffer
				if err := f.fn(&got, "example.com", v); err != nil {
					t.Fatal(err)
				}

				file := filepath.Join("testdata", name+"."+f.ext+".golden")
				if *update {
					if err := os.MkdirAll("testdata", 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(file, got.Bytes(), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				want, err := os.ReadFile(file)
				if err != nil {
					t.Fatalf("%v (run go test -update to create it)", err)
				}
				if !bytes.Equal(got.Bytes(), want) {
					t.Errorf("output differs from %s (run go test -update if the change is intended):\n%s", file, got.String())
				}
			})
		}
	}
}
