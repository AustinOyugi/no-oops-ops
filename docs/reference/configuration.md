# Configuration and generated state

Settings may be set as environment variables or in `$XDG_CONFIG_HOME/noops/.env.noops` (normally `~/.config/noops/.env.noops`). Environment variables take precedence when the CLI starts.

| Variable | Default | Purpose |
| --- | --- | --- |
| `NOOPS_STATE_DIR` | `$XDG_STATE_HOME/noops` | Installation metadata, rendered app files, release/deployment metadata, and secret metadata. |
| `NOOPS_DATA_DIR` | `$XDG_DATA_HOME/noops` | Persistent registry data. |
| `NOOPS_NETWORK_NAME` | `noops-net` | Shared external Swarm network. |
| `NOOPS_REGISTRY_NAME` | `noops-registry` | Internal registry service/stack name. |
| `NOOPS_REGISTRY_PORT` | `5000` | Internal registry port. |
| `NOOPS_NGINX_NAME` | `noops-nginx` | Shared nginx ingress stack name. |
| `NOOPS_NGINX_HTTP_PORT` | `80` | Host port published to nginx HTTP. |
| `NOOPS_NGINX_HTTPS_PORT` | `443` | Host port published to nginx HTTPS. |
| `NOOPS_ACME_EMAIL` | — | Email address registered with Let's Encrypt when an exposed route enables TLS. If unset, `noops deploy` prompts for and securely stores it. |

The state directory contains paths such as:

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

Secret files contain metadata only; values are stored by Docker Swarm. `remove` deletes an app environment's generated state but preserves its secrets and named volumes. `uninstall --purge` also deletes the configured persistent registry data.
