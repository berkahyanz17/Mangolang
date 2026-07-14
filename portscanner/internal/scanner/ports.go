package scanner

import (
	"fmt"
	"strconv"
	"strings"
)

// ParsePorts parses a port spec string into a sorted, de-duplicated slice
// of port numbers. Supported formats (comma-separated, can be mixed):
//
//	"80"            single port
//	"1-1024"        inclusive range
//	"80,443,8000-8100"  combination of the above
func ParsePorts(spec string) ([]int, error) {
	seen := make(map[int]bool)
	var result []int

	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			if len(bounds) != 2 {
				return nil, fmt.Errorf("invalid range %q", part)
			}
			start, err := strconv.Atoi(strings.TrimSpace(bounds[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid range start in %q: %w", part, err)
			}
			end, err := strconv.Atoi(strings.TrimSpace(bounds[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid range end in %q: %w", part, err)
			}
			if start > end {
				start, end = end, start
			}
			if err := validatePort(start); err != nil {
				return nil, err
			}
			if err := validatePort(end); err != nil {
				return nil, err
			}
			for p := start; p <= end; p++ {
				if !seen[p] {
					seen[p] = true
					result = append(result, p)
				}
			}
			continue
		}

		p, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid port %q: %w", part, err)
		}
		if err := validatePort(p); err != nil {
			return nil, err
		}
		if !seen[p] {
			seen[p] = true
			result = append(result, p)
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no ports specified")
	}

	return result, nil
}

func validatePort(p int) error {
	if p < 1 || p > 65535 {
		return fmt.Errorf("port %d out of range (1-65535)", p)
	}
	return nil
}
