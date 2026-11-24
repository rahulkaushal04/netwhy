package probe

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/rahulkaushal04/netwhy/internal/evidence"
)

var _ Probe = System{}

type System struct{}

func (System) Name() string   { return "system" }
func (System) Deps() []string { return nil }

func (System) Run(ctx context.Context) Result {
	start := time.Now()
	res := Result{Name: "system"}

	ifaces, err := net.Interfaces()
	if err != nil {
		res.Status = Fail
		res.Summary = "cannot list network interfaces"
		res.Evidence = evidence.Evidence{Sent: "list interfaces", Duration: time.Since(start), Error: err.Error()}
		return res
	}

	var usable []string
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok && ipn.IP.IsGlobalUnicast() {
				usable = append(usable, ifc.Name+" "+ipn.IP.String())
			}
		}
	}

	if len(usable) == 0 {
		res.Status = Fail
		res.Summary = "no active interface has a usable IP address (link down, or no DHCP lease)"
		res.Evidence = evidence.Evidence{Sent: "list interfaces", Received: "none usable", Duration: time.Since(start)}
		return res
	}

	var d net.Dialer
	conn, err := d.DialContext(ctx, "udp", "8.8.8.8:53")
	if err != nil {
		res.Status = Fail
		res.Summary = "interface has an address but the kernel has no route to the internet"
		res.Evidence = evidence.Evidence{Sent: "route lookup to 8.8.8.8", Received: fmt.Sprint(usable), Duration: time.Since(start), Error: err.Error()}
		return res
	}
	defer conn.Close()

	res.Status = Pass
	res.Summary = "interface up, route out via " + conn.LocalAddr().String()
	res.Evidence = evidence.Evidence{Sent: "route lookup to 8.8.8.8", Received: fmt.Sprint(usable), Duration: time.Since(start)}
	return res

}
