package biliutils

import (
	stderrors "errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	bilierrors "bilibili-ticket-golang/lib/models/errors"

	"github.com/imroc/req/v3"
)

func TestHTTP429RetryReplaysRequests(t *testing.T) {
	tests := []struct {
		name   string
		method string
		body   string
	}{
		{name: "get", method: http.MethodGet},
		{name: "post JSON body", method: http.MethodPost, body: `{"project_id":123}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempt := attempts.Add(1)
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("read request body: %v", err)
				}
				if string(body) != tt.body {
					t.Errorf("attempt %d body = %q, want %q", attempt, body, tt.body)
				}
				if attempt <= 2 {
					w.Header().Set("Retry-After", "0")
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(server.Close)

			client := req.NewClient()
			configureHTTP429Retry(client)
			request := client.R()
			if tt.body != "" {
				request.SetBodyString(tt.body)
			}
			resp, err := request.Send(tt.method, server.URL)
			if err != nil {
				t.Fatalf("Send() error = %v", err)
			}
			if resp.GetStatusCode() != http.StatusOK {
				t.Fatalf("Send() status = %d, want %d", resp.GetStatusCode(), http.StatusOK)
			}
			if got := attempts.Load(); got != 3 {
				t.Fatalf("attempts = %d, want 3", got)
			}
		})
	}
}

func TestHTTP429RetryStopsAfterLimit(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(server.Close)

	client := req.NewClient()
	configureHTTP429Retry(client)
	resp, err := client.R().Get(server.URL)
	var statusErr *bilierrors.BilibiliHTTPStatusError
	if !stderrors.As(err, &statusErr) {
		t.Fatalf("Get() error = %T %v, want *BilibiliHTTPStatusError", err, err)
	}
	if statusErr.HTTPStatusCode() != http.StatusTooManyRequests {
		t.Fatalf("HTTPStatusCode() = %d, want %d", statusErr.HTTPStatusCode(), http.StatusTooManyRequests)
	}
	if statusErr.Method != http.MethodGet || statusErr.URL != server.URL {
		t.Fatalf("request context = %q %q, want %q %q", statusErr.Method, statusErr.URL, http.MethodGet, server.URL)
	}
	if resp.GetStatusCode() != http.StatusTooManyRequests {
		t.Fatalf("Get() status = %d, want %d", resp.GetStatusCode(), http.StatusTooManyRequests)
	}
	if got, want := attempts.Load(), int32(http429MaxRetries+1); got != want {
		t.Fatalf("attempts = %d, want %d", got, want)
	}
}

func TestHTTP429RetryCanBeDisabledForOrderSubmission(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(server.Close)

	client := req.NewClient()
	configureHTTP429Retry(client)
	resp, err := client.R().SetRetryCount(0).Post(server.URL)
	if err != nil {
		t.Fatalf("Post() error = %v", err)
	}
	if resp.GetStatusCode() != http.StatusTooManyRequests {
		t.Fatalf("Post() status = %d, want %d", resp.GetStatusCode(), http.StatusTooManyRequests)
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("attempts = %d, want 1", got)
	}
}

func TestHTTP429RetryDoesNotRetryOtherStatuses(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	client := req.NewClient()
	configureHTTP429Retry(client)
	resp, err := client.R().Get(server.URL)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if resp.GetStatusCode() != http.StatusInternalServerError {
		t.Fatalf("Get() status = %d, want %d", resp.GetStatusCode(), http.StatusInternalServerError)
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("attempts = %d, want 1", got)
	}
}
