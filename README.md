# acme-gateway-hooks

Standalone DNS hook toolkit for `acme-gateway` DNS-01 integrations.

## Supported Hooks

- Excedo
  - `excedo-dns-deploy`
  - `excedo-dns-cleanup`

This repository is structured for additional provider binaries over time, for example:
- `route53-dns-deploy`
- `cloudflare-dns-cleanup`

## Environment Contract

Required:
- `EXCEDO_API_TOKEN`

Optional:
- `EXCEDO_API_URL` (default: `https://api.domainname.systems`)
- `ACME_GATEWAY_FQDN` (default: `_acme-challenge.<domain>`)

Domain input fallback:
- `CERTBOT_DOMAIN`
- `ACME_GATEWAY_DOMAIN`

TXT input fallback:
- `CERTBOT_VALIDATION`
- `ACME_GATEWAY_TOKEN`

## Local Usage

```bash
export EXCEDO_API_TOKEN="<token>"
export CERTBOT_DOMAIN="example.com"
export CERTBOT_VALIDATION="challenge-value"

./dist/bin/excedo-dns-deploy
./dist/bin/excedo-dns-cleanup
```

Build binaries:

```bash
make build
```

## Kubernetes InitContainer Example

```yaml
initContainers:
  - name: install-excedo-hook
    image: ghcr.io/<org>/acme-gateway-hooks:latest
    command: ["/bin/sh", "-ec"]
    args:
      - |
        cp /usr/local/bin/excedo-dns-deploy /hooks/excedo-dns-deploy
        cp /usr/local/bin/excedo-dns-cleanup /hooks/excedo-dns-cleanup
        chmod 0555 /hooks/excedo-dns-deploy /hooks/excedo-dns-cleanup
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
  deploy_script: "/hooks/excedo-dns-deploy"
  cleanup_script: "/hooks/excedo-dns-cleanup"
```

## Versioning

- SemVer tags are published as `ghcr.io/<org>/acme-gateway-hooks:vX.Y.Z`.
- `latest` tracks the newest released SemVer tag.

## Security Notes

- Secrets are never logged.
- Cleanup only deletes TXT records matching the exact challenge value.
- Cleanup is idempotent.
