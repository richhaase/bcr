package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/richhaase/bcr/internal/domain"
	"github.com/richhaase/bcr/internal/prompt"
	"github.com/richhaase/bcr/internal/provider"
)

type Config struct {
	Models          []string
	SummarizerModel string
	BaseURL         string
	APIKey          string
	Diff            string
	Extra           map[string]string
	Temperature     float64
	Concurrency     int
}

type modelCompleter interface {
	Complete(ctx context.Context, model string, messages []provider.Message, temp float64) (string, error)
}

type Runner struct {
	client modelCompleter
	cfg    Config
}

func NewRunner(cfg Config) *Runner {
	client := provider.NewClient(cfg.BaseURL, cfg.APIKey)
	return &Runner{
		client: client,
		cfg:    cfg,
	}
}

func (r *Runner) Run(ctx context.Context) (*domain.ReviewRun, error) {
	if len(r.cfg.Models) == 0 {
		return nil, fmt.Errorf("no reviewer models configured")
	}

	type reviewerResult struct {
		model    string
		findings []domain.Finding
		err      error
	}

	limit := len(r.cfg.Models)
	if r.cfg.Concurrency > 0 && r.cfg.Concurrency < len(r.cfg.Models) {
		limit = r.cfg.Concurrency
	}
	sem := make(chan struct{}, limit)

	resCh := make(chan reviewerResult, len(r.cfg.Models))
	var wg sync.WaitGroup

	for _, m := range r.cfg.Models {
		wg.Add(1)
		go func(model string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			messages := []provider.Message{
				{Role: "system", Content: prompt.ReviewSystemPrompt},
			}

			if extra, ok := r.cfg.Extra[model]; ok && extra != "" {
				messages = append(messages, provider.Message{
					Role:    "system",
					Content: extra,
				})
			}

			messages = append(messages, provider.Message{
				Role:    "user",
				Content: fmt.Sprintf("Here is the git diff to review:\n\n```diff\n%s\n```", r.cfg.Diff),
			})

			resp, err := r.client.Complete(ctx, model, messages, r.cfg.Temperature)
			if err != nil {
				resCh <- reviewerResult{model: model, err: err}
				return
			}

			findings, err := domain.ParseFindings(resp)
			if err != nil {
				resCh <- reviewerResult{model: model, err: err}
				return
			}

			for i := range findings {
				findings[i].Agent = model
			}

			resCh <- reviewerResult{model: model, findings: findings}
		}(m)
	}

	wg.Wait()
	close(resCh)

	var allFindings []domain.Finding
	for res := range resCh {
		if res.err == nil {
			allFindings = append(allFindings, res.findings...)
		}
	}

	groups := domain.GroupFindings(allFindings)

	if len(groups) == 0 {
		return &domain.ReviewRun{
			Diff:     r.cfg.Diff,
			Findings: allFindings,
			Final:    nil,
			Models:   r.cfg.Models,
		}, nil
	}

	groupsJSON, err := json.Marshal(groups)
	if err != nil {
		return nil, fmt.Errorf("marshal deduplicated groups: %w", err)
	}

	summarizerModel := r.cfg.SummarizerModel
	if summarizerModel == "" {
		summarizerModel = r.cfg.Models[0]
	}

	sumMessages := []provider.Message{
		{Role: "system", Content: prompt.SummarizerSystemPrompt},
		{
			Role: "user",
			Content: fmt.Sprintf(
				"Here is the git diff:\n\n```diff\n%s\n```\n\nHere are the findings gathered from parallel reviewers:\n\n```json\n%s\n```",
				r.cfg.Diff,
				string(groupsJSON),
			),
		},
	}

	sumResp, err := r.client.Complete(ctx, summarizerModel, sumMessages, 0.1)
	if err != nil {
		var fallbackFinal []domain.FinalFinding
		for _, g := range groups {
			fallbackFinal = append(fallbackFinal, domain.FinalFinding{
				Rule:       g.Rule,
				Category:   g.Category,
				Severity:   g.Severity,
				File:       g.File,
				Line:       g.Line,
				Message:    g.Message,
				Suggestion: g.Suggestion,
				Confidence: g.Confidence,
				Keep:       true,
				Agents:     g.Agents,
				Count:      g.Count,
			})
		}
		return &domain.ReviewRun{
			Diff:     r.cfg.Diff,
			Findings: allFindings,
			Final:    fallbackFinal,
			Models:   r.cfg.Models,
		}, nil
	}

	finals, err := domain.ParseFinalFindings(sumResp)
	if err != nil {
		finals = make([]domain.FinalFinding, 0, len(groups))
		for _, g := range groups {
			finals = append(finals, domain.FinalFinding{
				Rule:       g.Rule,
				Category:   g.Category,
				Severity:   g.Severity,
				File:       g.File,
				Line:       g.Line,
				Message:    g.Message,
				Suggestion: g.Suggestion,
				Confidence: g.Confidence,
				Keep:       true,
				Agents:     g.Agents,
				Count:      g.Count,
			})
		}
	}

	dismissed := 0
	for _, f := range finals {
		if !f.Keep {
			dismissed++
		}
	}

	return &domain.ReviewRun{
		Diff:      r.cfg.Diff,
		Findings:  allFindings,
		Final:     finals,
		Dismissed: dismissed,
		Models:    r.cfg.Models,
	}, nil
}
