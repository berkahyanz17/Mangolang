// Package reqedit provides shared request-editing primitives used by
// Match & Replace (regex-based, always-on rules) and, later, Intruder
// (placeholder-based payload substitution).
package reqedit

import (
	"fmt"
	"regexp"
	"sync"
)

// Target identifies which part of a request a Rule applies to.
type Target string

const (
	TargetHeader Target = "header"
	TargetBody   Target = "body"
	TargetURL    Target = "url"
)

// Rule is one Match & Replace rule: if Match (a regex) is found in the
// given Target, it's replaced with Replace (supports Go regexp
// backreferences like $1).
type Rule struct {
	ID      int64  `json:"id"`
	Enabled bool   `json:"enabled"`
	Target  Target `json:"target"`
	Match   string `json:"match"`
	Replace string `json:"replace"`
}

// RuleStore holds all configured rules, protected by a mutex - same
// pattern as intercept.Queue's pending map.
type RuleStore struct {
	mu     sync.Mutex
	nextID int64
	rules  []*Rule
}

func NewRuleStore() *RuleStore {
	return &RuleStore{}
}

// Add validates the rule's regex compiles, assigns it an ID, and stores it.
func (s *RuleStore) Add(r *Rule) (*Rule, error) {
	if _, err := regexp.Compile(r.Match); err != nil {
		return nil, fmt.Errorf("reqedit: invalid regex %q: %w", r.Match, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	r.ID = s.nextID
	s.rules = append(s.rules, r)
	return r, nil
}

func (s *RuleStore) List() []*Rule {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Rule, len(s.rules))
	copy(out, s.rules)
	return out
}

func (s *RuleStore) Delete(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, r := range s.rules {
		if r.ID == id {
			s.rules = append(s.rules[:i], s.rules[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("reqedit: no rule with id %d", id)
}

func (s *RuleStore) SetEnabled(id int64, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.rules {
		if r.ID == id {
			r.Enabled = enabled
			return nil
		}
	}
	return fmt.Errorf("reqedit: no rule with id %d", id)
}

// Apply runs every enabled rule for the given target against text, in the
// order they were added (so rule 2 sees the output of rule 1).
func (s *RuleStore) Apply(target Target, text string) string {
	s.mu.Lock()
	rules := make([]*Rule, len(s.rules))
	copy(rules, s.rules)
	s.mu.Unlock()

	for _, r := range rules {
		if !r.Enabled || r.Target != target {
			continue
		}
		re, err := regexp.Compile(r.Match)
		if err != nil {
			continue // already validated at Add time, but don't crash if something odd slips through
		}
		text = re.ReplaceAllString(text, r.Replace)
	}
	return text
}