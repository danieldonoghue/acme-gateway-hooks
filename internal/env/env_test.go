package env

import (
	"os"
	"testing"
)

func TestLoadDefaultsAndFallbacks(t *testing.T) {
	t.Setenv("EXCEDO_API_TOKEN", "token")
	t.Setenv("ACME_GATEWAY_DOMAIN", "Example.COM")
	t.Setenv("ACME_GATEWAY_TOKEN", "txt")
	t.Setenv("CERTBOT_DOMAIN", "")
	t.Setenv("CERTBOT_VALIDATION", "")
	t.Setenv("ACME_GATEWAY_FQDN", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.APIURL != "https://api.domainname.systems" {
		t.Fatalf("unexpected default API URL: %s", cfg.APIURL)
	}
	if cfg.Domain != "example.com" {
		t.Fatalf("unexpected domain: %s", cfg.Domain)
	}
	if cfg.FQDN != "_acme-challenge.example.com" {
		t.Fatalf("unexpected fqdn: %s", cfg.FQDN)
	}
}

func TestLoadRequiredVars(t *testing.T) {
	keys := []string{
		"EXCEDO_API_TOKEN",
		"CERTBOT_DOMAIN",
		"ACME_GATEWAY_DOMAIN",
		"CERTBOT_VALIDATION",
		"ACME_GATEWAY_TOKEN",
	}
	for _, key := range keys {
		_ = os.Unsetenv(key)
	}

	if _, err := Load(); err == nil {
		t.Fatalf("expected error for missing variables")
	}
}
