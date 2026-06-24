package excedo

import "testing"

func TestLoadConfigDefaultsAndFallbacks(t *testing.T) {
	t.Setenv("EXCEDO_API_TOKEN", "token")
	t.Setenv("ACME_GATEWAY_DOMAIN", "Example.COM")
	t.Setenv("ACME_GATEWAY_TOKEN", "txt")
	t.Setenv("CERTBOT_DOMAIN", "")
	t.Setenv("CERTBOT_VALIDATION", "")
	t.Setenv("ACME_GATEWAY_FQDN", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
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

func TestLoadConfigRequiredVars(t *testing.T) {
	t.Setenv("EXCEDO_API_TOKEN", "")
	t.Setenv("CERTBOT_DOMAIN", "")
	t.Setenv("ACME_GATEWAY_DOMAIN", "")
	t.Setenv("CERTBOT_VALIDATION", "")
	t.Setenv("ACME_GATEWAY_TOKEN", "")

	if _, err := LoadConfig(); err == nil {
		t.Fatalf("expected error for missing variables")
	}
}

func TestLoadConfigExplicitFQDNNormalized(t *testing.T) {
	t.Setenv("EXCEDO_API_TOKEN", "token")
	t.Setenv("CERTBOT_DOMAIN", "example.com")
	t.Setenv("CERTBOT_VALIDATION", "txt")
	t.Setenv("ACME_GATEWAY_FQDN", "_Acme-Challenge.Example.COM.")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.FQDN != "_acme-challenge.example.com" {
		t.Fatalf("unexpected fqdn normalization: %s", cfg.FQDN)
	}
}
