# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added
- Initial Excedo DNS-01 deploy and cleanup hook binaries.
- Excedo API client with strict JSON decoding, retries, and timeout handling.
- Unit and integration test scaffolding.
- CI and release workflow scaffolding.
- BIND integration test for deploy/cleanup idempotency using an in-process UDP DNS update responder.

### Changed
- Refactored environment loading to an ACME-focused shared model in `internal/env` with tagged field parsing.
- Moved provider-specific config ownership and validation to provider packages (`internal/bind`, `internal/excedo`).
- Standardized provider config loading through `bind.LoadConfig()` and `excedo.LoadConfig()`.
