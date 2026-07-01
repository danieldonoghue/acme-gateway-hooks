package excedo

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/danieldonoghue/acme-gateway-hooks/internal/env"
)

func TestZoneCandidates(t *testing.T) {
	got := ZoneCandidates("_acme-challenge.a.b.example.com")
	wantFirst := "a.b.example.com"
	wantLast := "example.com"

	if len(got) < 2 {
		t.Fatalf("expected multiple candidates, got %v", got)
	}
	if got[0] != wantFirst {
		t.Fatalf("first candidate = %q, want %q", got[0], wantFirst)
	}
	if got[len(got)-1] != wantLast {
		t.Fatalf("last candidate = %q, want %q", got[len(got)-1], wantLast)
	}
}

func TestRelativeRecordName(t *testing.T) {
	got := RelativeRecordName("_acme-challenge.api.example.com", "example.com")
	if got != "_acme-challenge.api" {
		t.Fatalf("RelativeRecordName() = %q", got)
	}
}

func TestResolveZoneAndRecordUsesConfiguredZone(t *testing.T) {
	zone, recName, err := resolveZoneAndRecord(context.Background(), nil, "", "_acme-challenge.dpd-test.example.com", "dpd-test.example.com")
	if err != nil {
		t.Fatalf("resolveZoneAndRecord() error = %v", err)
	}
	if zone != "dpd-test.example.com" {
		t.Fatalf("zone = %q, want dpd-test.example.com", zone)
	}
	if recName != "_acme-challenge" {
		t.Fatalf("record name = %q, want _acme-challenge", recName)
	}
}

func TestFindMatchingRecords(t *testing.T) {
	resp := &GetRecordsResponse{
		DNS: map[string]DomainBlock{
			"example.com": {
				Records: []DNSRecord{
					{RecordID: "1", Name: "_acme-challenge", Type: "TXT", Content: "good"},
					{RecordID: "2", Name: "_acme-challenge.example.com", Type: "TXT", Content: "good"},
					{RecordID: "3", Name: "_acme-challenge", Type: "TXT", Content: "other"},
				},
			},
		},
	}

	ids := findMatchingRecords(resp, "_acme-challenge.example.com", "_acme-challenge", "good")
	if len(ids) != 2 {
		t.Fatalf("matched records = %d, want 2", len(ids))
	}
}

func TestFindMatchingRecordsNormalizesQuotedTXTContent(t *testing.T) {
	resp := &GetRecordsResponse{
		DNS: map[string]DomainBlock{
			"example.com": {
				Records: []DNSRecord{
					{RecordID: "19742550", Name: "_acme-challenge.dpd-test", Type: "TXT", Content: `"challenge"`},
				},
			},
		},
	}

	ids := findMatchingRecords(resp, "_acme-challenge.dpd-test.example.com", "_acme-challenge.dpd-test", "challenge")
	if len(ids) != 1 {
		t.Fatalf("matched records = %d, want 1", len(ids))
	}
	if ids[0] != "19742550" {
		t.Fatalf("unexpected record id: %s", ids[0])
	}
}

func TestCleanupReturnsSuccessWhenLoginFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/authenticate/login/token" {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("temporary"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "token")
	client.maxRetries = 0
	client.backoff = 1 * time.Millisecond

	cfg := Config{CommonConfig: env.CommonConfig{Domain: "example.com", Validation: "txt", FQDN: "_acme-challenge.example.com"}}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	if err := Cleanup(context.Background(), logger, client, cfg); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
}

