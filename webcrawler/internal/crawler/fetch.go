package crawler

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Page holds the extracted data from a single fetched page.
type Page struct {
	URL    string
	Title  string
	Links  []string
	Status int
}

var (
	// crude but effective regexes for extracting what we need from raw HTML.
	// A real parser (like x/net/html) would be more robust against edge
	// cases, but requires an external dependency.
	hrefRe  = regexp.MustCompile(`(?i)<a\s+[^>]*href\s*=\s*["']([^"'#]+)["']`)
	titleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
)

// httpClient is reused across requests (connection pooling, timeouts).
var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

// Fetch downloads a page and extracts its title and outbound links.
// Links are resolved to absolute URLs relative to the page's own URL.
func Fetch(rawURL string) (Page, error) {
	page := Page{URL: rawURL}

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return page, fmt.Errorf("building request: %w", err)
	}
	// Identify ourselves honestly - good crawling etiquette.
	req.Header.Set("User-Agent", "webcrawler-learning-tool/0.1 (+https://example.local)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return page, fmt.Errorf("fetching %s: %w", rawURL, err)
	}
	defer resp.Body.Close()
	page.Status = resp.StatusCode

	if resp.StatusCode != http.StatusOK {
		return page, fmt.Errorf("%s returned status %d", rawURL, resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		return page, fmt.Errorf("%s is not HTML (content-type: %s)", rawURL, contentType)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024)) // cap at 5MB
	if err != nil {
		return page, fmt.Errorf("reading body: %w", err)
	}
	html := string(body)

	if m := titleRe.FindStringSubmatch(html); len(m) > 1 {
		page.Title = strings.TrimSpace(collapseWhitespace(m[1]))
	}

	base, err := url.Parse(rawURL)
	if err != nil {
		return page, fmt.Errorf("parsing base url: %w", err)
	}

	seen := make(map[string]bool)
	for _, m := range hrefRe.FindAllStringSubmatch(html, -1) {
		link := strings.TrimSpace(m[1])
		if link == "" || strings.HasPrefix(link, "javascript:") || strings.HasPrefix(link, "mailto:") {
			continue
		}
		resolved, err := base.Parse(link)
		if err != nil {
			continue
		}
		if resolved.Scheme != "http" && resolved.Scheme != "https" {
			continue
		}
		resolved.Fragment = ""
		abs := resolved.String()
		if !seen[abs] {
			seen[abs] = true
			page.Links = append(page.Links, abs)
		}
	}

	return page, nil
}

func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
