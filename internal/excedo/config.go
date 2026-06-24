package excedo

import (
	"fmt"

	"github.com/danieldonoghue/acme-gateway-hooks/internal/env"
)

const defaultAPIURL = "https://api.domainname.systems"

type Config struct {
	env.CommonConfig
	APIToken string `env:"EXCEDO_API_TOKEN,required"`
	APIURL   string `env:"EXCEDO_API_URL,default=https://api.domainname.systems"`
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
	return nil
}
