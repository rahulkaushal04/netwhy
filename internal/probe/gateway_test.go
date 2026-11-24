package probe

import (
	"net/netip"
	"testing"
)

func TestGatewayResult(t *testing.T) {
	gw := netip.MustParseAddr("192.168.1.1")
	tests := []struct {
		state string
		want  Status
	}{
		{"reachable", Pass},
		{"stale", Pass},
		{"delay", Pass},
		{"probe", Pass},
		{"permanent", Pass},
		{"present", Pass},
		{"failed", Fail},
		{"incomplete", Fail},
		{"none", Fail},
		{"", Fail},
	}
	for _, tc := range tests {
		if got, _ := gatewayResult(gw, "eth0", tc.state, "aa:bb:cc:dd:ee:ff"); got != tc.want {
			t.Errorf("state %q: status = %s, want %s", tc.state, got, tc.want)
		}
	}
}
