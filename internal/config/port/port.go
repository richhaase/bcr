package port

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/richhaase/bcr/internal/config"
)

var acrAgentDefaultModel = map[string]string{
	"claude": "anthropic/claude-sonnet-4-5",
	"codex":  "openai/gpt-5.3-codex",
	"agy":    "google/gemini-2.5-pro",
	"gemini": "google/gemini-2.5-pro",
}

var ErrExists = errors.New("target already exists")

type acrFilters struct {
	ExcludePatterns *[]string `yaml:"exclude_patterns"`
}

type acrFPFilter struct {
	Enabled   *bool    `yaml:"enabled"`
	Threshold *float64 `yaml:"threshold"`
}

type acrConfig struct {
	Concurrency       *int           `yaml:"concurrency"`
	Base              *string        `yaml:"base"`
	Retries           *int           `yaml:"retries"`
	PRFeedback        *bool          `yaml:"pr_feedback"`
	Filters           *acrFilters    `yaml:"filters"`
	GuidanceFile      *string        `yaml:"guidance_file"`
	SummarizerModel   *string        `yaml:"summarizer_model"`
	ReviewerAgents    *[]string      `yaml:"reviewer_agents"`
	ReviewerAgent     *string        `yaml:"reviewer_agent"`
	SummarizerAgent   *string        `yaml:"summarizer_agent"`
	Reviewers         *int           `yaml:"reviewers"`
	Timeout           *int           `yaml:"timeout"`
	Fetch             *int           `yaml:"fetch"`
	SummarizerTimeout *int           `yaml:"summarizer_timeout"`
	FPFilterTimeout   *int           `yaml:"fp_filter_timeout"`
	FPFilter          *acrFPFilter   `yaml:"fp_filter"`
	ReviewerModel     *string        `yaml:"reviewer_model"`
	Watch             map[string]any `yaml:"watch"`
	Adjudication      map[string]any `yaml:"adjudication"`
}

type PortResult struct {
	Config   *config.Config
	Warnings []string
	Source   string
}

