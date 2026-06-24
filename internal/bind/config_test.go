package bind

import (
	"strings"
	"testing"
)

func TestLoadConfigDefaultsAndZoneFallback(t *testing.T) {
	t.Setenv("CERTBOT_DOMAIN", "test.pebble-test.local")
	t.Setenv("CERTBOT_VALIDATION", "txt-value")
	t.Setenv("ACME_GATEWAY_FQDN", "_acme-challenge.test.pebble-test.local")
	t.Setenv("BIND_DNS_SERVER", "127.0.0.1")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
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

func TestLoadConfigTSIGPairValidation(t *testing.T) {
	t.Setenv("CERTBOT_DOMAIN", "example.com")
	t.Setenv("CERTBOT_VALIDATION", "txt")
	t.Setenv("BIND_DNS_ZONE", "example.com")
	t.Setenv("BIND_DNS_TSIG_KEY_NAME", "key-name")
	t.Setenv("BIND_DNS_TSIG_SECRET", "")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected error when only one TSIG field is set")
	}
}

func TestLoadConfigInvalidTTL(t *testing.T) {
	t.Setenv("CERTBOT_DOMAIN", "example.com")
	t.Setenv("CERTBOT_VALIDATION", "txt")
	t.Setenv("BIND_DNS_ZONE", "example.com")
	t.Setenv("BIND_DNS_TTL", "invalid")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected TTL parse error")
	}
	if !strings.Contains(err.Error(), "invalid unsigned integer value") {
		t.Fatalf("unexpected error: %v", err)
	}
}
