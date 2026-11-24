package probe

import (
	"testing"

	"github.com/vishvananda/netlink"
)

func TestNUDState(t *testing.T) {
	tests := []struct {
		in   int
		want string
	}{
		{netlink.NUD_REACHABLE, "reachable"},
		{netlink.NUD_STALE, "stale"},
		{netlink.NUD_DELAY, "delay"},
		{netlink.NUD_PROBE, "probe"},
		{netlink.NUD_FAILED, "failed"},
		{netlink.NUD_INCOMPLETE, "incomplete"},
		{netlink.NUD_PERMANENT, "permanent"},
		{netlink.NUD_REACHABLE | netlink.NUD_PERMANENT, "reachable"},
		{netlink.NUD_NONE, "none"},
	}
	for _, tc := range tests {
		if got := nudState(tc.in); got != tc.want {
			t.Errorf("nudState(%#x) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
