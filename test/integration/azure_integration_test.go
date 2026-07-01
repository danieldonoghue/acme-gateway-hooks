package integration

import (
	"strings"
	"testing"

	"github.com/danieldonoghue/acme-gateway-hooks/internal/azure"
	"github.com/danieldonoghue/acme-gateway-hooks/internal/env"
)

func TestAzureConfigValidation(t *testing.T) {
	cfg := azure.Config{
		CommonConfig: env.CommonConfig{
			Domain:     "app.test.example.com",
			Validation: "test-challenge-value",
			FQDN:       "_acme-challenge.app.test.example.com",
		},
		SubscriptionID: "test-sub",
		ResourceGroup:  "test-rg",
		ZoneName:       "test.example.com",
		TenantID:       "12345678-abcd-1234-abcd-1234567890ab",
		ClientID:       "test-client",
		ClientSecret:   "test-secret",
	}

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

	t.Run("config fields", func(t *testing.T) {
		if cfg.SubscriptionID != "test-sub" {
			t.Errorf("expected subscription ID test-sub, got %s", cfg.SubscriptionID)
		}
		if cfg.ResourceGroup != "test-rg" {
			t.Errorf("expected resource group test-rg, got %s", cfg.ResourceGroup)
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
