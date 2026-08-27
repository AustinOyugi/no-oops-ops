# Workspace storage

No Oops is workspace-based. Initialize a workspace with `noops init <directory>`.
The CLI may write only below that workspace's `.noops/` directory. Use the
workspace as the current directory or provide it with `--workspace`.

```text
workspace/
  apps.yml
  apps/
  .noops/
    config.yml
    state/
    data/
```

Commit `apps.yml` and `apps/`; add `.noops/` to `.gitignore`.

`init` writes an initial `apps.yml` when none exists. It is the source of
truth for platform settings and app aliases:

```yaml
version: v0.1.0

settings:
  platform:
    network:
      name: cranium-platform
    registry:
      name: cranium-registry
      port: 5000
    ingress:
      name: cranium-ingress
      http_port: 80
      https_port: 443
	  # Trust Cloudflare's client-IP header only from Cloudflare networks.
	  cloudflare: true
    networks:
      default: "cranium-{environment}"
      environments:
        prod: cranium-prod
        staging: cranium-staging

apps:
  nyota:
    manifest: ./apps/nyota/app.yml
```

`platform.network.name` is used only by No Oops platform services. Application
deployments use `platform.networks`: an explicit environment mapping wins, and
otherwise `{environment}` in `default` is replaced by the selected environment.
No Oops creates that overlay network on demand. The managed ingress joins an
environment network only when it serves an exposed app in that environment.

Set `platform.ingress.cloudflare: true` only when public ingress hostnames are
Cloudflare-proxied. No Oops then generates Nginx `set_real_ip_from` directives
for Cloudflare's published IPv4 and IPv6 proxy networks and uses
`CF-Connecting-IP` as the verified client address. This makes the generated
`X-Real-IP` and `X-Forwarded-For` headers contain the visitor address while
preventing direct callers from spoofing it. Keep the proxy enabled (orange
cloud) for every hostname served by that ingress. Cloudflare mode never uses
Let's Encrypt or prompts for an ACME email. Each HTTPS app must instead set
`x-noops.ingress.tls_certificate` to the name of a certificate imported with
`noops certificate import`.

The `version` value must exactly match `noops --version`. A release binary
creates a matching value during `noops init`; update the catalog in the same
change as the CLI when adopting a new No Oops release.

The workspace state directory contains paths such as:

```text
install.json
registry/config.yml
registry/stack.yml
nginx/stack.yml
apps/<app>/<environment>/.env
apps/<app>/<environment>/stack.yml
apps/<app>/<environment>/release.json
apps/<app>/<environment>/releases/<timestamp>.json
apps/<app>/<environment>/deployments/<timestamp>.json
secrets/<environment>/<key>/v<version>.json
```

Secret files contain metadata only; values are stored by Docker Swarm. `remove` deletes an app environment's generated
state but preserves its secrets and named volumes. `uninstall --purge` also deletes the configured persistent registry
data.