func TestCleanupReturnsSuccessWhenGetRecordsListingNonSuccess(t *testing.T) {
	var getRecordsCalls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/authenticate/login/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1000,"message":"ok","token":"session"}`))
		case r.URL.Path == "/dns/getrecords/session":
			call := getRecordsCalls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			if call == 1 {
				_, _ = w.Write([]byte(`{"code":1000,"message":"ok","dns":{"example.com":{"records":[]}}}`))
				return
			}
			_, _ = w.Write([]byte(`{"code":2004,"message":"zone not found","dns":{}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "token")
	client.maxRetries = 0
	client.backoff = 1 * time.Millisecond

	cfg := Config{CommonConfig: env.CommonConfig{Domain: "example.com", Validation: "txt", FQDN: "_acme-challenge.example.com"}}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	if err := Cleanup(context.Background(), logger, client, cfg); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
}

func TestCleanupContinuesWhenDeleteFails(t *testing.T) {
	var getRecordsCalls atomic.Int32
	var deleteCalls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/authenticate/login/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1000,"message":"ok","token":"session"}`))
		case r.URL.Path == "/dns/getrecords/session":
			call := getRecordsCalls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			if call == 1 {
				_, _ = w.Write([]byte(`{"code":1000,"message":"ok","dns":{"example.com":{"records":[]}}}`))
				return
			}
			_, _ = w.Write([]byte(`{"code":1000,"message":"ok","dns":{"example.com":{"records":[{"recordid":"1","name":"_acme-challenge","type":"TXT","content":"txt"},{"recordid":"2","name":"_acme-challenge","type":"TXT","content":"txt"}]}}}`))
		case r.URL.Path == "/dns/deleterecord/session":
			call := deleteCalls.Add(1)
			if call == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("temporary"))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1000,"message":"ok"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "token")
	client.maxRetries = 0
	client.backoff = 1 * time.Millisecond

	cfg := Config{CommonConfig: env.CommonConfig{Domain: "example.com", Validation: "txt", FQDN: "_acme-challenge.example.com"}}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	if err := Cleanup(context.Background(), logger, client, cfg); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if deleteCalls.Load() != 2 {
		t.Fatalf("delete calls = %d, want 2", deleteCalls.Load())
	}
}

func TestDeployWithConfiguredZoneBypassesZoneDiscovery(t *testing.T) {
	var getRecordsCalls atomic.Int32
	var addCalls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/authenticate/login/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1000,"message":"ok","token":"session"}`))
		case r.URL.Path == "/dns/getrecords/session":
			getRecordsCalls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":2004,"message":"zone not found","dns":{}}`))
		case r.URL.Path == "/dns/addrecord/session":
			addCalls.Add(1)
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm() error = %v", err)
			}
			if got := r.Form.Get("domainname"); got != "dpd-test.example.com" {
				t.Fatalf("domainname = %q, want dpd-test.example.com", got)
			}
			if got := r.Form.Get("name"); got != "_acme-challenge" {
				t.Fatalf("name = %q, want _acme-challenge", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1000,"message":"ok"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "token")
	client.maxRetries = 0
	client.backoff = 1 * time.Millisecond

	cfg := Config{
		CommonConfig: env.CommonConfig{Domain: "dpd-test.example.com", Validation: "txt", FQDN: "_acme-challenge.dpd-test.example.com"},
		DNSZone:      "dpd-test.example.com",
	}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	if err := Deploy(context.Background(), logger, client, cfg); err != nil {
		t.Fatalf("Deploy() error = %v", err)
	}
	if addCalls.Load() != 1 {
		t.Fatalf("add calls = %d, want 1", addCalls.Load())
	}
	if getRecordsCalls.Load() != 0 {
		t.Fatalf("getrecords calls = %d, want 0", getRecordsCalls.Load())
	}
}

func TestResolveZoneAndRecordWithoutConfiguredZoneFailsWhenNoZonesMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/dns/getrecords/session"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":2004,"message":"zone not found","dns":{}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "token")
	client.maxRetries = 0
	client.backoff = 1 * time.Millisecond

	_, _, err := resolveZoneAndRecord(context.Background(), client, "session", "_acme-challenge.dpd-test.example.com", "")
	if err == nil {
		t.Fatalf("expected resolveZoneAndRecord() error")
	}
}
