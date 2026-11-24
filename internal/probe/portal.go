package probe

import (
	"context"
	"net/http"
	"time"

	"github.com/rahulkaushal04/netwhy/internal/evidence"
)

// PortalURL is a plain-HTTP endpoint that answers 204 No Content when
// nothing sits between you and the internet.
const PortalURL = "http://connectivitycheck.gstatic.com/generate_204"

var _ Probe = Portal{}

// Portal detects a captive portal or filtering proxy by fetching a URL that
// must answer 204. This is the one request netwhy makes to a host the user did
// not name, so it can be turned off. A failed request is inconclusive and only
// warns; any other answer fails, because the portal blocks the real target.
type Portal struct {
	// URL defaults to PortalURL.
	URL string
}

func (Portal) Name() string { return "portal" }

func (Portal) Deps() []string { return []string{"dns"} }

func (p Portal) Run(ctx context.Context) Result {
	res := Result{Name: "portal"}
	url := p.URL
	if url == "" {
		url = PortalURL
	}
	ev := evidence.Evidence{Sent: "GET " + url}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		ev.Error = err.Error()
		res.Status, res.Summary, res.Evidence = Warn, "invalid captive-portal check URL", ev
		return res
	}
	client := &http.Client{
		Transport:     &http.Transport{Proxy: nil, DisableKeepAlives: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	start := time.Now()
	resp, err := client.Do(req)
	ev.Duration = time.Since(start)
	res.Evidence = ev

	if err != nil {
		res.Evidence.Error = err.Error()
		res.Status, res.Summary = Warn, "could not check for a captive portal"
		return res
	}
	resp.Body.Close()

	res.Evidence.Received = resp.Status
	if loc := resp.Header.Get("Location"); loc != "" {
		res.Evidence.Received += ", redirect to " + loc
	}
	if resp.StatusCode == http.StatusNoContent {
		res.Status, res.Summary = Pass, "no captive portal"
		return res
	}
	res.Status = Fail
	res.Summary = "a captive portal or filtering proxy is answering plain HTTP (" + resp.Status + " instead of 204): sign in through a browser"
	return res
}
