package domain

import (
	"fmt"
	"sort"
	"strings"
)

func groupKey(f Finding) string {
	rule := strings.ToLower(strings.TrimSpace(f.Rule))
	file := strings.ToLower(strings.TrimSpace(f.File))
	return fmt.Sprintf("%s:%d:%s", file, f.Line, rule)
}

func GroupFindings(findings []Finding) []Group {
	byKey := make(map[string]*Group)

	for _, f := range findings {
		key := groupKey(f)
		g, ok := byKey[key]
		if !ok {
			g = &Group{
				Rule:       f.Rule,
				Category:   f.Category,
				Severity:   f.Severity,
				File:       f.File,
				Line:       f.Line,
				Message:    f.Message,
				Suggestion: f.Suggestion,
			}
			byKey[key] = g
		}
		g.Count++
		g.Agents = mergeAgent(g.Agents, f.Agent)
		if f.Confidence > g.Confidence {
			g.Confidence = f.Confidence
		}
		if g.Message == "" {
			g.Message = f.Message
		}
		if g.Suggestion == "" {
			g.Suggestion = f.Suggestion
		}
		if g.Category == "" {
			g.Category = f.Category
		}
	}

	out := make([]Group, 0, len(byKey))
	for _, g := range byKey {
		out = append(out, *g)
	}

	sort.SliceStable(out, func(i, j int) bool {
		wi, wj := severityWeight(out[i].Severity), severityWeight(out[j].Severity)
		if wi != wj {
			return wi > wj
		}
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		li, lj := out[i].Line, out[j].Line
		if li != lj {
			return li < lj
		}
		return out[i].File < out[j].File
	})

	return out
}

func severityWeight(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium", "warning", "warn":
		return 2
	case "low", "info":
		return 1
	default:
		return 2
	}
}

func mergeAgent(existing []string, agent string) []string {
	if agent == "" {
		return existing
	}
	for _, a := range existing {
		if a == agent {
			return existing
		}
	}
	return append(existing, agent)
}
