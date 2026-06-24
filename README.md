# acme-gateway-hooks

Standalone DNS hook toolkit for `acme-gateway` DNS-01 integrations.

## Supported Hooks

- BIND / RFC2136
  - `bind-dns-deploy`
  - `bind-dns-cleanup`
- Excedo
  - `excedo-dns-deploy`
  - `excedo-dns-cleanup`

## Environment Contract

Common inputs (all binaries):
- `CERTBOT_DOMAIN` or `ACME_GATEWAY_DOMAIN`
- `CERTBOT_VALIDATION` or `ACME_GATEWAY_TOKEN`
- Optional `ACME_GATEWAY_FQDN` (defaults to `_acme-challenge.<domain>`)

BIND / RFC2136 variables:
- Optional `BIND_DNS_SERVER` (default: `127.0.0.1:53`)
- Optional `BIND_DNS_ZONE` (default inferred from FQDN, last two labels)
- Optional `BIND_DNS_TTL` (default: `60`)
- Optional TSIG:
  - `BIND_DNS_TSIG_KEY_NAME`
  - `BIND_DNS_TSIG_SECRET`
  - `BIND_DNS_TSIG_ALGORITHM` (default: `hmac-sha256.`)

Excedo variables:
- Required `EXCEDO_API_TOKEN`
- `EXCEDO_API_URL` (default: `https://api.domainname.systems`)

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

./dist/bin-local/excedo-dns-deploy
./dist/bin-local/excedo-dns-cleanup
```

Build release (linux/amd64) binaries:

```bash
make build
```

Local build output:
- `dist/bin-local/bind-dns-deploy`
- `dist/bin-local/bind-dns-cleanup`
- `dist/bin-local/excedo-dns-deploy`
- `dist/bin-local/excedo-dns-cleanup`

Build output:
- `dist/bin/bind-dns-deploy`
- `dist/bin/bind-dns-cleanup`
- `dist/bin/excedo-dns-deploy`
- `dist/bin/excedo-dns-cleanup`

## Testing

- Run all tests with:

```bash
make test
```

- Integration coverage includes:
  - Excedo deploy/cleanup idempotency against a local fake Excedo API server.
  - BIND deploy/cleanup idempotency against a local in-process UDP DNS update responder.

## Contributing New Providers

See `docs/adding-dns-provider.md` for the provider implementation workflow, test requirements, and documentation checklist.

## Kubernetes InitContainer Example

```yaml
initContainers:
  - name: install-hook-binaries
    image: ghcr.io/<org>/acme-gateway-hooks:latest
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
```

## Versioning

- SemVer tags are published as `ghcr.io/<org>/acme-gateway-hooks:vX.Y.Z`.
- `latest` tracks the newest released SemVer tag.

## Security Notes

- Secrets are never logged.
- Cleanup only deletes TXT records matching the exact challenge value.
- Cleanup is idempotent.
