// Command certgen writes a self-signed ECDSA certificate for the namespace
// tests. A negative -days makes an already expired one, which openssl cannot.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"log"
	"math/big"
	"os"
	"time"
)

func main() {
	name := flag.String("name", "", "DNS name the certificate is valid for")
	days := flag.Int("days", 30, "days until expiry, negative for an expired certificate")
	certFile := flag.String("cert", "cert.pem", "certificate output file")
	keyFile := flag.String("key", "key.pem", "private key output file")
	flag.Parse()
	if *name == "" {
		log.Fatal("-name is required")
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}
	notAfter := time.Now().AddDate(0, 0, *days)
	notBefore := time.Now().Add(-48 * time.Hour)
	if notBefore.After(notAfter) {
		notBefore = notAfter.Add(-48 * time.Hour)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: *name},
		DNSNames:     []string{*name},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		log.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		log.Fatal(err)
	}
	write(*certFile, "CERTIFICATE", der)
	write(*keyFile, "EC PRIVATE KEY", keyDER)
}

func write(file, kind string, der []byte) {
	if err := os.WriteFile(file, pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: der}), 0o600); err != nil {
		log.Fatal(err)
	}
}
