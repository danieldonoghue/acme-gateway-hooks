# Adding a DNS Provider

This guide describes the expected structure for adding a new DNS provider to this repository.

## Design Principles

- Keep shared ACME env handling in `internal/env`.
- Keep provider-specific config fields and validation in the provider package.
- Keep command binaries thin.
- Ensure cleanup remains idempotent.
- Add unit tests and at least one integration test for deploy and cleanup behavior.

## 1. Create Provider Package

Add a new package at:

- `internal/<provider>/`

Typical files:

- `config.go`
- `client.go` (if external API client is needed)
- `ops.go`
- `types.go` (optional)
- `*_test.go`

## 2. Define Provider Config

In `internal/<provider>/config.go`:

1. Define a `Config` struct that embeds `env.CommonConfig`.
2. Add provider env tags for provider fields.
3. Implement:
   - `func LoadConfig() (Config, error)`
   - `func (c *Config) Validate() error`
4. Use `env.LoadAndValidate(&cfg)` in `LoadConfig()`.
5. Call `c.CommonConfig.Validate()` inside provider `Validate()` first.

Pattern:

```go
package provider

import "github.com/danieldonoghue/acme-gateway-hooks/internal/env"

type Config struct {
    env.CommonConfig
    APIToken string `env:"PROVIDER_API_TOKEN,required"`
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
    // Provider-specific validation.
    return nil
}
```

## 3. Implement Provider Operations

In `internal/<provider>/ops.go` implement:

- `Deploy(ctx, logger, clientOrDeps, cfg)`
- `Cleanup(ctx, logger, clientOrDeps, cfg)`

Requirements:

- Deploy should return a hard error on failure.
- Cleanup should be idempotent:
  - treat not-found and retryable edge cases as success where appropriate
  - log warnings for transient failures

## 4. Add Command Binaries

Create two binaries:

- `cmd/<provider>-dns-deploy/main.go`
- `cmd/<provider>-dns-cleanup/main.go`

Each binary should:

1. Initialize logging.
2. Load config using `<provider>.LoadConfig()`.
3. Create provider client/dependencies.
4. Call `Deploy` or `Cleanup`.
5. Exit non-zero on deploy failure.
6. Keep cleanup behavior idempotent.

## 5. Update Build and Packaging

Ensure provider binaries are included where needed:

- `Makefile` build targets
- `Dockerfile` copy/install patterns
- release workflow (if applicable)

If naming follows existing convention, use:

- `<provider>-dns-deploy`
- `<provider>-dns-cleanup`

## 6. Add Tests

### Unit Tests

Add provider package tests for:

- config defaults and normalization
- required var validation
- parsing failures
- API/operation edge cases

### Integration Tests

Add at least one test in:

- `test/integration/<provider>_integration_test.go`

Cover:

- deploy success path
- cleanup success path
- cleanup idempotency (second cleanup should still succeed)

Use an in-process fake service:

- `httptest.NewServer` for HTTP APIs
- local UDP responder for DNS protocols

## 7. Document Environment Contract

Update:

- `README.md` environment contract section
- local usage examples
- any provider-specific notes

Also add an entry in:

- `CHANGELOG.md` under `[Unreleased]`

## 8. Validate Before PR

Run:

```bash
make lint test security build
```

PR checklist:

- [ ] Provider config uses `env.LoadAndValidate`
- [ ] `Config.Validate()` calls `CommonConfig.Validate()`
- [ ] Deploy and cleanup binaries exist
- [ ] Cleanup is idempotent
- [ ] Unit tests added
- [ ] Integration test added
- [ ] README updated
- [ ] CHANGELOG updated