func FindACRConfig() (string, error) {
	if _, err := os.Stat(".acr.yaml"); err == nil {
		if abs, err := filepath.Abs(".acr.yaml"); err == nil {
			return abs, nil
		}
		return ".acr.yaml", nil
	}

	if dir := acrConfigDir(); dir != "" {
		p := filepath.Join(dir, "acr", "config.yaml")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", nil
}

func Port(path string) (*PortResult, error) {
	res := &PortResult{Config: config.DefaultConfig(), Source: path}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read acr config %q: %w", path, err)
	}

	var acr acrConfig
	if err := yaml.Unmarshal(data, &acr); err != nil {
		return nil, fmt.Errorf("parse acr config %q: %w", path, err)
	}

	if acr.Base != nil {
		res.Config.Base = *acr.Base
	}
	if acr.Concurrency != nil && *acr.Concurrency > 0 {
		res.Config.Concurrency = *acr.Concurrency
	}
	if acr.Retries != nil && *acr.Retries > 0 {
		res.Config.Retries = *acr.Retries
	}
	if acr.PRFeedback != nil {
		res.Config.PRFeedback = *acr.PRFeedback
	}
	if acr.Filters != nil && acr.Filters.ExcludePatterns != nil {
		res.Config.Exclude = *acr.Filters.ExcludePatterns
	}
	if acr.GuidanceFile != nil {
		res.Config.GuidanceFile = *acr.GuidanceFile
	}
	if acr.SummarizerModel != nil && *acr.SummarizerModel != "" {
		res.Config.SummarizerModel = *acr.SummarizerModel
	}

	var agents []string
	if acr.ReviewerAgents != nil {
		agents = append(agents, *acr.ReviewerAgents...)
	}
	if acr.ReviewerAgent != nil && *acr.ReviewerAgent != "" {
		agents = append(agents, *acr.ReviewerAgent)
	}
	if len(agents) > 0 {
		var models []string
		seen := make(map[string]bool)
		for _, a := range agents {
			m, ok := acrAgentDefaultModel[a]
			if !ok {
				res.Warnings = append(res.Warnings, fmt.Sprintf("no model mapping for agent %q", a))
				continue
			}
			if !seen[m] {
				seen[m] = true
				models = append(models, m)
			}
		}
		if len(models) > 0 {
			res.Config.Models = models
		}
	}

	if acr.SummarizerAgent != nil && *acr.SummarizerAgent != "" {
		if m, ok := acrAgentDefaultModel[*acr.SummarizerAgent]; ok {
			res.Config.SummarizerModel = m
		} else {
			res.Warnings = append(res.Warnings, fmt.Sprintf("no model mapping for summarizer agent %q", *acr.SummarizerAgent))
		}
	}

	if acr.Reviewers != nil {
		res.Warnings = append(res.Warnings, "reviewers count has no BCR model list equivalent; reviewer agents were used instead")
	}
	if acr.ReviewerModel != nil {
		res.Warnings = append(res.Warnings, fmt.Sprintf("reviewer_model %q is ambiguous; no BCR equivalent", *acr.ReviewerModel))
	}
	if acr.Timeout != nil {
		res.Warnings = append(res.Warnings, "timeout has no BCR equivalent and was not ported")
	}
	if acr.Fetch != nil {
		res.Warnings = append(res.Warnings, "fetch has no BCR equivalent and was not ported")
	}
	if acr.SummarizerTimeout != nil {
		res.Warnings = append(res.Warnings, "summarizer_timeout has no BCR equivalent and was not ported")
	}
	if acr.FPFilterTimeout != nil {
		res.Warnings = append(res.Warnings, "fp_filter_timeout has no BCR equivalent and was not ported")
	}
	if acr.FPFilter != nil {
		if acr.FPFilter.Enabled != nil {
			res.Warnings = append(res.Warnings, "fp_filter.enabled has no BCR equivalent; the synthesizer always runs")
		}
		if acr.FPFilter.Threshold != nil {
			res.Warnings = append(res.Warnings, "fp_filter.threshold has no BCR equivalent and was not ported")
		}
	}
	if len(acr.Watch) > 0 {
		res.Warnings = append(res.Warnings, "watch settings are out of scope and were not ported")
	}
	if len(acr.Adjudication) > 0 {
		res.Warnings = append(res.Warnings, "adjudication settings are out of scope and were not ported")
	}

	return res, nil
}

type portedConfig struct {
	BaseURL         string   `yaml:"base_url"`
	Models          []string `yaml:"models"`
	SummarizerModel string   `yaml:"summarizer_model"`
	Base            string   `yaml:"base"`
	Temperature     float64  `yaml:"temperature"`
	Concurrency     int      `yaml:"concurrency"`
	Retries         int      `yaml:"retries"`
	PRFeedback      bool     `yaml:"pr_feedback"`
	Exclude         []string `yaml:"exclude"`
	Guidance        string   `yaml:"guidance"`
	GuidanceFile    string   `yaml:"guidance_file"`
}

func Write(path string, cfg *config.Config, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return ErrExists
		}
	}

	out := portedConfig{
		BaseURL:         cfg.BaseURL,
		Models:          cfg.Models,
		SummarizerModel: cfg.SummarizerModel,
		Base:            cfg.Base,
		Temperature:     cfg.Temperature,
		Concurrency:     cfg.Concurrency,
		Retries:         cfg.Retries,
		PRFeedback:      cfg.PRFeedback,
		Exclude:         cfg.Exclude,
		Guidance:        cfg.Guidance,
		GuidanceFile:    cfg.GuidanceFile,
	}

	data, err := yaml.Marshal(out)
	if err != nil {
		return fmt.Errorf("marshal bcr config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write bcr config %q: %w", path, err)
	}
	return nil
}

func acrConfigDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return dir
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config")
	}
	return ""
}
