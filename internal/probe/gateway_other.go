//go:build !linux

package probe

import (
	"context"
	"net/netip"
)

// macOS and the BSDs show an empty ARP table to unsigned programs, even from
// the arp command, so a check would report false failures. Linux only.
func defaultGateway(context.Context) (netip.Addr, string, error) {
	return netip.Addr{}, "", errUnsupported
}

func neighbour(context.Context, netip.Addr, string) (string, string, error) {
	return "", "", errUnsupported
}
