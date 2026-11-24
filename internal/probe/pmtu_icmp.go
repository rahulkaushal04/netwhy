package probe

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"syscall"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// pingWait is how long to wait for one echo reply.
const pingWait = 300 * time.Millisecond

type icmpEcho struct {
	conn *icmp.PacketConn
	dst  *net.UDPAddr
	seq  int
}

// openEchoer opens an unprivileged ICMP socket (no root needed where the OS
// allows it) with the don't-fragment bit set.
func openEchoer(dst netip.Addr) (echoer, error) {
	c, err := icmp.ListenPacket("udp4", "0.0.0.0")
	if err != nil {
		return nil, err
	}
	sc, ok := c.IPv4PacketConn().PacketConn.(syscall.Conn)
	if !ok {
		c.Close()
		return nil, errors.New("cannot reach the ICMP socket")
	}
	rc, err := sc.SyscallConn()
	if err != nil {
		c.Close()
		return nil, err
	}
	var setErr error
	if err := rc.Control(func(fd uintptr) { setErr = setDontFragment(int(fd)) }); err != nil {
		setErr = err
	}
	if setErr != nil {
		c.Close()
		return nil, setErr
	}
	return &icmpEcho{conn: c, dst: &net.UDPAddr{IP: dst.AsSlice()}}, nil
}

func (e *icmpEcho) Close() error { return e.conn.Close() }

func (e *icmpEcho) ping(ctx context.Context, payload int) bool {
	e.seq++
	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Body: &icmp.Echo{ID: os.Getpid() & 0xffff, Seq: e.seq, Data: make([]byte, payload)},
	}
	b, err := msg.Marshal(nil)
	if err != nil {
		return false
	}
	deadline := time.Now().Add(pingWait)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	e.conn.SetDeadline(deadline)

	// EMSGSIZE here means the packet exceeds the local link MTU with DF set.
	if _, err := e.conn.WriteTo(b, e.dst); err != nil {
		return false
	}
	buf := make([]byte, 2048)
	for {
		n, _, err := e.conn.ReadFrom(buf)
		if err != nil {
			return false
		}
		m, err := icmp.ParseMessage(1, buf[:n])
		if err != nil {
			continue
		}
		if echo, ok := m.Body.(*icmp.Echo); ok && m.Type == ipv4.ICMPTypeEchoReply && echo.Seq == e.seq {
			return true
		}
	}
}
