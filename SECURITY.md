# Security Policy

## Reporting a Vulnerability

Report vulnerabilities privately via repository security advisories.

## Security Practices

- Hook binaries do not log secrets.
- Environment validation fails fast for required inputs.
- Container image runs as non-root with dropped capabilities.
- CI includes `govulncheck` in the security target.
