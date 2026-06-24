package env

import (
	"errors"
	"strings"
	"testing"
)

type sampleConfig struct {
	Required string `env:"PRIMARY|SECONDARY,required"`
	Count    uint32 `env:"COUNT,default=60"`
	Enabled  bool   `env:"ENABLED,default=true"`
}

func (s *sampleConfig) Validate() error {
	if s.Required == "bad" {
		return errors.New("validator failed")
	}
	return nil
}

func TestLoadAppliesFallbackAndDefaults(t *testing.T) {
	lookup := func(key string) (string, bool) {
		values := map[string]string{
			"SECONDARY": "value-from-secondary",
		}
		v, ok := values[key]
		return v, ok
	}

	var cfg sampleConfig
	err := Load(&cfg, WithLookup(lookup))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Required != "value-from-secondary" {
		t.Fatalf("Required = %q", cfg.Required)
	}
	if cfg.Count != 60 {
		t.Fatalf("Count = %d", cfg.Count)
	}
	if !cfg.Enabled {
		t.Fatal("Enabled expected true")
	}
}

func TestLoadCollectsErrors(t *testing.T) {
	var cfg sampleConfig
	err := Load(&cfg, WithLookup(func(string) (string, bool) { return "", false }))
	if err == nil {
		t.Fatal("expected error")
	}

	var loadErr *LoadError
	if !errors.As(err, &loadErr) {
		t.Fatalf("expected LoadError, got %T", err)
	}
	if len(loadErr.Problems) == 0 {
		t.Fatal("expected at least one problem")
	}
	if !strings.Contains(loadErr.Error(), "missing required environment variable") {
		t.Fatalf("unexpected error text: %s", loadErr.Error())
	}
}

func TestLoadAndValidateError(t *testing.T) {
	lookup := func(key string) (string, bool) {
		if key == "PRIMARY" {
			return "bad", true
		}
		return "", false
	}

	var cfg sampleConfig
	err := LoadAndValidate(&cfg, WithLookup(lookup))
	if err == nil {
		t.Fatal("expected validator error")
	}
	if !strings.Contains(err.Error(), "validator failed") {
		t.Fatalf("unexpected validator error: %v", err)
	}
}

type unsupportedTypeConfig struct {
	Values []string `env:"VALUES,default=a"`
}

func TestLoadUnsupportedFieldType(t *testing.T) {
	var cfg unsupportedTypeConfig
	err := Load(&cfg, WithLookup(func(string) (string, bool) { return "", false }))
	if err == nil {
		t.Fatal("expected unsupported type error")
	}
	if !strings.Contains(err.Error(), "unsupported field type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type nonValidatableConfig struct {
	Required string `env:"PRIMARY,required"`
}

func TestLoadAndValidateRequiresValidatable(t *testing.T) {
	var cfg nonValidatableConfig
	err := LoadAndValidate(&cfg, WithLookup(func(key string) (string, bool) {
		if key == "PRIMARY" {
			return "ok", true
		}
		return "", false
	}))
	if err == nil {
		t.Fatal("expected Validatable enforcement error")
	}
	if !strings.Contains(err.Error(), "must implement Validatable") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCommonConfigValidateNormalizesAndDefaults(t *testing.T) {
	cfg := CommonConfig{
		Domain:     " Example.COM ",
		Validation: " challenge ",
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if cfg.Domain != "example.com" {
		t.Fatalf("Domain = %q", cfg.Domain)
	}
	if cfg.Validation != "challenge" {
		t.Fatalf("Validation = %q", cfg.Validation)
	}
	if cfg.FQDN != "_acme-challenge.example.com" {
		t.Fatalf("FQDN = %q", cfg.FQDN)
	}
}
