package azure

import (
	"testing"

	"github.com/danieldonoghue/acme-gateway-hooks/internal/env"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with secret",
			cfg: Config{
				CommonConfig: env.CommonConfig{
					Domain:     "test.example.com",
					Validation: "challenge-value",
				},
				SubscriptionID: "sub123",
				ResourceGroup:  "rg1",
				ZoneName:       "example.com",
				TenantID:       "tenant123",
				ClientID:       "client123",
				ClientSecret:   "secret123",
			},
			wantErr: false,
		},
		{
			name: "valid config with certificate",
			cfg: Config{
				CommonConfig: env.CommonConfig{
					Domain:     "test.example.com",
					Validation: "challenge-value",
				},
				SubscriptionID: "sub123",
				ResourceGroup:  "rg1",
				ZoneName:       "example.com",
				TenantID:       "tenant123",
				ClientID:       "client123",
				ClientCertPath: "/path/to/cert.pfx",
			},
			wantErr: false,
		},
		{
			name: "missing subscription ID",
			cfg: Config{
				CommonConfig: env.CommonConfig{
					Domain:     "test.example.com",
					Validation: "challenge-value",
				},
				ResourceGroup: "rg1",
				ZoneName:      "example.com",
				TenantID:      "tenant123",
				ClientID:      "client123",
				ClientSecret:  "secret123",
			},
			wantErr: true,
			errMsg:  "AZURE_SUBSCRIPTION_ID",
		},
		{
			name: "missing resource group",
			cfg: Config{
				CommonConfig: env.CommonConfig{
					Domain:     "test.example.com",
					Validation: "challenge-value",
				},
				SubscriptionID: "sub123",
				ZoneName:       "example.com",
				TenantID:       "tenant123",
				ClientID:       "client123",
				ClientSecret:   "secret123",
			},
			wantErr: true,
			errMsg:  "AZURE_RESOURCE_GROUP",
		},
		{
			name: "missing zone name",
			cfg: Config{
				CommonConfig: env.CommonConfig{
					Domain:     "test.example.com",
					Validation: "challenge-value",
				},
				SubscriptionID: "sub123",
				ResourceGroup:  "rg1",
				TenantID:       "tenant123",
				ClientID:       "client123",
				ClientSecret:   "secret123",
			},
			wantErr: true,
			errMsg:  "AZURE_ZONE_NAME",
		},
		{
			name: "missing tenant ID",
			cfg: Config{
				CommonConfig: env.CommonConfig{
					Domain:     "test.example.com",
					Validation: "challenge-value",
				},
				SubscriptionID: "sub123",
				ResourceGroup:  "rg1",
				ZoneName:       "example.com",
				ClientID:       "client123",
				ClientSecret:   "secret123",
			},
			wantErr: true,
			errMsg:  "AZURE_TENANT_ID",
		},
		{
			name: "missing client ID",
			cfg: Config{
				CommonConfig: env.CommonConfig{
					Domain:     "test.example.com",
					Validation: "challenge-value",
				},
				SubscriptionID: "sub123",
				ResourceGroup:  "rg1",
				ZoneName:       "example.com",
				TenantID:       "tenant123",
				ClientSecret:   "secret123",
			},
			wantErr: true,
			errMsg:  "AZURE_CLIENT_ID",
		},
		{
			name: "both secret and certificate set",
			cfg: Config{
				CommonConfig: env.CommonConfig{
					Domain:     "test.example.com",
					Validation: "challenge-value",
				},
				SubscriptionID: "sub123",
				ResourceGroup:  "rg1",
				ZoneName:       "example.com",
				TenantID:       "tenant123",
				ClientID:       "client123",
				ClientSecret:   "secret123",
				ClientCertPath: "/path/to/cert",
			},
			wantErr: true,
			errMsg:  "mutually exclusive",
		},
		{
			name: "neither secret nor certificate set",
			cfg: Config{
				CommonConfig: env.CommonConfig{
					Domain:     "test.example.com",
					Validation: "challenge-value",
				},
				SubscriptionID: "sub123",
				ResourceGroup:  "rg1",
				ZoneName:       "example.com",
				TenantID:       "tenant123",
				ClientID:       "client123",
			},
			wantErr: true,
			errMsg:  "exactly one",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil {
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, expected to contain %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestRelativeRecordName(t *testing.T) {
	tests := []struct {
		fqdn     string
		zone     string
		expected string
	}{
		{
			fqdn:     "_acme-challenge.app.example.com",
			zone:     "example.com",
			expected: "_acme-challenge.app",
		},
		{
			fqdn:     "app.example.com",
			zone:     "example.com",
			expected: "app",
		},
		{
			fqdn:     "example.com",
			zone:     "example.com",
			expected: "example.com",
		},
		{
			fqdn:     "_acme-challenge.subdomain.app.example.com",
			zone:     "example.com",
			expected: "_acme-challenge.subdomain.app",
		},
		{
			fqdn:     "example.com.",
			zone:     "example.com",
			expected: "example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.fqdn, func(t *testing.T) {
			got := RelativeRecordName(tt.fqdn, tt.zone)
			if got != tt.expected {
				t.Errorf("RelativeRecordName(%q, %q) = %q, expected %q", tt.fqdn, tt.zone, got, tt.expected)
			}
		})
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
