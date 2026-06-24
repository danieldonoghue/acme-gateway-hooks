package excedo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDecodeJSONStrictRejectsUnknownField(t *testing.T) {
	input := []byte(`{"code":1000,"message":"ok","token":"abc","extra":1}`)
	var out AuthResponse
	if err := decodeJSONStrict(input, &out); err == nil {
		t.Fatalf("expected unknown field error")
	}
}

func TestDecodeJSONStrictRejectsTrailingContent(t *testing.T) {
	input := []byte(`{"code":1000,"message":"ok","token":"abc"} {}`)
	var out AuthResponse
	if err := decodeJSONStrict(input, &out); err == nil {
		t.Fatalf("expected trailing content error")
	}
}

func TestLoginRetriesOnTransientHTTPStatus(t *testing.T) {
	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.Body.Close()
		n := attempts.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("temporary error"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1000,"message":"ok","token":"session"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "token")
	client.backoff = 1 * time.Millisecond
	client.maxRetries = 3

	token, err := client.Login(context.Background())
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if token != "session" {
		t.Fatalf("Login() token = %q", token)
	}
	if attempts.Load() != 3 {
		t.Fatalf("attempt count = %d, want 3", attempts.Load())
	}
}

func TestLoginDoesNotRetryOnBadRequest(t *testing.T) {
	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.Body.Close()
		attempts.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad request"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "token")
	client.backoff = 1 * time.Millisecond
	client.maxRetries = 3

	_, err := client.Login(context.Background())
	if err == nil {
		t.Fatal("expected Login() to fail")
	}
	if attempts.Load() != 1 {
		t.Fatalf("attempt count = %d, want 1", attempts.Load())
	}
}

func TestLoginRejectsUnknownJSONFieldFromEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1000,"message":"ok","token":"session","extra":1}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "token")
	_, err := client.Login(context.Background())
	if err == nil {
		t.Fatal("expected strict decode error")
	}
	if !strings.Contains(err.Error(), "decode auth response") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetRecordsRejectsUnknownJSONFieldFromEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1000,"message":"ok","dns":{},"extra":1}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "token")
	_, err := client.GetRecords(context.Background(), "session", "example.com")
	if err == nil {
		t.Fatal("expected strict decode error")
	}
	if !strings.Contains(err.Error(), "decode getrecords response") {
		t.Fatalf("unexpected error: %v", err)
	}
}
