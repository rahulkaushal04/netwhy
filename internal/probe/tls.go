package probe

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rahulkaushal04/netwhy/internal/evidence"
)

const expiryWarning = 14 * 24 * time.Hour

var _ Probe = TLS{}

// TLS handshakes with the address TCP connected to. The chain is verified inside
// VerifyConnection so the certificate stays available as evidence when it fails.
type TLS struct {
	Host string
	Path *Path
	// Roots overrides the trusted CAs; nil means the system roots.
	Roots *x509.CertPool
}

func (TLS) Name() string { return "tls" }

func (TLS) Deps() []string { return []string{"tcp"} }

func (p TLS) Run(ctx context.Context) Result {
	res := Result{Name: "tls"}
	start := time.Now()

	var seen tls.ConnectionState
	var verified bool
	var verifyErr error
	cfg := &tls.Config{
		ServerName: p.Host,
		// The default verification is replaced by VerifyConnection below,
		// which does the same checks but keeps the state for evidence.
		InsecureSkipVerify: true,
		VerifyConnection: func(cs tls.ConnectionState) error {
			seen, verified = cs, true
			verifyErr = verifyChain(cs, p.Host, p.Roots)
			return verifyErr
		},
	}
	conn, err := (&tls.Dialer{Config: cfg}).DialContext(ctx, "tcp", p.Path.Conn.String())

	ev := evidence.Evidence{
		Sent:     fmt.Sprintf("TLS handshake with %s, server name %q", p.Path.Conn, p.Host),
		Duration: time.Since(start),
	}
	if verified {
		ev.Received = describeCert(seen)
	}

	if err != nil {
		ev.Error = err.Error()
		res.Status, res.Evidence = Fail, ev
		if verified && verifyErr != nil {
			res.Summary = explainVerify(verifyErr, seen.PeerCertificates[0], p.Host)
		} else {
			res.Summary = "TLS handshake failed"
		}
		return res
	}
	conn.Close()

	leaf := seen.PeerCertificates[0]
	left := time.Until(leaf.NotAfter)
	days := int(left.Hours() / 24)
	res.Evidence = ev
	res.Status = Pass
	res.Summary = fmt.Sprintf("%s, certificate valid for %d more days (issuer: %s)", tls.VersionName(seen.Version), days, leaf.Issuer.CommonName)
	if left < expiryWarning {
		res.Status = Warn
		res.Summary = fmt.Sprintf("certificate expires in %d days (%s)", days, leaf.NotAfter.Format("2006-01-02"))
	}
	return res
}

func verifyChain(cs tls.ConnectionState, host string, roots *x509.CertPool) error {
	if len(cs.PeerCertificates) == 0 {
		return errors.New("server sent no certificate")
	}
	inter := x509.NewCertPool()
	for _, c := range cs.PeerCertificates[1:] {
		inter.AddCert(c)
	}
	_, err := cs.PeerCertificates[0].Verify(x509.VerifyOptions{DNSName: host, Roots: roots, Intermediates: inter})
	return err
}

func explainVerify(err error, leaf *x509.Certificate, host string) string {
	var hostErr x509.HostnameError
	var invalid x509.CertificateInvalidError
	var unknown x509.UnknownAuthorityError
	switch {
	case errors.As(err, &hostErr):
		return fmt.Sprintf("certificate is not valid for %s (it covers: %s)", host, strings.Join(certNames(leaf), ", "))
	case errors.As(err, &invalid) && invalid.Reason == x509.Expired:
		if time.Now().Before(leaf.NotBefore) {
			return "certificate is not valid until " + leaf.NotBefore.Format("2006-01-02")
		}
		return "certificate expired on " + leaf.NotAfter.Format("2006-01-02")
	case errors.As(err, &unknown):
		return fmt.Sprintf("certificate chain is not trusted: unknown issuer %q", leaf.Issuer.CommonName)
	default:
		return "certificate verification failed"
	}
}

func describeCert(cs tls.ConnectionState) string {
	if len(cs.PeerCertificates) == 0 {
		return tls.VersionName(cs.Version) + ", no certificate"
	}
	leaf := cs.PeerCertificates[0]
	return fmt.Sprintf("%s, %s\nsubject: %s\nissuer: %s\nvalid: %s to %s\nnames: %s",
		tls.VersionName(cs.Version), tls.CipherSuiteName(cs.CipherSuite),
		leaf.Subject.CommonName, leaf.Issuer.CommonName,
		leaf.NotBefore.Format("2006-01-02"), leaf.NotAfter.Format("2006-01-02"),
		strings.Join(certNames(leaf), ", "))
}

func certNames(c *x509.Certificate) []string {
	names := append([]string{}, c.DNSNames...)
	for _, ip := range c.IPAddresses {
		names = append(names, ip.String())
	}
	return names
}
