package bind

import (
	"fmt"
	"net"
	"strings"

	"github.com/danieldonoghue/acme-gateway-hooks/internal/env"
	"golang.org/x/net/publicsuffix"
)

const defaultDNSServer = "127.0.0.1:53"
const defaultDNSTTL uint32 = 60
const defaultTSIGAlgorithm = "hmac-sha256."

type Config struct {
	env.CommonConfig
	DNSServer     string `env:"BIND_DNS_SERVER|ACME_GATEWAY_DNS_SERVER,default=127.0.0.1:53"`
	DNSZone       string `env:"BIND_DNS_ZONE|ACME_GATEWAY_DNS_ZONE"`
	TTL           uint32 `env:"BIND_DNS_TTL|ACME_GATEWAY_DNS_TTL,default=60"`
	TSIGKeyName   string `env:"BIND_DNS_TSIG_KEY_NAME|ACME_GATEWAY_DNS_TSIG_KEY_NAME"`
	TSIGSecret    string `env:"BIND_DNS_TSIG_SECRET|ACME_GATEWAY_DNS_TSIG_SECRET"`
	TSIGAlgorithm string `env:"BIND_DNS_TSIG_ALGORITHM|ACME_GATEWAY_DNS_TSIG_ALGORITHM,default=hmac-sha256."`
}

func LoadConfig() (Config, error) {
	cfg := Config{}
	if err := env.LoadAndValidate(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c *Config) Validate() error {
	if err := c.CommonConfig.Validate(); err != nil {
		return err
	}

	c.DNSServer = normalizeDNSServer(c.DNSServer)
	if c.DNSServer == "" {
		c.DNSServer = defaultDNSServer
	}

	c.DNSZone = env.NormalizeFQDN(c.DNSZone)
	if c.DNSZone == "" {
		c.DNSZone = deriveDefaultZone(c.FQDN)
	}

	if c.DNSZone == "" {
		return fmt.Errorf("missing required DNS zone: BIND_DNS_ZONE or ACME_GATEWAY_DNS_ZONE")
	}

	c.TSIGKeyName = env.NormalizeFQDN(c.TSIGKeyName)
	c.TSIGSecret = strings.TrimSpace(c.TSIGSecret)
	c.TSIGAlgorithm = strings.TrimSpace(c.TSIGAlgorithm)

	if c.TSIGAlgorithm == "" {
		c.TSIGAlgorithm = defaultTSIGAlgorithm
	}

	if (c.TSIGKeyName == "") != (c.TSIGSecret == "") {
		return fmt.Errorf("BIND TSIG requires both key name and secret")
	}

	if c.TTL == 0 {
		c.TTL = defaultDNSTTL
	}

	return nil
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
	normalized := env.NormalizeFQDN(fqdn)
	if normalized == "" {
		return ""
	}

	if etldPlusOne, err := publicsuffix.EffectiveTLDPlusOne(normalized); err == nil {
		return etldPlusOne
	}

	labels := strings.Split(normalized, ".")
	if len(labels) < 2 {
		return ""
	}
	return strings.Join(labels[len(labels)-2:], ".")
}
