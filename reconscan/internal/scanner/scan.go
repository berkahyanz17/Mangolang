package scanner

import (
	"bufio"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
)

// Result represents the outcome of probing a single port.
type Result struct {
	Port    int
	Open    bool
	Service string
	Banner  string // only populated if GrabBanner is enabled and something was read
	Err     error  // non-nil for unexpected errors (not just "closed")
}

// Options configures a scan run.
type Options struct {
	Concurrency int           // number of ports probed in parallel
	Timeout     time.Duration // dial timeout per port
	GrabBanner  bool          // attempt to read a short banner from open ports
}

// Scan probes every port in `ports` on `host` using a pool of worker
// goroutines, and streams results back on the returned channel as they
// complete (order is NOT guaranteed — sort afterwards if you need it).
// The channel is closed once every port has been probed.
//
// This is a "TCP connect scan": it performs a full TCP handshake
// (connect()) to each port, which is the same technique tools like a
// basic `nc -zv` or the connect-scan mode of nmap use. It's slower than a
// raw SYN scan but doesn't require elevated privileges or raw sockets.
func Scan(host string, ports []int, opts Options) <-chan Result {
	out := make(chan Result)

	if opts.Concurrency < 1 {
		opts.Concurrency = 1
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 2 * time.Second
	}

	go func() {
		defer close(out)

		jobs := make(chan int, len(ports))
		for _, p := range ports {
			jobs <- p
		}
		close(jobs) // all jobs are known upfront, so we can close right away

		var wg sync.WaitGroup
		for i := 0; i < opts.Concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for port := range jobs {
					out <- probe(host, port, opts)
				}
			}()
		}

		wg.Wait()
	}()

	return out
}

// probe checks a single port and optionally grabs a banner.
func probe(host string, port int, opts Options) Result {
	address := net.JoinHostPort(host, fmt.Sprint(port))

	conn, err := net.DialTimeout("tcp", address, opts.Timeout)
	if err != nil {
		// A dial failure (connection refused/timeout) just means the port
		// is closed or filtered — this is expected and not a real error.
		return Result{Port: port, Open: false, Service: ServiceName(port)}
	}
	defer conn.Close()

	result := Result{Port: port, Open: true, Service: ServiceName(port)}

	if opts.GrabBanner {
		result.Banner = grabBanner(conn)
	}

	return result
}

// grabBanner attempts a quick, best-effort read from an open connection.
// Many services (SSH, FTP, SMTP) send a greeting banner immediately on
// connect without needing any input from us.
func grabBanner(conn net.Conn) string {
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return ""
	}
	return strings.TrimSpace(line)
}

// SortResults sorts results by port number ascending — handy since Scan()
// delivers them in completion order, not port order.
func SortResults(results []Result) {
	sort.Slice(results, func(i, j int) bool {
		return results[i].Port < results[j].Port
	})
}
