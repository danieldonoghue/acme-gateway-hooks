package azure

import (
	"fmt"

	"github.com/danieldonoghue/acme-gateway-hooks/internal/env"
)

type Config struct {
	env.CommonConfig
	SubscriptionID     string `env:"AZURE_SUBSCRIPTION_ID,required"`
	ResourceGroup      string `env:"AZURE_RESOURCE_GROUP,required"`
	ZoneName           string `env:"AZURE_ZONE_NAME,required"`
	TenantID           string `env:"AZURE_TENANT_ID"`
	ClientID           string `env:"AZURE_CLIENT_ID,required"`
	ClientSecret       string `env:"AZURE_CLIENT_SECRET"`
	ClientCertPath     string `env:"AZURE_CLIENT_CERTIFICATE_PATH"`
	ClientCertPassword string `env:"AZURE_CLIENT_CERTIFICATE_PASSWORD"`
	BaseURL            string `env:"AZURE_BASE_URL"`
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

	if c.SubscriptionID == "" {
		return fmt.Errorf("missing required environment variable AZURE_SUBSCRIPTION_ID")
	}
	if c.ResourceGroup == "" {
		return fmt.Errorf("missing required environment variable AZURE_RESOURCE_GROUP")
	}
	if c.ZoneName == "" {
		return fmt.Errorf("missing required environment variable AZURE_ZONE_NAME")
	}
	if c.ClientID == "" {
		return fmt.Errorf("missing required environment variable AZURE_CLIENT_ID")
	}

	// Validate mutual exclusivity of secret vs certificate
	hasSecret := c.ClientSecret != ""
	hasCert := c.ClientCertPath != ""

	if !hasSecret && !hasCert {
		return fmt.Errorf("exactly one of AZURE_CLIENT_SECRET or AZURE_CLIENT_CERTIFICATE_PATH must be set")
	}

	if hasSecret && hasCert {
		return fmt.Errorf("AZURE_CLIENT_SECRET and AZURE_CLIENT_CERTIFICATE_PATH are mutually exclusive; use only one")
	}

	return nil
}
