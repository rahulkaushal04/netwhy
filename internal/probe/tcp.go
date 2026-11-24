package probe

import (
	"context"
	"errors"
	"net"
	"strconv"
	"syscall"
	"time"

	"github.com/rahulkaushal04/netwhy/internal/evidence"
)

var _ Probe = TCP{}

type TCP struct {
	Host string
	Port int
}

func (TCP) Name() string   { return "tcp" }
func (TCP) Deps() []string { return []string{"dns"} }

func (p TCP) Run(ctx context.Context) Result {
	res := Result{Name: "tcp"}
	addr := net.JoinHostPort(p.Host, strconv.Itoa(p.Port))

	start := time.Now()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	ev := evidence.Evidence{Sent: "TCP connect to " + addr, Duration: time.Since(start)}

	if err != nil {
		ev.Error = err.Error()
		res.Status = Fail
		res.Evidence = ev

		var ne net.Error
		switch {
		case errors.Is(err, syscall.ECONNREFUSED):
			res.Summary = "connection refused (RST): host is reachable but nothing accepts connections on port " + strconv.Itoa(p.Port)
		case errors.Is(err, syscall.ENETUNREACH), errors.Is(err, syscall.EHOSTUNREACH):
			res.Summary = "no route to host: the network or host is unreachable"
		case errors.As(err, &ne) && ne.Timeout():
			res.Summary = "connection timed out: packets are being silently dropped (firewall DROP or routing problem)"
		default:
			res.Summary = "TCP connection failed"
		}
		return res
	}
	defer conn.Close()

	ev.Received = "connected " + conn.LocalAddr().String() + " -> " + conn.RemoteAddr().String()
	res.Status = Pass
	res.Summary = "TCP connection to " + addr + " succeeded"
	res.Evidence = ev
	return res
}
