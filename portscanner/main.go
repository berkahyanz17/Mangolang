package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"time"

	"portscanner/internal/scanner"
)

func main() {
	host := flag.String("host", "", "target host to scan (IP or hostname) - required")
	portsSpec := flag.String("ports", "1-1024", `ports to scan, e.g. "80", "1-1024", or "22,80,443,8000-8100"`)
	workers := flag.Int("workers", 100, "number of concurrent probes")
	timeoutMs := flag.Int("timeout-ms", 1500, "dial timeout per port in milliseconds")
	banner := flag.Bool("banner", false, "attempt to grab a service banner from open ports")
	showClosed := flag.Bool("show-closed", false, "also print closed/filtered ports (default: only show open)")
	output := flag.String("output", "", "optional CSV file path to save results")
	flag.Parse()

	if *host == "" {
		fmt.Println(`usage: portscanner -host <target> [-ports "1-1024"] [-workers 100] [-banner] [-output results.csv]`)
		os.Exit(1)
	}

	ports, err := scanner.ParsePorts(*portsSpec)
	if err != nil {
		fmt.Println("error parsing -ports:", err)
		os.Exit(1)
	}

	opts := scanner.Options{
		Concurrency: *workers,
		Timeout:     time.Duration(*timeoutMs) * time.Millisecond,
		GrabBanner:  *banner,
	}

	fmt.Printf("Scanning %s (%d ports, %d workers)...\n\n", *host, len(ports), *workers)

	start := time.Now()
	var results []scanner.Result
	for r := range scanner.Scan(*host, ports, opts) {
		results = append(results, r)
	}
	elapsed := time.Since(start)

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

	fmt.Printf("\nScan complete in %s. %d/%d ports open.\n", elapsed.Round(time.Millisecond), openCount, len(ports))

	if *output != "" {
		if err := writeCSV(*output, results); err != nil {
			fmt.Println("error writing CSV:", err)
			os.Exit(1)
		}
		fmt.Printf("Results saved to %s\n", *output)
	}
}

func printResult(r scanner.Result) {
	status := "CLOSED"
	if r.Open {
		status = "OPEN"
	}
	line := fmt.Sprintf("%-6d %-8s %s", r.Port, status, r.Service)
	if r.Banner != "" {
		line += fmt.Sprintf("  [%s]", r.Banner)
	}
	fmt.Println(line)
}

func writeCSV(path string, results []scanner.Result) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	w.Write([]string{"port", "status", "service", "banner"})
	for _, r := range results {
		status := "closed"
		if r.Open {
			status = "open"
		}
		w.Write([]string{fmt.Sprint(r.Port), status, r.Service, r.Banner})
	}
	return nil
}
