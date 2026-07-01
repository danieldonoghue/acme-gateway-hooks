package integration

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/danieldonoghue/acme-gateway-hooks/internal/azure"
	"github.com/danieldonoghue/acme-gateway-hooks/internal/env"
)

type fakeAzureDNS struct {
	mu      sync.Mutex
	records map[string][]string // relative record name → TXT values
}

func newFakeAzureDNS() *fakeAzureDNS {
	return &fakeAzureDNS{records: make(map[string][]string)}
}

func (f *fakeAzureDNS) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "oauth2") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "fake-token",
				"expires_in":   3600,
			})
			return
		}

		parts := strings.Split(r.URL.Path, "/TXT/")
		if len(parts) < 2 {
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}
		recordName := strings.Split(parts[1], "?")[0]

		f.mu.Lock()
		defer f.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case "GET":
			values, exists := f.records[recordName]
			if !exists {
				http.Error(w, `{"error":{"code":"NotFound"}}`, http.StatusNotFound)
				return
			}
			var txtRecords []map[string]interface{}
			for _, v := range values {
				txtRecords = append(txtRecords, map[string]interface{}{"value": []string{v}})
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"name": recordName,
				"properties": map[string]interface{}{
					"TTL":        120,
					"TXTRecords": txtRecords,
				},
			})

		case "PUT":
			var payload struct {
				Properties struct {
					TTL        int32 `json:"TTL"`
					TxtRecords []struct {
						Value []string `json:"value"`
					} `json:"TXTRecords"`
				} `json:"properties"`
			}
			json.NewDecoder(r.Body).Decode(&payload)

			var values []string
			for _, rec := range payload.Properties.TxtRecords {
				values = append(values, strings.Join(rec.Value, ""))
			}
			f.records[recordName] = values

			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":   "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/dnszones/zone/TXT/" + recordName,
				"name": recordName,
				"properties": map[string]interface{}{
					"TTL":        payload.Properties.TTL,
					"TXTRecords": payload.Properties.TxtRecords,
				},
			})

		case "DELETE":
			delete(f.records, recordName)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{})
		}
	})
}

func (f *fakeAzureDNS) hasTXT(name, value string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, v := range f.records[name] {
		if v == value {
			return true
		}
	}
	return false
}

func (f *fakeAzureDNS) recordCount(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.records[name])
}

