package crawler

import (
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"
)

// ErrRobotsDisallowed is returned (inside a Result, not as a Go error you
// check with if-err-return) when robots.txt blocks a URL from being crawled.
var ErrRobotsDisallowed = errors.New("blocked by robots.txt")

// Result represents one crawled page (or a skipped/failed attempt), tagged
// with its depth in the crawl tree.
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
	Delay          time.Duration // global polite delay between requests (shared across workers)
	Concurrency    int           // number of worker goroutines fetching in parallel
	RespectRobots  bool          // check robots.txt before fetching each URL
}

type job struct {
	url   string
	depth int
}

// Crawl performs a breadth-first crawl starting from startURL using a pool
// of concurrent workers, and streams results back on the returned channel.
// The channel is closed when the crawl finishes.
//
// Concurrency model (worth understanding, not just copying):
//   - `jobs` is a buffered channel acting as the work queue. Multiple
//     worker goroutines pull from it concurrently.
//   - A sync.WaitGroup (`pending`) tracks how many jobs are queued or being
//     processed. Every time a URL is queued, we call pending.Add(1). Every
//     time a worker finishes processing one job (success, failure, or
//     skipped), it calls pending.Done(). A separate goroutine waits on
//     `pending` and closes `jobs` once it hits zero — that's how we detect
//     "the crawl is fully finished" without a fixed number of iterations.
//   - `visitedMu` + `visited` map prevent two workers from queuing the same
//     URL twice (classic shared-state race, hence the mutex).
//   - `fetchCount` uses a mutex too, to enforce -max-pages safely across
//     goroutines.
//   - The global rate limiter (a time.Ticker) is shared by all workers, so
//     -delay-ms limits the *overall* request rate regardless of how many
//     workers you run — increasing -workers gets you more parallel
//     connections in flight, not a faster hammering rate.
func Crawl(startURL string, opts Options) <-chan Result {
	out := make(chan Result)

	if opts.Concurrency < 1 {
		opts.Concurrency = 1
	}

	go func() {
		defer close(out)

		startHost := ""
		if u, err := url.Parse(startURL); err == nil {
			startHost = u.Host
		}

		jobs := make(chan job, 1000)

		var visitedMu sync.Mutex
		visited := map[string]bool{startURL: true}

		var countMu sync.Mutex
		fetchCount := 0

		var pending sync.WaitGroup

		var robots *RobotsChecker
		if opts.RespectRobots {
			robots = NewRobotsChecker()
		}

		var ticker *time.Ticker
		if opts.Delay > 0 {
			ticker = time.NewTicker(opts.Delay)
			defer ticker.Stop()
		}

		enqueue := func(u string, depth int) {
			pending.Add(1)
			jobs <- job{url: u, depth: depth}
		}

		// seed the queue with the start URL. This must happen synchronously
		// (not in a goroutine) and before the closer below starts waiting,
		// otherwise pending.Wait() could see a count of zero and close the
		// channel before the first job is even added.
		enqueue(startURL, 0)

		// closer: once all pending work is done, close the jobs channel so
		// workers know to stop.
		go func() {
			pending.Wait()
			close(jobs)
		}()

		var workerWg sync.WaitGroup
		for i := 0; i < opts.Concurrency; i++ {
			workerWg.Add(1)
			go func() {
				defer workerWg.Done()

				for j := range jobs {
					func() {
						defer pending.Done()

						countMu.Lock()
						if opts.MaxPages > 0 && fetchCount >= opts.MaxPages {
							countMu.Unlock()
							return
						}
						fetchCount++
						countMu.Unlock()

						if robots != nil && !robots.Allowed(j.url) {
							out <- Result{Page: Page{URL: j.url}, Depth: j.depth, Err: ErrRobotsDisallowed}
							return
						}

						if ticker != nil {
							<-ticker.C
						}

						page, err := Fetch(j.url)
						out <- Result{Page: page, Depth: j.depth, Err: err}

						if err != nil || j.depth >= opts.MaxDepth {
							return
						}

						for _, link := range page.Links {
							if opts.SameDomainOnly {
								lu, err := url.Parse(link)
								if err != nil || lu.Host != startHost {
									continue
								}
							}

							visitedMu.Lock()
							already := visited[link]
							if !already {
								visited[link] = true
							}
							visitedMu.Unlock()

							if !already {
								enqueue(link, j.depth+1)
							}
						}
					}()
				}
			}()
		}

		workerWg.Wait()
	}()

	return out
}

// FormatResult gives a human-readable one-line summary of a Result.
func FormatResult(r Result) string {
	if r.Err != nil {
		return fmt.Sprintf("[depth %d] SKIPPED %s: %v", r.Depth, r.URL, r.Err)
	}
	title := r.Title
	if title == "" {
		title = "(no title)"
	}
	return fmt.Sprintf("[depth %d] %s — %s (%d links found)", r.Depth, r.URL, title, len(r.Links))
}
