package review

import (
	"fmt"
	"strings"

	"github.com/richhaase/bcr/internal/domain"
)

func HasKept(run *domain.ReviewRun) bool {
	for _, f := range run.Final {
		if f.Keep {
			return true
		}
	}
	return false
}

func keptFindings(run *domain.ReviewRun) []domain.FinalFinding {
	var kept []domain.FinalFinding
	for _, f := range run.Final {
		if f.Keep {
			kept = append(kept, f)
		}
	}
	return kept
}

func PRBody(run *domain.ReviewRun) string {
	var b strings.Builder
	kept := keptFindings(run)

	if len(kept) == 0 {
		b.WriteString("## BCR Review\n\n:white_check_mark: **LGTM** — no actionable findings detected in the reviewed diff.")
		if run.Dismissed > 0 {
			fmt.Fprintf(&b, "\n\n_%d false positive/duplicate finding(s) filtered out._", run.Dismissed)
		}
		if run.Excluded > 0 {
			fmt.Fprintf(&b, "\n\n_%d finding(s) excluded by regex patterns._", run.Excluded)
		}
		b.WriteString("\n")
		return b.String()
	}

	fmt.Fprintf(&b, "## BCR Review\n\nFound %d actionable finding(s) across %d model(s):\n\n", len(kept), len(run.Models))
	for i, f := range kept {
		fmt.Fprintf(&b, "### %d. [%s] %s\n\n", i+1, strings.ToUpper(f.Severity), f.Message)
		fmt.Fprintf(&b, "- **File:** `%s:%d`\n", f.File, f.Line)
		if f.Rule != "" {
			fmt.Fprintf(&b, "- **Rule:** `%s`\n", f.Rule)
		}
		if f.Suggestion != "" {
			fmt.Fprintf(&b, "- **Suggestion:** %s\n", f.Suggestion)
		}
		b.WriteString("\n")
	}
	if run.Dismissed > 0 {
		fmt.Fprintf(&b, "\n_%d false positive/duplicate finding(s) filtered out._\n", run.Dismissed)
	}
	if run.Excluded > 0 {
		fmt.Fprintf(&b, "\n_%d finding(s) excluded by regex patterns._\n", run.Excluded)
	}
	return b.String()
}

func Disposition(run *domain.ReviewRun, ciOK bool, selfReview bool) (event string, reason string) {
	hasKept := HasKept(run)

	if selfReview {
		if hasKept {
			return "request-changes", "reviewing own PR with findings (approval not allowed)"
		}
		return "comment", "reviewing own PR; approval not allowed"
	}

	if hasKept {
		return "request-changes", "actionable findings present"
	}

	if ciOK {
		return "approve", "no actionable findings and CI passed"
	}

	return "comment", "review is clean but CI is not green; waiting on CI"
}
