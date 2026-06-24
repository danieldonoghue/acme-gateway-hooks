package excedo

import (
	"fmt"
	"strings"

	"github.com/danieldonoghue/acme-gateway-hooks/internal/env"
	"golang.org/x/net/publicsuffix"
)

const defaultAPIURL = "https://api.domainname.systems"

type Config struct {
	env.CommonConfig
	APIToken string `env:"EXCEDO_API_TOKEN,required"`
	APIURL   string `env:"EXCEDO_API_URL,default=https://api.domainname.systems"`
	DNSZone  string `env:"EXCEDO_DNS_ZONE|EXCEDO_ZONE|EXCEDO_DOMAINNAME|ACME_GATEWAY_DNS_ZONE"`
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

	if c.APIURL == "" {
		c.APIURL = defaultAPIURL
	}

	if c.APIToken == "" {
		return fmt.Errorf("missing required environment variable EXCEDO_API_TOKEN")
	}
	c.DNSZone = env.NormalizeFQDN(c.DNSZone)
	if c.DNSZone == "" {
		c.DNSZone = deriveDefaultZone(c.Domain, c.FQDN)
	}
	return nil
}

func deriveDefaultZone(domain, fqdn string) string {
	if domain = env.NormalizeFQDN(domain); domain != "" {
		if etldPlusOne, err := publicsuffix.EffectiveTLDPlusOne(domain); err == nil {
			return etldPlusOne
		}
		labels := strings.Split(domain, ".")
		if len(labels) >= 2 {
			return strings.Join(labels[len(labels)-2:], ".")
		}
		return domain
	}

	if fqdn = env.NormalizeFQDN(fqdn); fqdn != "" {
		if etldPlusOne, err := publicsuffix.EffectiveTLDPlusOne(fqdn); err == nil {
			return etldPlusOne
		}
		labels := strings.Split(fqdn, ".")
		if len(labels) >= 2 {
			return strings.Join(labels[len(labels)-2:], ".")
		}
		return fqdn
	}

	return ""
}
