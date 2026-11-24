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
	Raw string
	// Scheme is http, https, or tcp for a bare host:port.
	Scheme string
	Host   string
	Port   int
	Path   string
	IsIP   bool
}

// ParseTarget accepts a URL, host:port, a hostname or an IP. A bare host is https on 443.
func ParseTarget(raw string) (Target, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Target{}, errors.New("empty target")
	}

	t := Target{Raw: raw}

	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil {
			return Target{}, fmt.Errorf("invalid URL: %w", err)
		}

		t.Scheme, t.Host, t.Path = u.Scheme, u.Hostname(), u.RequestURI()

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
		t.Scheme, t.Host, t.Port, t.Path = "https", strings.Trim(raw, "[]"), 443, "/"
	}

	if t.Host == "" {
		return Target{}, errors.New("target has no host")
	}

	_, err := netip.ParseAddr(t.Host)
	t.IsIP = err == nil

	return t, nil
}

func (t Target) URL() string {
	host := t.Host
	switch {
	case t.Scheme == "http" && t.Port != 80, t.Scheme == "https" && t.Port != 443:
		host = net.JoinHostPort(t.Host, strconv.Itoa(t.Port))
	case strings.Contains(t.Host, ":"):
		host = "[" + t.Host + "]"
	}
	return t.Scheme + "://" + host + t.Path
}

func parsePort(s string) (int, error) {
	p, err := strconv.Atoi(s)
	if err != nil || p < 1 || p > 65535 {
		return 0, fmt.Errorf("invalid port %q", s)
	}
	return p, nil
}
