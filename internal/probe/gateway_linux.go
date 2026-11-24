package probe

import (
	"context"
	"net"
	"net/netip"

	"github.com/vishvananda/netlink"
)

// defaultGateway is a kernel route lookup, not traffic.
func defaultGateway(context.Context) (netip.Addr, string, error) {
	routes, err := netlink.RouteGet(net.IPv4(8, 8, 8, 8))
	if err != nil {
		return netip.Addr{}, "", err
	}
	if len(routes) == 0 || routes[0].Gw == nil {
		return netip.Addr{}, "", errNoGateway
	}
	link, err := netlink.LinkByIndex(routes[0].LinkIndex)
	if err != nil {
		return netip.Addr{}, "", err
	}
	gw, ok := netip.AddrFromSlice(routes[0].Gw)
	if !ok {
		return netip.Addr{}, "", errNoGateway
	}
	return gw.Unmap(), link.Attrs().Name, nil
}

func neighbour(_ context.Context, gw netip.Addr, iface string) (state, mac string, err error) {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return "", "", err
	}
	neighs, err := netlink.NeighList(link.Attrs().Index, netlink.FAMILY_V4)
	if err != nil {
		return "", "", err
	}
	for _, n := range neighs {
		if n.IP.Equal(net.IP(gw.AsSlice())) {
			return nudState(n.State), n.HardwareAddr.String(), nil
		}
	}
	return "none", "", nil
}

func nudState(s int) string {
	switch {
	case s&netlink.NUD_FAILED != 0:
		return "failed"
	case s&netlink.NUD_INCOMPLETE != 0:
		return "incomplete"
	case s&netlink.NUD_REACHABLE != 0:
		return "reachable"
	case s&netlink.NUD_PERMANENT != 0:
		return "permanent"
	case s&netlink.NUD_NOARP != 0:
		return "noarp"
	case s&netlink.NUD_DELAY != 0:
		return "delay"
	case s&netlink.NUD_PROBE != 0:
		return "probe"
	case s&netlink.NUD_STALE != 0:
		return "stale"
	}
	return "none"
}
