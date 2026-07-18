package ca

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"sync"
	"time"
)

// leafCache caches generated leaf certs per hostname so we don't re-sign a
// new cert on every single connection to the same host.
//
// Same pattern as RobotsChecker.cache in webcrawler: map + sync.Mutex
// protecting concurrent goroutines.
type leafCache struct {
	mu    sync.Mutex
	certs map[string]*tls.Certificate
}

func newLeafCache() *leafCache {
	return &leafCache{certs: make(map[string]*tls.Certificate)}
}

// GetOrCreateLeaf returns a cached leaf cert for host, or generates,
// caches, and returns a new one, signed by the root CA.
func (a *Authority) GetOrCreateLeaf(host string) (*tls.Certificate, error) {
	a.leafCache.mu.Lock()
	defer a.leafCache.mu.Unlock()

	if cert, ok := a.leafCache.certs[host]; ok {
		return cert, nil
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: host},
		DNSNames:     []string{host}, // SAN - modern browsers require this, not just CN
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(1, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:         false,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, a.Cert, &key.PublicKey, a.Key)
	if err != nil {
		return nil, fmt.Errorf("ca: failed to sign leaf cert for %s: %w", host, err)
	}

	cert := &tls.Certificate{
		Certificate: [][]byte{derBytes, a.Cert.Raw}, // leaf + root, so client can build the chain
		PrivateKey:  key,
	}
	a.leafCache.certs[host] = cert
	return cert, nil
}