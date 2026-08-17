package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

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
	}
}

func (c *Client) Complete(ctx context.Context, model string, messages []Message, temp float64) (string, error) {
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
		return "", fmt.Errorf("provider status %d: %s", res.StatusCode, string(body))
	}

	var chatRes ChatResponse
	if err := json.Unmarshal(body, &chatRes); err != nil {
		return "", fmt.Errorf("decode provider response: %w", err)
	}

	if chatRes.Error != nil && chatRes.Error.Message != "" {
		return "", fmt.Errorf("provider api error: %s", chatRes.Error.Message)
	}

	if len(chatRes.Choices) == 0 {
		return "", errors.New("no completions returned from provider")
	}

	return chatRes.Choices[0].Message.Content, nil
}
