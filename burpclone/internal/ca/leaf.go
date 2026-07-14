package ca

import (
	"crypto/tls"
	"sync"
)

// leafCache caches generated leaf certs per hostname so we don't re-sign a
// new cert on every single connection to the same host.
//
// This is the EXACT same pattern as RobotsChecker.cache in the webcrawler
// project (map + sync.Mutex protecting concurrent goroutines) — reuse that
// idea directly here.
type leafCache struct {
	mu    sync.Mutex
	certs map[string]*tls.Certificate
}

func newLeafCache() *leafCache {
	return &leafCache{certs: make(map[string]*tls.Certificate)}
}

// GetOrCreateLeaf returns a cached leaf cert for host, or generates,
// caches, and returns a new one.
//
// TODO(phase 2):
//  1. Lock mu, check cache map for host. If present, return it.
//  2. If absent:
//     a. generate a fresh key (ECDSA P256, cheap to generate per host)
//     b. build an x509.Certificate template with:
//        - Subject.CommonName = host
//        - DNSNames = []string{host}  (SANs matter more than CN on modern browsers)
//        - NotBefore/NotAfter (e.g. valid for 1 year, or just match root CA's window)
//        - IsCA: false
//     c. sign with x509.CreateCertificate(rand.Reader, template, a.Cert, &key.PublicKey, a.Key)
//        (a.Cert / a.Key = the root Authority from ca.go)
//     d. wrap into a tls.Certificate{Certificate: [][]byte{leafDER, a.Cert.Raw}, PrivateKey: key}
//        (include the root cert in the chain so clients can verify it)
//     e. store in cache map, unlock, return
func (a *Authority) GetOrCreateLeaf(host string) (*tls.Certificate, error) {
	panic("TODO: implement GetOrCreateLeaf (phase 2)")
}
