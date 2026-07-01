# acme-gateway-hooks

Standalone DNS hook toolkit for `acme-gateway` DNS-01 integrations.

## Supported Hooks

- BIND / RFC2136
  - `bind-dns-deploy`
  - `bind-dns-cleanup`
- Excedo
  - `excedo-dns-deploy`
  - `excedo-dns-cleanup`
- Azure DNS
  - `azure-dns-deploy`
  - `azure-dns-cleanup`

## Environment Contract

Common inputs (all binaries):
- `CERTBOT_DOMAIN` or `ACME_GATEWAY_DOMAIN`
- `CERTBOT_VALIDATION` or `ACME_GATEWAY_TOKEN`
- Optional `ACME_GATEWAY_FQDN` (defaults to `_acme-challenge.<domain>`)

BIND / RFC2136 variables:
- Optional `BIND_DNS_SERVER` (default: `127.0.0.1:53`)
- Optional `BIND_DNS_ZONE` (default inferred from FQDN using eTLD+1 with local fallback)
- Optional `BIND_DNS_TTL` (default: `60`)
- Optional TSIG:
  - `BIND_DNS_TSIG_KEY_NAME`
  - `BIND_DNS_TSIG_SECRET`
  - `BIND_DNS_TSIG_ALGORITHM` (default: `hmac-sha256.`)

Excedo variables:
- Required `EXCEDO_API_TOKEN`
- `EXCEDO_API_URL` (default: `https://api.domainname.systems`)
- Default zone behavior: if no explicit zone is set, the hook infers the parent zone (eTLD+1) from `CERTBOT_DOMAIN`/`ACME_GATEWAY_DOMAIN`.
- Optional explicit zone override:
  - `EXCEDO_DNS_ZONE` (preferred)
  - `EXCEDO_ZONE` (compatibility alias)
  - `EXCEDO_DOMAINNAME` (compatibility alias)
  - `ACME_GATEWAY_DNS_ZONE` (shared gateway alias)

Azure DNS variables:
- Required `AZURE_SUBSCRIPTION_ID`
- Required `AZURE_RESOURCE_GROUP`
- Required `AZURE_ZONE_NAME` (the delegated DNS zone, e.g., `challenges.example.com`)
- Required `AZURE_TENANT_ID`
- Required `AZURE_CLIENT_ID`
- **Exactly one** of:
  - `AZURE_CLIENT_SECRET` (service principal secret)
  - `AZURE_CLIENT_CERTIFICATE_PATH` (path to PKCS12 or PEM certificate)
- Optional `AZURE_CLIENT_CERTIFICATE_PASSWORD` (only if certificate is password-protected)
- Optional `AZURE_BASE_URL` (override ARM management **and** token endpoints; intended for testing)

## Local Usage

Build local binaries first:

```bash
make build-local
export ACME_HOOKS_BIN_DIR="$PWD/dist/bin-local"
```

### BIND / RFC2136

```bash
export CERTBOT_DOMAIN="test.pebble-test.local"
export CERTBOT_VALIDATION="challenge-value"
export ACME_GATEWAY_FQDN="_acme-challenge.test.pebble-test.local"
export BIND_DNS_SERVER="127.0.0.1:1053"
export BIND_DNS_ZONE="pebble-test.local"

./dist/bin-local/bind-dns-deploy
./dist/bin-local/bind-dns-cleanup
```

### Excedo

```bash
export EXCEDO_API_TOKEN="<token>"
export CERTBOT_DOMAIN="example.com"
export CERTBOT_VALIDATION="challenge-value"
# Optional override when account routing requires a specific domainname:
export EXCEDO_DNS_ZONE="example.com"

./dist/bin-local/excedo-dns-deploy
./dist/bin-local/excedo-dns-cleanup
```

### Azure DNS

```bash
# Service principal with secret
export AZURE_TENANT_ID="<tenant-id>"
export AZURE_CLIENT_ID="<client-id>"
export AZURE_CLIENT_SECRET="<secret>"
export AZURE_SUBSCRIPTION_ID="<subscription-id>"
export AZURE_RESOURCE_GROUP="dns-resources"
export AZURE_ZONE_NAME="challenges.example.com"
export CERTBOT_DOMAIN="app.example.com"
export CERTBOT_VALIDATION="challenge-value"

./dist/bin-local/azure-dns-deploy
./dist/bin-local/azure-dns-cleanup
```

