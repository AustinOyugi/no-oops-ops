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
version: 1

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
