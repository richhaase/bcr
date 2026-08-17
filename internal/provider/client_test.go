package provider

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"testing"
)

func TestIsRetryable(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"status 429", &statusError{StatusCode: 429, Body: "rate limited"}, true},
		{"status 500", &statusError{StatusCode: 500, Body: "boom"}, true},
		{"status 503", &statusError{StatusCode: 503, Body: "unavailable"}, true},
		{"status 400", &statusError{StatusCode: 400, Body: "bad request"}, false},
		{"status 401", &statusError{StatusCode: 401, Body: "unauthorized"}, false},
		{"status 404", &statusError{StatusCode: 404, Body: "invalid model"}, false},
		{"empty choices", errEmptyChoices, true},
		{"network error", &url.Error{Err: &netError{}}, true},
		{"plain error", errors.New("decode failed"), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRetryable(tc.err); got != tc.want {
				t.Errorf("isRetryable(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

type netError struct{}

func (*netError) Error() string   { return "dial timeout" }
func (*netError) Timeout() bool   { return true }
func (*netError) Temporary() bool { return true }

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestClient(attempts int, status int, body string) *Client {
	c := NewClient("http://test", "key")
	c.MaxRetries = 3
	c.BaseDelay = 0
	c.MaxDelay = 0
	count := 0
	c.HTTPClient.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if count < attempts {
			count++
			return &http.Response{StatusCode: status, Body: io.NopCloser(bytes.NewReader([]byte(body))), Header: http.Header{}}, nil
		}
		count++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))), Header: http.Header{}}, nil
	})
	return c
}

func TestCompleteRetriesTransient(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		attempts int
	}{
		{"429 then success", 429, 2},
		{"500 then success", 500, 3},
		{"503 then success", 503, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(tc.attempts, tc.status, "transient")
			out, err := c.Complete(context.Background(), "m", nil, 0)
			if err != nil {
				t.Fatalf("Complete error: %v", err)
			}
			if out != "ok" {
				t.Errorf("expected out ok, got %q", out)
			}
		})
	}
}

func TestCompletePermanentNotRetried(t *testing.T) {
	c := NewClient("http://test", "key")
	c.MaxRetries = 3
	c.BaseDelay = 0
	c.MaxDelay = 0
	calls := 0
	c.HTTPClient.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 400, Body: io.NopCloser(bytes.NewReader([]byte("invalid model"))), Header: http.Header{}}, nil
	})
	out, err := c.Complete(context.Background(), "m", nil, 0)
	if err == nil {
		t.Fatal("expected error for permanent status 400")
	}
	if out != "" {
		t.Errorf("expected empty out, got %q", out)
	}
	var se *statusError
	if !errors.As(err, &se) || se.StatusCode != 400 {
		t.Errorf("expected statusError 400, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call (no retry), got %d", calls)
	}
}

func TestCompleteExhaustsRetries(t *testing.T) {
	c := NewClient("http://test", "key")
	c.MaxRetries = 2
	c.BaseDelay = 0
	c.MaxDelay = 0
	calls := 0
	c.HTTPClient.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 500, Body: io.NopCloser(bytes.NewReader([]byte("boom"))), Header: http.Header{}}, nil
	})
	out, err := c.Complete(context.Background(), "m", nil, 0)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if out != "" {
		t.Errorf("expected empty out, got %q", out)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls (1 + 2 retries), got %d", calls)
	}
}

func TestCompleteRetriesEmptyOutput(t *testing.T) {
	c := NewClient("http://test", "key")
	c.MaxRetries = 1
	c.BaseDelay = 0
	c.MaxDelay = 0
	calls := 0
	c.HTTPClient.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader([]byte(`{"choices":[]}`))), Header: http.Header{}}, nil
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))), Header: http.Header{}}, nil
	})
	out, err := c.Complete(context.Background(), "m", nil, 0)
	if err != nil {
		t.Fatalf("Complete error: %v", err)
	}
	if out != "ok" {
		t.Errorf("expected ok, got %q", out)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}
