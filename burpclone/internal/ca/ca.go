// Package ca handles the root Certificate Authority: generating it once,
// persisting it to disk, and reloading it on subsequent runs.
//
// This is the PKI concept lo udah pernah praktekin manual pakai OpenSSL
// (3-tier Root CA -> Personal CA -> end-entity di lab Polibatam) — bedanya
// sekarang semuanya dilakuin programmatic pakai crypto/x509, dan cuma butuh
// 2 tingkat: Root CA (dibuat sekali, di-trust manual di browser/OS) ->
// leaf cert per-domain (dibikin on-the-fly tiap ada request HTTPS baru,
// lihat leaf.go).
package ca

import (
	"crypto/ecdsa"
	"crypto/x509"
	"log"
	"os"
)

// Authority holds the root CA's certificate and private key, kept in memory
// for the lifetime of the process so leaf.go can sign new leaf certs
// without touching disk on every request.
type Authority struct {
	Cert *x509.Certificate
	Key  *ecdsa.PrivateKey // EC key, same family lo pakai di PKI lab (prime256v1)
}

// LoadOrCreateRootCA loads root.pem + root.key from dir if they exist,
// otherwise generates a new self-signed root CA and writes both files.
//
// TODO(phase 2):
//  1. Check if dir/root.pem and dir/root.key exist.
//  2. If yes: parse them with x509.ParseCertificate + x509.ParseECPrivateKey,
//     return an *Authority.
//  3. If no:
//     a. generate an ECDSA key with ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
//     b. build an x509.Certificate template with:
//        - IsCA: true
//        - KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature
//        - BasicConstraintsValid: true
//        - a reasonable NotBefore/NotAfter (e.g. 10 years)
//     c. self-sign with x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
//     d. PEM-encode cert + key, write to dir/root.pem and dir/root.key
//     e. print a reminder to import root.pem into the browser/OS trust store
//  4. Return the *Authority.
func LoadOrCreateRootCA(dir string) (*Authority, error) {
	// Phase 1 placeholder: since plain-HTTP-only mode doesn't need a CA at
	// all (no TLS termination happens yet), we just make sure the
	// directory exists and return an empty Authority so main.go can start
	// end-to-end. Real key/cert generation lands in phase 2 - see the
	// TODO comment above for the exact steps.
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	log.Println("ca: phase 2 not implemented yet - root CA generation skipped, HTTPS/CONNECT requests will be rejected")
	return &Authority{}, nil
}
