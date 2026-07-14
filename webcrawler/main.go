package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"time"

	"webcrawler/internal/crawler"
)

func main() {
	url := flag.String("url", "", "starting URL to crawl (required)")
	depth := flag.Int("depth", 1, "how many link-hops to follow from the start page")
	maxPages := flag.Int("max-pages", 20, "safety cap on total pages fetched")
	sameDomain := flag.Bool("same-domain", true, "only follow links on the same domain as -url")
	delayMs := flag.Int("delay-ms", 300, "delay between requests in milliseconds (be polite to servers)")
	output := flag.String("output", "", "optional CSV file path to save results")
	workers := flag.Int("workers", 4, "number of concurrent fetch workers")
	respectRobots := flag.Bool("respect-robots", true, "check robots.txt before crawling each URL")
	flag.Parse()

	if *url == "" {
		fmt.Println("usage: webcrawler -url https://example.com [-depth 1] [-max-pages 20] [-output results.csv]")
		os.Exit(1)
	}

	opts := crawler.Options{
		MaxDepth:       *depth,
		MaxPages:       *maxPages,
		SameDomainOnly: *sameDomain,
		Delay:          time.Duration(*delayMs) * time.Millisecond,
		Concurrency:    *workers,
		RespectRobots:  *respectRobots,
	}

	var csvWriter *csv.Writer
	var csvFile *os.File
	if *output != "" {
		var err error
		csvFile, err = os.Create(*output)
		if err != nil {
			fmt.Println("error creating output file:", err)
			os.Exit(1)
		}
		defer csvFile.Close()
		csvWriter = csv.NewWriter(csvFile)
		defer csvWriter.Flush()
		csvWriter.Write([]string{"depth", "url", "title", "status", "link_count", "error"})
	}

	count := 0
	for r := range crawler.Crawl(*url, opts) {
		fmt.Println(crawler.FormatResult(r))
		count++

		if csvWriter != nil {
			errStr := ""
			if r.Err != nil {
				errStr = r.Err.Error()
			}
			csvWriter.Write([]string{
				fmt.Sprint(r.Depth),
				r.URL,
				r.Title,
				fmt.Sprint(r.Status),
				fmt.Sprint(len(r.Links)),
				errStr,
			})
		}
	}

	fmt.Printf("\nDone. %d page(s) fetched.\n", count)
	if *output != "" {
		fmt.Printf("Results saved to %s\n", *output)
	}
}
