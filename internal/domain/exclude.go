package domain

import (
	"fmt"
	"regexp"
	"strings"
)

func ExcludeFindings(findings []Finding, patterns []string) ([]Finding, int, error) {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if strings.TrimSpace(p) == "" {
			continue
		}
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid exclude pattern %q: %w", p, err)
		}
		compiled = append(compiled, re)
	}

	if len(compiled) == 0 {
		return findings, 0, nil
	}

	kept := make([]Finding, 0, len(findings))
	excluded := 0
	for _, f := range findings {
		if matchesExclude(f, compiled) {
			excluded++
			continue
		}
		kept = append(kept, f)
	}
	return kept, excluded, nil
}

func matchesExclude(f Finding, patterns []*regexp.Regexp) bool {
	for _, re := range patterns {
		if re.MatchString(f.File) || re.MatchString(f.Rule) || re.MatchString(f.Message) {
			return true
		}
	}
	return false
}
