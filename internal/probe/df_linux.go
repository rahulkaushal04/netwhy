package probe

import "golang.org/x/sys/unix"

// setDontFragment sets DF and ignores the kernel's cached path MTU, so every
// packet is sent at the size asked for.
func setDontFragment(fd int) error {
	return unix.SetsockoptInt(fd, unix.IPPROTO_IP, unix.IP_MTU_DISCOVER, unix.IP_PMTUDISC_PROBE)
}
