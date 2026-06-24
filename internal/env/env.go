package env

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

const defaultExcedoAPIURL = "https://api.domainname.systems"
const defaultBindDNSServer = "127.0.0.1:53"
const defaultBindDNSTTL uint32 = 60
const defaultTSIGAlgorithm = "hmac-sha256."

type CommonConfig struct {
	Domain     string
	Validation string
	FQDN       string
}

type ExcedoConfig struct {
	CommonConfig
	APIToken string
	APIURL   string
}

type BindConfig struct {
	CommonConfig
	DNSServer     string
	DNSZone       string
	TTL           uint32
	TSIGKeyName   string
	TSIGSecret    string
	TSIGAlgorithm string
}

func LoadCommon() (CommonConfig, error) {
	cfg := CommonConfig{
		Domain: firstNonEmpty("CERTBOT_DOMAIN", "ACME_GATEWAY_DOMAIN"),
		Validation: firstNonEmpty(
			"CERTBOT_VALIDATION",
			"ACME_GATEWAY_TOKEN",
		),
		FQDN: strings.TrimSpace(os.Getenv("ACME_GATEWAY_FQDN")),
	}

	if cfg.Domain == "" {
		return CommonConfig{}, fmt.Errorf("missing required domain input: CERTBOT_DOMAIN or ACME_GATEWAY_DOMAIN")
	}
	if cfg.Validation == "" {
		return CommonConfig{}, fmt.Errorf("missing required TXT value input: CERTBOT_VALIDATION or ACME_GATEWAY_TOKEN")
	}

	cfg.Domain = normalizeFQDN(cfg.Domain)
	cfg.FQDN = normalizeFQDN(cfg.FQDN)
	if cfg.FQDN == "" {
		cfg.FQDN = "_acme-challenge." + cfg.Domain
	}

	return cfg, nil
}

func LoadExcedo() (ExcedoConfig, error) {
	common, err := LoadCommon()
	if err != nil {
		return ExcedoConfig{}, err
	}

	cfg := ExcedoConfig{
		CommonConfig: common,
		APIToken:     strings.TrimSpace(os.Getenv("EXCEDO_API_TOKEN")),
		APIURL:       strings.TrimSpace(os.Getenv("EXCEDO_API_URL")),
	}

	if cfg.APIURL == "" {
		cfg.APIURL = defaultExcedoAPIURL
	}

	if cfg.APIToken == "" {
		return ExcedoConfig{}, fmt.Errorf("missing required environment variable EXCEDO_API_TOKEN")
	}

	return cfg, nil
}

func LoadBind() (BindConfig, error) {
	common, err := LoadCommon()
	if err != nil {
		return BindConfig{}, err
	}

	cfg := BindConfig{
		CommonConfig: common,
		DNSServer:    normalizeDNSServer(firstNonEmpty("BIND_DNS_SERVER", "ACME_GATEWAY_DNS_SERVER")),
		DNSZone:      normalizeFQDN(firstNonEmpty("BIND_DNS_ZONE", "ACME_GATEWAY_DNS_ZONE")),
		TTL:          defaultBindDNSTTL,
		TSIGKeyName: normalizeFQDN(firstNonEmpty(
			"BIND_DNS_TSIG_KEY_NAME",
			"ACME_GATEWAY_DNS_TSIG_KEY_NAME",
		)),
		TSIGSecret: strings.TrimSpace(firstNonEmpty(
			"BIND_DNS_TSIG_SECRET",
			"ACME_GATEWAY_DNS_TSIG_SECRET",
		)),
		TSIGAlgorithm: strings.TrimSpace(firstNonEmpty(
			"BIND_DNS_TSIG_ALGORITHM",
			"ACME_GATEWAY_DNS_TSIG_ALGORITHM",
		)),
	}

	if cfg.DNSServer == "" {
		cfg.DNSServer = defaultBindDNSServer
	}
	if cfg.DNSZone == "" {
		cfg.DNSZone = deriveDefaultZone(cfg.FQDN)
	}
	if cfg.DNSZone == "" {
		return BindConfig{}, fmt.Errorf("missing required DNS zone: BIND_DNS_ZONE or ACME_GATEWAY_DNS_ZONE")
	}

	if ttlRaw := strings.TrimSpace(firstNonEmpty("BIND_DNS_TTL", "ACME_GATEWAY_DNS_TTL")); ttlRaw != "" {
		ttlParsed, parseErr := strconv.ParseUint(ttlRaw, 10, 32)
		if parseErr != nil {
			return BindConfig{}, fmt.Errorf("invalid BIND_DNS_TTL value %q: %w", ttlRaw, parseErr)
		}
		cfg.TTL = uint32(ttlParsed)
	}

	if cfg.TSIGAlgorithm == "" {
		cfg.TSIGAlgorithm = defaultTSIGAlgorithm
	}
	if (cfg.TSIGKeyName == "") != (cfg.TSIGSecret == "") {
		return BindConfig{}, fmt.Errorf("BIND TSIG requires both key name and secret")
	}

	return cfg, nil
}

func normalizeDNSServer(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if strings.Contains(v, ":") {
		if _, _, err := net.SplitHostPort(v); err == nil {
			return v
		}
	}
	return net.JoinHostPort(v, "53")
}

func deriveDefaultZone(fqdn string) string {
	labels := strings.Split(normalizeFQDN(fqdn), ".")
	if len(labels) < 2 {
		return ""
	}
	return strings.Join(labels[len(labels)-2:], ".")
}

func Load() (ExcedoConfig, error) {
	return LoadExcedo()
}

type HookConfig = ExcedoConfig

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
