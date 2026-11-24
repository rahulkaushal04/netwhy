package plan

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

type Target struct {
	Raw    string
	Scheme string
	Host   string
	Port   int
	IsIP   bool
}

func ParseTarget(raw string) (Target, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Target{}, errors.New("empty target")
	}

	t := Target{Raw: raw}

	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil {
			return Target{}, fmt.Errorf("Invalid URL: %w", err)
		}

		t.Scheme, t.Host = u.Scheme, u.Hostname()

		switch u.Scheme {
		case "http":
			t.Port = 80
		case "https":
			t.Port = 443
		default:
			return Target{}, fmt.Errorf("unsupported scheme %q", u.Scheme)
		}

		if p := u.Port(); p != "" {
			if t.Port, err = parsePort(p); err != nil {
				return Target{}, err
			}
		}
	} else if host, p, err := net.SplitHostPort(raw); err == nil {
		t.Scheme, t.Host = "tcp", host
		if t.Port, err = parsePort(p); err != nil {
			return Target{}, err
		}
	} else {
		t.Scheme, t.Host, t.Port = "https", strings.Trim(raw, "[]"), 443
	}

	if t.Host == "" {
		return Target{}, errors.New("target has no host")
	}

	_, err := netip.ParseAddr(t.Host)
	t.IsIP = err == nil

	return t, nil
}

func parsePort(s string) (int, error) {
	p, err := strconv.Atoi(s)
	if err != nil || p < 1 || p > 65535 {
		return 0, fmt.Errorf("invalid port %q", s)
	}
	return p, nil
}
