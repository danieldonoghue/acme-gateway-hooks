package excedo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestDecodeJSONStrictAllowsUnknownField(t *testing.T) {
	input := []byte(`{"code":1000,"message":"ok","token":"abc","extra":1}`)
	var out AuthResponse
	if err := decodeJSONStrict(input, &out); err != nil {
		t.Fatalf("expected unknown field to be ignored, got: %v", err)
	}
}

func TestDecodeJSONStrictAllowsKnownDescField(t *testing.T) {
	input := []byte(`{"code":1000,"desc":"ok","token":"abc"}`)
	var out AuthResponse
	if err := decodeJSONStrict(input, &out); err != nil {
		t.Fatalf("expected desc field to decode successfully, got: %v", err)
	}
	if out.Token != "abc" {
		t.Fatalf("unexpected token: %q", out.Token)
	}
}

func TestDecodeJSONStrictAllowsKnownParametersField(t *testing.T) {
	input := []byte(`{"code":1000,"parameters":{"foo":"bar"},"token":"abc"}`)
	var out AuthResponse
	if err := decodeJSONStrict(input, &out); err != nil {
		t.Fatalf("expected parameters field to decode successfully, got: %v", err)
	}
	if out.Parameters["foo"] != "bar" {
		t.Fatalf("unexpected parameters content: %#v", out.Parameters)
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

func TestLoginAllowsUnknownJSONFieldFromEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1000,"message":"ok","token":"session","extra":1}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "token")
	token, err := client.Login(context.Background())
	if err != nil {
		t.Fatalf("expected success with unknown field, got: %v", err)
	}
	if token != "session" {
		t.Fatalf("unexpected token: %q", token)
	}
}

func TestLoginAcceptsSpecShapedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1000,"desc":"Command completed successfully","parameters":{"token":"session-from-parameters","accID":"0","usrID":"0"},"runtime":0.0709}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "token")
	token, err := client.Login(context.Background())
	if err != nil {
		t.Fatalf("expected success with spec-shaped auth response, got: %v", err)
	}
	if token != "session-from-parameters" {
		t.Fatalf("unexpected token: %q", token)
	}
}

func TestGetRecordsAllowsUnknownJSONFieldFromEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1000,"message":"ok","dns":{},"extra":1}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "token")
	resp, err := client.GetRecords(context.Background(), "session", "example.com")
	if err != nil {
		t.Fatalf("expected success with unknown field, got: %v", err)
	}
	if resp.Code != 1000 {
		t.Fatalf("unexpected code: %d", resp.Code)
	}
}

func TestGetRecordsAcceptsDNSArrayFromEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1000,"message":"ok","dns":[{"domainname":"aurorateleq.com","records":[{"recordid":19742549,"name":"_acme-challenge.dpd-test","type":"TXT","content":"challenge"}]}]}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "token")
	resp, err := client.GetRecords(context.Background(), "session", "aurorateleq.com")
	if err != nil {
		t.Fatalf("expected success with dns array payload, got: %v", err)
	}
	if resp.Code != 1000 {
		t.Fatalf("unexpected code: %d", resp.Code)
	}
	block, ok := resp.DNS["aurorateleq.com"]
	if !ok {
		t.Fatalf("expected aurorateleq.com block in parsed dns payload")
	}
	if len(block.Records) != 1 {
		t.Fatalf("unexpected record count: %d", len(block.Records))
	}
	if block.Records[0].RecordID != "19742549" {
		t.Fatalf("unexpected record id: %s", block.Records[0].RecordID)
	}
}

func TestGetRecordsAcceptsDNSObjectWithRecordArrayValues(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1000,"message":"ok","dns":{"aurorateleq.com":[{"recordid":19742549,"name":"_acme-challenge.dpd-test","type":"TXT","content":"challenge"}]}}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "token")
	resp, err := client.GetRecords(context.Background(), "session", "aurorateleq.com")
	if err != nil {
		t.Fatalf("expected success with dns object/array payload, got: %v", err)
	}
	block, ok := resp.DNS["aurorateleq.com"]
	if !ok {
		t.Fatalf("expected aurorateleq.com block in parsed dns payload")
	}
	if len(block.Records) != 1 {
		t.Fatalf("unexpected record count: %d", len(block.Records))
	}
	if block.Records[0].RecordID != "19742549" {
		t.Fatalf("unexpected record id: %s", block.Records[0].RecordID)
	}
}

func TestGetRecordsAcceptsDNSObjectWithRecordsMapValues(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1000,"message":"ok","dns":{"aurorateleq.com":{"records":{"19742550":{"name":"_acme-challenge.dpd-test","type":"TXT","content":"challenge"}}}}}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "token")
	resp, err := client.GetRecords(context.Background(), "session", "aurorateleq.com")
	if err != nil {
		t.Fatalf("expected success with dns object/records-map payload, got: %v", err)
	}
	block, ok := resp.DNS["aurorateleq.com"]
	if !ok {
		t.Fatalf("expected aurorateleq.com block in parsed dns payload")
	}
	if len(block.Records) != 1 {
		t.Fatalf("unexpected record count: %d", len(block.Records))
	}
	if block.Records[0].RecordID != "19742550" {
		t.Fatalf("unexpected record id: %s", block.Records[0].RecordID)
	}
}
