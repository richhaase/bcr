package domain

type Finding struct {
	Rule       string  `json:"rule"`
	Category   string  `json:"category"`
	Severity   string  `json:"severity"`
	File       string  `json:"file"`
	Line       int     `json:"line"`
	Message    string  `json:"message"`
	Suggestion string  `json:"suggestion,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
	Agent      string  `json:"agent,omitempty"`
}

func (f Finding) valid() bool {
	return f.Rule != "" && f.File != "" && f.Message != ""
}

type FinalFinding struct {
	Rule          string   `json:"rule"`
	Category      string   `json:"category"`
	Severity      string   `json:"severity"`
	File          string   `json:"file"`
	Line          int      `json:"line"`
	Message       string   `json:"message"`
	Suggestion    string   `json:"suggestion,omitempty"`
	Confidence    float64  `json:"confidence,omitempty"`
	Keep          bool     `json:"keep"`
	DismissReason string   `json:"dismiss_reason,omitempty"`
	Agents        []string `json:"agents,omitempty"`
	Count         int      `json:"count,omitempty"`
}

func (f FinalFinding) valid() bool {
	return f.Rule != "" && f.File != "" && f.Message != ""
}

type Group struct {
	Rule       string
	Category   string
	Severity   string
	File       string
	Line       int
	Message    string
	Suggestion string
	Confidence float64
	Count      int
	Agents     []string
}

type ReviewRun struct {
	Diff      string
	Findings  []Finding
	Final     []FinalFinding
	Excluded  int
	Dismissed int
	Models    []string
}