Or with certificate:

```bash
# Service principal with certificate
export AZURE_TENANT_ID="<tenant-id>"
export AZURE_CLIENT_ID="<client-id>"
export AZURE_CLIENT_CERTIFICATE_PATH="/path/to/cert.pfx"
export AZURE_CLIENT_CERTIFICATE_PASSWORD="cert-password"  # optional
export AZURE_SUBSCRIPTION_ID="<subscription-id>"
export AZURE_RESOURCE_GROUP="dns-resources"
export AZURE_ZONE_NAME="challenges.example.com"
export CERTBOT_DOMAIN="app.example.com"
export CERTBOT_VALIDATION="challenge-value"

./dist/bin-local/azure-dns-deploy
./dist/bin-local/azure-dns-cleanup
```

Build release (linux/amd64 and linux/arm64) binaries:

```bash
make build
```

Local build output:
- `dist/bin-local/bind-dns-deploy`
- `dist/bin-local/bind-dns-cleanup`
- `dist/bin-local/excedo-dns-deploy`
- `dist/bin-local/excedo-dns-cleanup`
- `dist/bin-local/azure-dns-deploy`
- `dist/bin-local/azure-dns-cleanup`

Build output:
- `dist/bin/amd64/bind-dns-deploy`
- `dist/bin/amd64/bind-dns-cleanup`
- `dist/bin/amd64/excedo-dns-deploy`
- `dist/bin/amd64/excedo-dns-cleanup`
- `dist/bin/amd64/azure-dns-deploy`
- `dist/bin/amd64/azure-dns-cleanup`
- `dist/bin/arm64/bind-dns-deploy`
- `dist/bin/arm64/bind-dns-cleanup`
- `dist/bin/arm64/excedo-dns-deploy`
- `dist/bin/arm64/excedo-dns-cleanup`
- `dist/bin/arm64/azure-dns-deploy`
- `dist/bin/arm64/azure-dns-cleanup`

## Testing

- Run all tests with:

```bash
make test
```

- Integration coverage includes:
  - Excedo deploy/cleanup idempotency against a local fake Excedo API server.
  - BIND deploy/cleanup idempotency against a local in-process UDP DNS update responder.
  - Azure DNS config validation, record-name helpers, and client operations against a local httptest fake.

## Contributing New Providers

See `docs/adding-dns-provider.md` for the provider implementation workflow, test requirements, and documentation checklist.

## Kubernetes InitContainer Example

```yaml
initContainers:
  - name: install-hook-binaries
    image: ghcr.io/danieldonoghue/acme-gateway-hooks:latest
    command: ["/bin/sh", "-ec"]
    args:
      - |
        cp /usr/local/bin/*-dns-deploy /hooks
        cp /usr/local/bin/*-dns-cleanup /hooks
        chmod 0555 /hooks/*-dns-deploy /hooks/*-dns-cleanup
    securityContext:
      allowPrivilegeEscalation: false
      readOnlyRootFilesystem: true
      runAsNonRoot: true
      runAsUser: 65532
      capabilities:
        drop: ["ALL"]
    volumeMounts:
      - name: hooks
        mountPath: /hooks
containers:
  - name: acme-gateway
    volumeMounts:
      - name: hooks
        mountPath: /hooks
        readOnly: true
volumes:
  - name: hooks
    emptyDir: {}
```

Then configure `acme-gateway`:

```yaml
dns_hook:
  # BIND / RFC2136
  deploy_script: "/hooks/bind-dns-deploy"
  cleanup_script: "/hooks/bind-dns-cleanup"

  # Or Excedo
  # deploy_script: "/hooks/excedo-dns-deploy"
  # cleanup_script: "/hooks/excedo-dns-cleanup"

  # Or Azure DNS
  # deploy_script: "/hooks/azure-dns-deploy"
  # cleanup_script: "/hooks/azure-dns-cleanup"
```

## Versioning

- SemVer tags are published as `ghcr.io/danieldonoghue/acme-gateway-hooks:vX.Y.Z`.
- `latest` tracks the newest released SemVer tag.
- Versions are pinned to the minimum acme-gateway version required to support these hooks.

## Security Notes

- Secrets are never logged.
- Cleanup only deletes TXT records matching the exact challenge value.
- Cleanup is idempotent.
