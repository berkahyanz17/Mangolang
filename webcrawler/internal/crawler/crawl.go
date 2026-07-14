package crawler

import (
	"fmt"
	"net/url"
	"time"
)

// Result represents one successfully crawled page, tagged with its depth.
type Result struct {
	Page
	Depth int
	Err   error
}

// Options configures a crawl run.
type Options struct {
	MaxDepth       int           // 0 = only the start page, no following links
	MaxPages       int           // safety cap on total pages fetched
	SameDomainOnly bool          // only follow links on the same host as the start URL
	Delay          time.Duration // polite delay between requests
}

// Crawl performs a breadth-first crawl starting from startURL and streams
// results back on the returned channel. The channel is closed when the
// crawl finishes.
func Crawl(startURL string, opts Options) <-chan Result {
	out := make(chan Result)

	go func() {
		defer close(out)

		startHost := ""
		if u, err := url.Parse(startURL); err == nil {
			startHost = u.Host
		}

		type queued struct {
			url   string
			depth int
		}

		visited := map[string]bool{startURL: true}
		queue := []queued{{startURL, 0}}
		fetched := 0

		for len(queue) > 0 {
			if opts.MaxPages > 0 && fetched >= opts.MaxPages {
				return
			}

			item := queue[0]
			queue = queue[1:]

			page, err := Fetch(item.url)
			fetched++
			out <- Result{Page: page, Depth: item.depth, Err: err}

			if err != nil {
				continue
			}

			if item.depth >= opts.MaxDepth {
				continue
			}

			for _, link := range page.Links {
				if visited[link] {
					continue
				}
				if opts.SameDomainOnly {
					lu, err := url.Parse(link)
					if err != nil || lu.Host != startHost {
						continue
					}
				}
				visited[link] = true
				queue = append(queue, queued{link, item.depth + 1})
			}

			if opts.Delay > 0 {
				time.Sleep(opts.Delay)
			}
		}
	}()

	return out
}

// FormatResult gives a human-readable one-line summary of a Result.
func FormatResult(r Result) string {
	if r.Err != nil {
		return fmt.Sprintf("[depth %d] ERROR %s: %v", r.Depth, r.URL, r.Err)
	}
	title := r.Title
	if title == "" {
		title = "(no title)"
	}
	return fmt.Sprintf("[depth %d] %s — %s (%d links found)", r.Depth, r.URL, title, len(r.Links))
}
