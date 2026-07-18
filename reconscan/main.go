package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"reconscan/internal/crawler"
	"reconscan/internal/scanner"
)

func main() {
	// ---------------------------------------------------------------
	// a) Flag parsing — semua flag WAJIB dideklarasikan di sini,
	//    di dalam func main(), sebelum flag.Parse() dipanggil.
	// ---------------------------------------------------------------
	host := flag.String("host", "", "target host to scan (required)")
	portsSpec := flag.String("ports", "1-1024", "ports to scan, e.g. \"22,80,8000-8100\"")
	workers := flag.Int("workers", 100, "scan concurrency")
	timeout := flag.Duration("timeout", 2*time.Second, "dial timeout per port")
	grabBanner := flag.Bool("banner", true, "attempt banner grabbing on open ports")
	showClosed := flag.Bool("show-closed", false, "also print closed/filtered ports")
	output := flag.String("output", "", "write results to CSV at this path")

	runCrawl := flag.Bool("crawl", false, "crawl open http/https ports found by the scan")
	crawlDepth := flag.Int("crawl-depth", 1, "max crawl depth")
	crawlMaxPages := flag.Int("crawl-max-pages", 20, "max pages to crawl per open web port")
	crawlWorkers := flag.Int("crawl-workers", 4, "crawler concurrency")
	crawlDelay := flag.Duration("crawl-delay", 200*time.Millisecond, "global delay between crawl requests")
	respectRobots := flag.Bool("respect-robots", true, "respect robots.txt while crawling")

	runHTTPCheck := flag.Bool("httpcheck", false, "run web misconfig check on open http/https ports")

	flag.Parse()

	// ---------------------------------------------------------------
	// b) Validasi
	// ---------------------------------------------------------------
	if *host == "" {
		fmt.Println("usage: portscanner -host <target> [-ports 1-1024] [-crawl] [-httpcheck]")
		os.Exit(1)
	}

	ports, err := scanner.ParsePorts(*portsSpec)
	if err != nil {
		fmt.Println("error parsing -ports:", err)
		os.Exit(1)
	}

	// ---------------------------------------------------------------
	// TAHAP 1 — Port scan
	// ---------------------------------------------------------------
	var results []scanner.Result
	for r := range scanner.Scan(*host, ports, scanner.Options{
		Concurrency: *workers,
		Timeout:     *timeout,
		GrabBanner:  *grabBanner,
	}) {
		results = append(results, r)
	}
	scanner.SortResults(results)

	openCount := 0
	for _, r := range results {
		if r.Open {
			openCount++
			printResult(r)
		} else if *showClosed {
			printResult(r)
		}
	}
	fmt.Printf("\nScan selesai: %d/%d port open\n", openCount, len(results))

	if *output != "" {
		if err := writeCSV(*output, results); err != nil {
			fmt.Println("gagal nulis CSV:", err)
		} else {
			fmt.Println("hasil disimpan ke", *output)
		}
	}

	// ---------------------------------------------------------------
	// TAHAP 2 — Crawl (opsional, hanya port http/https yang open)
	// ---------------------------------------------------------------
	if *runCrawl {
		for _, r := range results {
			if !r.Open || (r.Service != "http" && r.Service != "https") {
				continue
			}

			startURL := fmt.Sprintf("%s://%s:%d", r.Service, *host, r.Port)
			fmt.Printf("\n--- Crawling %s ---\n", startURL)

			crawlOpts := crawler.Options{
				MaxDepth:       *crawlDepth,
				MaxPages:       *crawlMaxPages,
				SameDomainOnly: true,
				Delay:          *crawlDelay,
				Concurrency:    *crawlWorkers,
				RespectRobots:  *respectRobots,
			}

			for cr := range crawler.Crawl(startURL, crawlOpts) {
				fmt.Println(crawler.FormatResult(cr))
			}
		}
	}

	// ---------------------------------------------------------------
	// TAHAP 3 — HTTP misconfig check (opsional, hanya port http/https yang open)
	// ---------------------------------------------------------------
	if *runHTTPCheck {
		fmt.Println("\n--- Web Misconfig Check ---")
		for _, r := range results {
			if !r.Open || (r.Service != "http" && r.Service != "https") {
				continue
			}
			check := scanner.CheckHTTP(*host, r.Port, r.Service)
			fmt.Print(scanner.FormatHTTPCheck(check))
		}
	}
}

// printResult prints one scan result as a readable terminal line.
func printResult(r scanner.Result) {
	status := "CLOSED"
	if r.Open {
		status = "OPEN"
	}
	line := fmt.Sprintf("%-6d %-8s %-8s", r.Port, status, r.Service)
	if r.Banner != "" {
		line += " banner=" + strconv.Quote(r.Banner)
	}
	fmt.Println(line)
}

// writeCSV writes all scan results (open and closed) to path as CSV.
func writeCSV(path string, results []scanner.Result) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{"port", "open", "service", "banner"}); err != nil {
		return err
	}
	for _, r := range results {
		if err := w.Write([]string{
			strconv.Itoa(r.Port),
			strconv.FormatBool(r.Open),
			r.Service,
			r.Banner,
		}); err != nil {
			return err
		}
	}
	return nil
}