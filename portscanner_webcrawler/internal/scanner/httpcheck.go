package scanner

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// HTTPCheckResult holds findings from a lightweight web misconfig check.
type HTTPCheckResult struct {
	Port            int
	Scheme          string
	MissingHeaders  []string
	ExposedPaths    []string
	InsecureCookies []string // cookies set without Secure/HttpOnly
	TLSVersion      string   // empty if scheme is http
	Err             error
}

var securityHeaders = []string{
	"Content-Security-Policy",
	"X-Frame-Options",
	"X-Content-Type-Options",
	"Strict-Transport-Security",
}

// commonExposedPaths are files/dirs that should never be publicly reachable.
var commonExposedPaths = []string{
	"/.env",
	"/.git/HEAD",
	"/.git/config",
	"/backup.sql",
	"/config.php.bak",
}

var httpCheckClient = &http.Client{
	Timeout: 5 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse // don't follow redirects, we want raw response
	},
}

// CheckHTTP probes host:port for common web misconfigurations.
// scheme should be "http" or "https".
func CheckHTTP(host string, port int, scheme string) HTTPCheckResult {
	res := HTTPCheckResult{Port: port, Scheme: scheme}
	base := fmt.Sprintf("%s://%s:%d", scheme, host, port)

	// 1. Fetch root page, check security headers + cookies
	req, err := http.NewRequest(http.MethodGet, base+"/", nil)
	if err != nil {
		res.Err = err
		return res
	}
	req.Header.Set("User-Agent", "portscanner-httpcheck/1.0")

	resp, err := httpCheckClient.Do(req)
	if err != nil {
		res.Err = err
		return res
	}
	defer resp.Body.Close()

	for _, h := range securityHeaders {
		if resp.Header.Get(h) == "" {
			res.MissingHeaders = append(res.MissingHeaders, h)
		}
	}

	for _, c := range resp.Cookies() {
		if !c.Secure || !c.HttpOnly {
			res.InsecureCookies = append(res.InsecureCookies, c.Name)
		}
	}

	if scheme == "https" && resp.TLS != nil {
		res.TLSVersion = tlsVersionName(resp.TLS.Version)
	}

	// 2. Probe commonly-exposed sensitive paths
	for _, p := range commonExposedPaths {
		if isExposed(base + p) {
			res.ExposedPaths = append(res.ExposedPaths, p)
		}
	}

	return res
}

// isExposed does a lightweight GET and treats 200 OK as "reachable".
func isExposed(url string) bool {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "portscanner-httpcheck/1.0")

	resp, err := httpCheckClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

func tlsVersionName(v uint16) string {
	switch v {
	case tls.VersionTLS10:
		return "TLS 1.0 (weak)"
	case tls.VersionTLS11:
		return "TLS 1.1 (weak)"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return "unknown"
	}
}

// FormatHTTPCheck renders one result as a readable terminal block.
func FormatHTTPCheck(r HTTPCheckResult) string {
	if r.Err != nil {
		return fmt.Sprintf("  [httpcheck] port %d: skipped (%v)", r.Port, r.Err)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "  [httpcheck] port %d (%s)\n", r.Port, r.Scheme)

	if len(r.MissingHeaders) > 0 {
		fmt.Fprintf(&sb, "    missing headers: %s\n", strings.Join(r.MissingHeaders, ", "))
	}
	if len(r.ExposedPaths) > 0 {
		fmt.Fprintf(&sb, "    exposed paths:   %s\n", strings.Join(r.ExposedPaths, ", "))
	}
	if len(r.InsecureCookies) > 0 {
		fmt.Fprintf(&sb, "    insecure cookies: %s\n", strings.Join(r.InsecureCookies, ", "))
	}
	if r.TLSVersion != "" {
		fmt.Fprintf(&sb, "    tls version:     %s\n", r.TLSVersion)
	}
	if len(r.MissingHeaders) == 0 && len(r.ExposedPaths) == 0 && len(r.InsecureCookies) == 0 {
		sb.WriteString("    no obvious issues found\n")
	}

	return sb.String()
}