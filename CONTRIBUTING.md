# Contributing

## Development

- Go version: `1.26.4`
- Run `make lint test security build` before opening a PR.
- `make test` runs unit tests and integration tests for both Excedo and BIND hooks.

## Adding Providers

- For a full provider implementation checklist and code patterns, see `docs/adding-dns-provider.md`.

## Configuration Ownership

- Keep shared ACME env parsing and common field validation in `internal/env`.
- Keep provider-specific config fields and rules in provider packages:
	- `internal/excedo`
	- `internal/bind`
- Prefer provider loaders (`excedo.LoadConfig()`, `bind.LoadConfig()`) in app and integration code.

## Pull Requests

- Keep changes focused and include tests.
- Update `README.md` when changing behavior or environment contract.
- Add a changelog entry in `CHANGELOG.md` for user-facing changes.
