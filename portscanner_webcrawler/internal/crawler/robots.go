package crawler

import (
	"bufio"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// RobotsChecker fetches and caches robots.txt rules per host, and answers
// whether a given URL is allowed to be crawled.
//
// This is a simplified implementation: it only looks at the rule group
// for User-agent "*", and only supports plain prefix-matching for
// Disallow/Allow paths (no wildcards like "*" or "$" inside paths).
// That covers the vast majority of real-world robots.txt files.
type RobotsChecker struct {
	mu     sync.Mutex
	cache  map[string]*robotsRules
	client *http.Client
}

type robotsRules struct {
	disallow []string
	allow    []string
}

// NewRobotsChecker creates a checker with its own short-timeout HTTP client
// (robots.txt fetches shouldn't block the crawl for long).
func NewRobotsChecker() *RobotsChecker {
	return &RobotsChecker{
		cache: make(map[string]*robotsRules),
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Allowed reports whether rawURL may be fetched according to the robots.txt
// of its host. If robots.txt can't be fetched or parsed, it defaults to
// allowing the request (fail-open), which matches how most crawlers behave
// for unreachable robots.txt.
func (rc *RobotsChecker) Allowed(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return true
	}

	rules := rc.rulesFor(u.Scheme, u.Host)
	if rules == nil {
		return true
	}

	path := u.Path
	if path == "" {
		path = "/"
	}

	// Longest matching rule wins (more specific rule takes precedence).
	longestAllow := longestPrefixMatch(rules.allow, path)
	longestDisallow := longestPrefixMatch(rules.disallow, path)

	if longestDisallow == -1 {
		return true
	}
	return longestAllow >= longestDisallow
}

func longestPrefixMatch(prefixes []string, path string) int {
	best := -1
	for _, p := range prefixes {
		if p == "" {
			continue
		}
		if strings.HasPrefix(path, p) && len(p) > best {
			best = len(p)
		}
	}
	return best
}

func (rc *RobotsChecker) rulesFor(scheme, host string) *robotsRules {
	rc.mu.Lock()
	if rules, ok := rc.cache[host]; ok {
		rc.mu.Unlock()
		return rules
	}
	rc.mu.Unlock()

	rules := rc.fetchAndParse(scheme, host)

	rc.mu.Lock()
	rc.cache[host] = rules
	rc.mu.Unlock()

	return rules
}

func (rc *RobotsChecker) fetchAndParse(scheme, host string) *robotsRules {
	robotsURL := scheme + "://" + host + "/robots.txt"

	resp, err := rc.client.Get(robotsURL)
	if err != nil {
		return nil // fail-open: couldn't reach it, assume allowed
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil // no robots.txt (404 etc.) = everything allowed
	}

	rules := &robotsRules{}
	relevant := false // are we inside a "User-agent: *" group?

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		field := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])

		switch field {
		case "user-agent":
			relevant = value == "*"
		case "disallow":
			if relevant && value != "" {
				rules.disallow = append(rules.disallow, value)
			}
		case "allow":
			if relevant && value != "" {
				rules.allow = append(rules.allow, value)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil
	}

	return rules
}
