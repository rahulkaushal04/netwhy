package probe

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"
)

// testCert makes a self-signed certificate for localhost and 127.0.0.1.
func testCert(t *testing.T, notBefore, notAfter time.Time) (tls.Certificate, *x509.Certificate) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "netwhy test"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, leaf
}

// tlsServer serves cert on a loopback port and returns the address.
func tlsServer(t *testing.T, cert tls.Certificate) netip.AddrPort {
	t.Helper()
	l, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				c.(*tls.Conn).Handshake()
				c.Close()
			}()
		}
	}()
	return netip.MustParseAddrPort(l.Addr().String())
}

func TestTLS(t *testing.T) {
	now := time.Now()
	day := 24 * time.Hour

	tests := []struct {
		name     string
		notAfter time.Time
		host     string
		trusted  bool
		want     Status
		contains string
	}{
		{"valid", now.Add(90 * day), "localhost", true, Pass, "valid for"},
		{"expiring soon", now.Add(5 * day), "localhost", true, Warn, "expires in"},
		{"expired", now.Add(-day), "localhost", true, Fail, "expired on"},
		{"wrong host", now.Add(90 * day), "other.test", true, Fail, "not valid for other.test"},
		{"untrusted issuer", now.Add(90 * day), "localhost", false, Fail, "not trusted"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cert, leaf := testCert(t, now.Add(-2*day), tc.notAfter)
			p := TLS{Host: tc.host, Path: &Path{Conn: tlsServer(t, cert)}}
			if tc.trusted {
				p.Roots = x509.NewCertPool()
				p.Roots.AddCert(leaf)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			got := p.Run(ctx)

			if got.Status != tc.want || !strings.Contains(got.Summary, tc.contains) {
				t.Errorf("got %s %q, want %s containing %q", got.Status, got.Summary, tc.want, tc.contains)
			}
		})
	}
}
