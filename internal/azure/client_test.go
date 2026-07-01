package azure

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientCreateTXTRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "oauth2") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "test-token",
				"expires_in":   3600,
			})
			return
		}

		if r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(RecordSetResp{
				ID:   "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/dnszones/zone/TXT/test",
				Name: "test",
				Properties: RecordSetProperties{
					TTL: 120,
				},
			})
			return
		}

		if r.Method == "PUT" {
			var payload RecordSetPayload
			json.NewDecoder(r.Body).Decode(&payload)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(RecordSetResp{
				ID:   "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/dnszones/zone/TXT/test",
				Name: "test",
				Properties: RecordSetProperties{
					TTL:        payload.Properties.TTL,
					TxtRecords: payload.Properties.TxtRecords,
				},
			})
		}
	}))
	defer server.Close()

	client := &Client{
		token:    "test-token",
		tokenExp: time.Now().Add(1 * time.Hour),
		tenantID: "tenant",
		clientID: "client",
		baseURL:  server.URL,
	}

	resp, err := client.CreateTXTRecord(context.Background(), "sub", "rg", "zone", "test", "challenge-value", 120)
	if err != nil {
		t.Fatalf("CreateTXTRecord failed: %v", err)
	}

	if resp.Name != "test" {
		t.Errorf("expected name 'test', got '%s'", resp.Name)
	}
	if resp.Value != "challenge-value" {
		t.Errorf("expected value 'challenge-value', got '%s'", resp.Value)
	}
}

func TestClientListTXTRecords(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "oauth2") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "test-token",
				"expires_in":   3600,
			})
			return
		}

		if r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(RecordSetResp{
				Name: "test",
				Properties: RecordSetProperties{
					TTL: 120,
					TxtRecords: []TXTRecord{
						{Value: []string{"value1"}},
						{Value: []string{"value2"}},
					},
				},
			})
		}
	}))
	defer server.Close()

	client := &Client{
		token:    "test-token",
		tokenExp: time.Now().Add(1 * time.Hour),
		tenantID: "tenant",
		clientID: "client",
		baseURL:  server.URL,
	}

	records, err := client.ListTXTRecords(context.Background(), "sub", "rg", "zone", "test")
	if err != nil {
		t.Fatalf("ListTXTRecords failed: %v", err)
	}

	if len(records) != 2 {
		t.Errorf("expected 2 records, got %d", len(records))
	}
	if records[0] != "value1" || records[1] != "value2" {
		t.Errorf("unexpected record values: %v", records)
	}
}

func TestClientListTXTRecordsNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "oauth2") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "test-token",
				"expires_in":   3600,
			})
			return
		}

		if r.Method == "GET" {
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := &Client{
		token:    "test-token",
		tokenExp: time.Now().Add(1 * time.Hour),
		tenantID: "tenant",
		clientID: "client",
		baseURL:  server.URL,
	}

	records, err := client.ListTXTRecords(context.Background(), "sub", "rg", "zone", "test")
	if err != nil {
		t.Fatalf("ListTXTRecords failed: %v", err)
	}

	if len(records) != 0 {
		t.Errorf("expected 0 records for 404, got %d", len(records))
	}
}

func TestClientDeleteTXTRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "oauth2") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "test-token",
				"expires_in":   3600,
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if r.Method == "GET" {
			json.NewEncoder(w).Encode(RecordSetResp{
				Name: "test",
				Properties: RecordSetProperties{
					TTL: 120,
					TxtRecords: []TXTRecord{
						{Value: []string{"value"}},
					},
				},
			})
			return
		}

		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{})
			return
		}
	}))
	defer server.Close()

	client := &Client{
		token:    "test-token",
		tokenExp: time.Now().Add(1 * time.Hour),
		tenantID: "tenant",
		clientID: "client",
		baseURL:  server.URL,
	}

	err := client.DeleteTXTRecord(context.Background(), "sub", "rg", "zone", "test", "value")
	if err != nil {
		t.Fatalf("DeleteTXTRecord failed: %v", err)
	}
}

func TestClientDeleteTXTRecordNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "oauth2") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "test-token",
				"expires_in":   3600,
			})
			return
		}

		if r.Method == "GET" {
			// Record doesn't exist
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		if r.Method == "DELETE" {
			// This shouldn't be called for a non-existent record
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
	}))
	defer server.Close()

	client := &Client{
		token:    "test-token",
		tokenExp: time.Now().Add(1 * time.Hour),
		tenantID: "tenant",
		clientID: "client",
		baseURL:  server.URL,
	}

	// Should not error on 404
	err := client.DeleteTXTRecord(context.Background(), "sub", "rg", "zone", "test", "value")
	if err != nil {
		t.Fatalf("DeleteTXTRecord should succeed on 404, got error: %v", err)
	}
}
