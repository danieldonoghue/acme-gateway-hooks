package env

import (
	"os"
	"testing"
)

func TestLoadExcedoDefaultsAndFallbacks(t *testing.T) {
	t.Setenv("EXCEDO_API_TOKEN", "token")
	t.Setenv("ACME_GATEWAY_DOMAIN", "Example.COM")
	t.Setenv("ACME_GATEWAY_TOKEN", "txt")
	t.Setenv("CERTBOT_DOMAIN", "")
	t.Setenv("CERTBOT_VALIDATION", "")
	t.Setenv("ACME_GATEWAY_FQDN", "")

	cfg, err := LoadExcedo()
	if err != nil {
		t.Fatalf("LoadExcedo() error = %v", err)
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

func TestLoadExcedoRequiredVars(t *testing.T) {
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

	if _, err := LoadExcedo(); err == nil {
		t.Fatalf("expected error for missing variables")
	}
}

func TestLoadBindDefaultsAndZoneFallback(t *testing.T) {
	t.Setenv("CERTBOT_DOMAIN", "test.pebble-test.local")
	t.Setenv("CERTBOT_VALIDATION", "txt-value")
	t.Setenv("ACME_GATEWAY_FQDN", "_acme-challenge.test.pebble-test.local")
	t.Setenv("BIND_DNS_SERVER", "127.0.0.1")

	cfg, err := LoadBind()
	if err != nil {
		t.Fatalf("LoadBind() error = %v", err)
	}
	if cfg.DNSServer != "127.0.0.1:53" {
		t.Fatalf("unexpected default DNS server: %s", cfg.DNSServer)
	}
	if cfg.DNSZone != "pebble-test.local" {
		t.Fatalf("unexpected inferred zone: %s", cfg.DNSZone)
	}
	if cfg.TTL != 60 {
		t.Fatalf("unexpected default TTL: %d", cfg.TTL)
	}
}

func TestLoadBindTSIGPairValidation(t *testing.T) {
	t.Setenv("CERTBOT_DOMAIN", "example.com")
	t.Setenv("CERTBOT_VALIDATION", "txt")
	t.Setenv("BIND_DNS_ZONE", "example.com")
	t.Setenv("BIND_DNS_TSIG_KEY_NAME", "key-name")
	t.Setenv("BIND_DNS_TSIG_SECRET", "")

	if _, err := LoadBind(); err == nil {
		t.Fatal("expected error when only one TSIG field is set")
	}
}
