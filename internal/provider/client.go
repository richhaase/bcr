package provider

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	defaultMaxRetries = 3
	defaultBaseDelay  = 200 * time.Millisecond
	defaultMaxDelay   = 4 * time.Second
)

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

type statusError struct {
	StatusCode int
	Body       string
}

func (e *statusError) Error() string {
	return fmt.Sprintf("provider status %d: %s", e.StatusCode, e.Body)
}

var errEmptyChoices = errors.New("no completions returned from provider")

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
}

type ChatChoice struct {
	Message Message `json:"message"`
}

type ChatResponse struct {
	Choices []ChatChoice `json:"choices"`
	Error   *APIError    `json:"error,omitempty"`
}

type APIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    any    `json:"code"`
}

func NewClient(baseURL, apiKey string) *Client {
	if baseURL == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		MaxRetries: defaultMaxRetries,
		BaseDelay:  defaultBaseDelay,
		MaxDelay:   defaultMaxDelay,
	}
}

func (c *Client) Complete(ctx context.Context, model string, messages []Message, temp float64) (string, error) {
	maxRetries := c.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}

	var err error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		var out string
		out, err = c.completeOnce(ctx, model, messages, temp)
		if err == nil {
			return out, nil
		}
		if ctx.Err() != nil || attempt == maxRetries || !isRetryable(err) {
			return "", err
		}
		if err = c.wait(ctx, attempt+1); err != nil {
			return "", err
		}
	}
	return "", err
}

func (c *Client) completeOnce(ctx context.Context, model string, messages []Message, temp float64) (string, error) {
	reqBody := ChatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: temp,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	endpoint := c.BaseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	req.Header.Set("HTTP-Referer", "https://github.com/richhaase/bcr")
	req.Header.Set("X-Title", "bcr")

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", &statusError{StatusCode: res.StatusCode, Body: string(body)}
	}

	var chatRes ChatResponse
	if err := json.Unmarshal(body, &chatRes); err != nil {
		return "", fmt.Errorf("decode provider response: %w", err)
	}

	if chatRes.Error != nil && chatRes.Error.Message != "" {
		return "", fmt.Errorf("provider api error: %s", chatRes.Error.Message)
	}

	if len(chatRes.Choices) == 0 {
		return "", errEmptyChoices
	}

	return chatRes.Choices[0].Message.Content, nil
}

func (c *Client) wait(ctx context.Context, retry int) error {
	delay := c.BaseDelay << uint(retry-1)
	if c.MaxDelay > 0 && delay > c.MaxDelay {
		delay = c.MaxDelay
	}
	if delay > 0 {
		delay = jitter(delay)
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func jitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return d / 2
	}
	n := binary.BigEndian.Uint64(b[:])
	frac := float64(n) / float64(math.MaxUint64)
	return time.Duration(float64(d) * frac)
}

func isRetryable(err error) bool {
	var se *statusError
	if errors.As(err, &se) {
		return se.StatusCode == http.StatusTooManyRequests || se.StatusCode >= http.StatusInternalServerError
	}
	if errors.Is(err, errEmptyChoices) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}