func TestAzureDeployAndCleanupIdempotent(t *testing.T) {
	fake := newFakeAzureDNS()
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()

	t.Setenv("AZURE_SUBSCRIPTION_ID", "test-sub")
	t.Setenv("AZURE_RESOURCE_GROUP", "test-rg")
	t.Setenv("AZURE_ZONE_NAME", "test.example.com")
	t.Setenv("AZURE_TENANT_ID", "test-tenant")
	t.Setenv("AZURE_CLIENT_ID", "test-client")
	t.Setenv("AZURE_CLIENT_SECRET", "test-secret")
	t.Setenv("AZURE_BASE_URL", srv.URL)
	t.Setenv("CERTBOT_DOMAIN", "app.test.example.com")
	t.Setenv("CERTBOT_VALIDATION", "test-challenge-token")
	t.Setenv("ACME_GATEWAY_FQDN", "_acme-challenge.app.test.example.com")

	cfg, err := azure.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	ctx := context.Background()
	client, err := azure.NewClient(ctx, cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	// Deploy
	err = azure.Deploy(ctx, logger, client, cfg)
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	if !fake.hasTXT("_acme-challenge.app", "test-challenge-token") {
		t.Fatal("expected TXT record to exist after deploy")
	}

	// Deploy again (idempotent — should not duplicate)
	err = azure.Deploy(ctx, logger, client, cfg)
	if err != nil {
		t.Fatalf("second Deploy failed: %v", err)
	}
	if fake.recordCount("_acme-challenge.app") != 1 {
		t.Fatalf("expected 1 record after idempotent deploy, got %d", fake.recordCount("_acme-challenge.app"))
	}

	// Cleanup
	err = azure.Cleanup(ctx, logger, client, cfg)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	if fake.hasTXT("_acme-challenge.app", "test-challenge-token") {
		t.Fatal("expected TXT record to be removed after cleanup")
	}

	// Cleanup again (idempotent)
	err = azure.Cleanup(ctx, logger, client, cfg)
	if err != nil {
		t.Fatalf("second Cleanup failed: %v", err)
	}
}

func TestAzureConfigValidation(t *testing.T) {
	t.Run("config validation", func(t *testing.T) {
		invalidCfg := azure.Config{
			CommonConfig: env.CommonConfig{
				Domain:     "test.example.com",
				Validation: "test-value",
			},
			SubscriptionID: "sub",
			ResourceGroup:  "rg",
			ZoneName:       "zone",
			TenantID:       "tenant",
			ClientID:       "client",
			ClientSecret:   "secret",
			ClientCertPath: "/path/to/cert",
		}
		err := invalidCfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
			t.Errorf("expected mutual exclusivity error, got: %v", err)
		}

		invalidCfg2 := azure.Config{
			CommonConfig: env.CommonConfig{
				Domain:     "test.example.com",
				Validation: "test-value",
			},
			SubscriptionID: "sub",
			ResourceGroup:  "rg",
			ZoneName:       "zone",
			TenantID:       "tenant",
			ClientID:       "client",
		}
		err = invalidCfg2.Validate()
		if err == nil || !strings.Contains(err.Error(), "exactly one") {
			t.Errorf("expected 'exactly one' error, got: %v", err)
		}
	})

	t.Run("relative record name", func(t *testing.T) {
		name := azure.RelativeRecordName("_acme-challenge.app.test.example.com", "test.example.com")
		if name != "_acme-challenge.app" {
			t.Errorf("expected '_acme-challenge.app', got '%s'", name)
		}

		name = azure.RelativeRecordName("test.example.com", "test.example.com")
		if name != "test.example.com" {
			t.Errorf("expected 'test.example.com', got '%s'", name)
		}
	})
}

func TestAzureDNSConfigMutualExclusivity(t *testing.T) {
	tests := []struct {
		name    string
		cfg     azure.Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid with secret",
			cfg: azure.Config{
				CommonConfig:   env.CommonConfig{Domain: "test.com", Validation: "val"},
				SubscriptionID: "sub", ResourceGroup: "rg", ZoneName: "zone",
				TenantID: "tenant", ClientID: "client", ClientSecret: "secret",
			},
			wantErr: false,
		},
		{
			name: "valid with certificate",
			cfg: azure.Config{
				CommonConfig:   env.CommonConfig{Domain: "test.com", Validation: "val"},
				SubscriptionID: "sub", ResourceGroup: "rg", ZoneName: "zone",
				TenantID: "tenant", ClientID: "client", ClientCertPath: "/path",
			},
			wantErr: false,
		},
		{
			name: "invalid: both secret and cert",
			cfg: azure.Config{
				CommonConfig:   env.CommonConfig{Domain: "test.com", Validation: "val"},
				SubscriptionID: "sub", ResourceGroup: "rg", ZoneName: "zone",
				TenantID: "tenant", ClientID: "client", ClientSecret: "secret", ClientCertPath: "/path",
			},
			wantErr: true,
			errMsg:  "mutually exclusive",
		},
		{
			name: "invalid: neither secret nor cert",
			cfg: azure.Config{
				CommonConfig:   env.CommonConfig{Domain: "test.com", Validation: "val"},
				SubscriptionID: "sub", ResourceGroup: "rg", ZoneName: "zone",
				TenantID: "tenant", ClientID: "client",
			},
			wantErr: true,
			errMsg:  "exactly one",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("validate() error = %v, expected substring %q", err, tt.errMsg)
			}
		})
	}
}
