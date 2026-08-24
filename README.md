# No Oops Ops

No Oops Ops is a self-hosted CLI for repeatable Docker Swarm deployments. It creates immutable releases in an internal registry, deploys them with Docker Stack, records deployment history, and manages environment-scoped Swarm secrets.

It is intended for small Docker-based services that need a dependable deployment workflow without operating a full platform.

## Quick start

Prerequisites: Go 1.25+, Docker running locally, and Docker configured to allow the local HTTP registry. See [Getting started](docs/getting-started.md).

```bash
make install
noops install
noops release prod examples/lango.app.yml
noops deploy prod examples/lango.app.yml
```

If the app uses secrets, create them before deployment:

```bash
noops secret set prod AUTH_SERVER_API_CLIENT_SECRET
```

## Documentation

- [Getting started](docs/getting-started.md) — requirements, installation, and first deployment
- [Concepts](docs/concepts.md) — releases, deployments, rollouts, secrets, and state
- [Command reference](docs/commands.md) — all implemented CLI commands
- [App manifest reference](docs/reference/app-manifest.md) — supported manifest fields and defaults
- [Environment-file reference](docs/reference/env-file.md) — generated `.env` values and secret references
- [Configuration and generated state](docs/reference/configuration.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Current limitations](docs/limitations.md)

## Typical workflow

```text
install → secret set (when needed) → release → deploy → rollback or remove
```

`release` creates and pushes an immutable timestamped image. `deploy` uses the latest recorded release by default, or a supplied release tag. Docker Swarm owns task health, replacement, and automatic rollout rollback; No Oops Ops waits for and records the final Swarm outcome.

## Development

```bash
make build
make test
```

`make build` writes the local executable to `.bin/noops`; `make install` installs it to `~/.local/bin/noops`.

## Status

The project currently supports a local Docker Swarm, a plain-HTTP internal registry, and a shared nginx ingress on ports 80 and 443. Route and TLS configuration, and registry garbage collection policy, are not implemented. See [Current limitations](docs/limitations.md).

## License

This project is licensed under the terms in [LICENSE](LICENSE).
