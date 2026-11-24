package probe

import "golang.org/x/sys/unix"

func setDontFragment(fd int) error {
	return unix.SetsockoptInt(fd, unix.IPPROTO_IP, unix.IP_DONTFRAG, 1)
}
