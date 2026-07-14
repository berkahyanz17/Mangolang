package proxy

import "net"

// mitmTLS is the core trick of the whole proxy: after handleConnect sends
// the "200 Connection Established" reply, the browser starts a TLS
// handshake AS IF it were talking directly to the real server. We
// intercept that handshake ourselves.
//
// TODO(phase 2):
//  1. Wrap conn with tls.Server(conn, &tls.Config{
//       GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
//           return p.opts.RootCA.GetOrCreateLeaf(hello.ServerName)
//       },
//     })
//     GetCertificate lets us mint a fresh leaf cert on demand using the
//     SNI hostname the browser sent in its ClientHello - this is exactly
//     how Burp/mitmproxy do it, no need to know the domain in advance.
//  2. tlsConn.Handshake() to actually perform it. If this fails (e.g. cert
//     pinning, or the client refuses our root CA), fall back to
//     passthrough tunnel or just close the connection.
//  3. Now read the decrypted HTTP request from tlsConn with
//     http.ReadRequest(bufio.NewReader(tlsConn)) - from here it's a normal
//     HTTP request/response cycle in cleartext, same handling as
//     handlePlainHTTP but talking over tlsConn instead of a raw conn.
//  4. Open a SEPARATE outbound TLS connection to the real server:
//     tls.Dial("tcp", targetHostPort, &tls.Config{ServerName: host})
//     forward the request there, read the response, log it (phase 3),
//     optionally hold it in the interceptor (phase 4), then write the
//     response back down tlsConn to the browser.
//  5. Loop (a single TLS connection can carry multiple HTTP requests via
//     keep-alive) until the client closes the connection.
func (p *Proxy) mitmTLS(conn net.Conn, host string) {
	panic("TODO: implement mitmTLS (phase 2)")
}
