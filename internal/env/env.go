package env

import (
	"fmt"
	"os"
	"strings"
)

const defaultExcedoAPIURL = "https://api.domainname.systems"

type HookConfig struct {
	APIToken   string
	APIURL     string
	Domain     string
	Validation string
	FQDN       string
}

func Load() (HookConfig, error) {
	cfg := HookConfig{
		APIToken: strings.TrimSpace(os.Getenv("EXCEDO_API_TOKEN")),
		APIURL:   strings.TrimSpace(os.Getenv("EXCEDO_API_URL")),
		Domain:   firstNonEmpty("CERTBOT_DOMAIN", "ACME_GATEWAY_DOMAIN"),
		Validation: firstNonEmpty(
			"CERTBOT_VALIDATION",
			"ACME_GATEWAY_TOKEN",
		),
		FQDN: strings.TrimSpace(os.Getenv("ACME_GATEWAY_FQDN")),
	}

	if cfg.APIURL == "" {
		cfg.APIURL = defaultExcedoAPIURL
	}

	if cfg.APIToken == "" {
		return HookConfig{}, fmt.Errorf("missing required environment variable EXCEDO_API_TOKEN")
	}
	if cfg.Domain == "" {
		return HookConfig{}, fmt.Errorf("missing required domain input: CERTBOT_DOMAIN or ACME_GATEWAY_DOMAIN")
	}
	if cfg.Validation == "" {
		return HookConfig{}, fmt.Errorf("missing required TXT value input: CERTBOT_VALIDATION or ACME_GATEWAY_TOKEN")
	}

	cfg.Domain = normalizeFQDN(cfg.Domain)
	cfg.FQDN = normalizeFQDN(cfg.FQDN)
	if cfg.FQDN == "" {
		cfg.FQDN = "_acme-challenge." + cfg.Domain
	}

	return cfg, nil
}

func firstNonEmpty(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func normalizeFQDN(v string) string {
	return strings.TrimSuffix(strings.TrimSpace(strings.ToLower(v)), ".")
}
