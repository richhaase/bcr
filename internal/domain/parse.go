package domain

import (
	"encoding/json"
	"errors"
	"strings"
)

type findingSet struct {
	Findings []Finding `json:"findings"`
}

type finalFindingSet struct {
	Findings []FinalFinding `json:"findings"`
}

var errEmptyFindings = errors.New("model returned empty findings")

func ExtractJSON(body string) string {
	content := body
	if i := strings.Index(content, "```"); i >= 0 {
		content = content[i:]
		rest := strings.TrimPrefix(content[3:], "json")
		if j := strings.Index(rest, "```"); j >= 0 {
			rest = rest[:j]
		}
		content = rest
	}

	start := strings.IndexByte(content, '{')
	if start < 0 {
		return ""
	}

	depth := 0
	inStr := false
	escaped := false
	for i := start; i < len(content); i++ {
		c := content[i]
		if inStr {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return content[start : i+1]
			}
		}
	}
	return ""
}

func ParseFindings(body string) ([]Finding, error) {
	obj := ExtractJSON(body)
	if obj == "" {
		return nil, errEmptyFindings
	}

	var set findingSet
	if err := json.Unmarshal([]byte(obj), &set); err != nil {
		return nil, err
	}

	out := make([]Finding, 0, len(set.Findings))
	for _, f := range set.Findings {
		if f.valid() {
			out = append(out, f)
		}
	}
	return out, nil
}

func ParseFinalFindings(body string) ([]FinalFinding, error) {
	obj := ExtractJSON(body)
	if obj == "" {
		return nil, errEmptyFindings
	}

	var set finalFindingSet
	if err := json.Unmarshal([]byte(obj), &set); err != nil {
		return nil, err
	}

	out := make([]FinalFinding, 0, len(set.Findings))
	for _, f := range set.Findings {
		if f.valid() {
			out = append(out, f)
		}
	}
	return out, nil
}
