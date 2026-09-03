# No Oops Ops

No Oops Ops is a self-hosted CLI for repeatable Docker Swarm deployments. It fetches application source from Git,
builds every image in a one-shot isolated Swarm task, creates immutable releases in an internal registry, deploys them
with Docker
Stack, records deployment history, and manages environment-scoped Swarm secrets.

It is intended for services that need a dependable deployment workflow without operating a full platform.

## Quick start

Prerequisites: Go 1.25+, Docker running locally, and Docker configured to allow the local HTTP registry.
See [Getting started](docs/getting-started.md).

```bash
make install
noops init /srv/noops/example
cd /srv/noops/example
# Add the `redis` app alias and its manifest path to apps.yml.
noops install
noops secret set prod REDIS_PASSWORD
noops release prod redis
noops deploy prod redis
# Or create a fresh release and deploy that exact image in one command.
noops release --deploy prod redis
```

`init` creates an empty, version-matched `apps.yml` catalog. Add each app's stable alias and Compose-shaped manifest
path before releasing it; see [Configuration and generated state](docs/reference/configuration.md) for the catalog
format and the [`redis` example](examples/paas/apps/redis/app.yml) for a manifest.

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
- [Networking and TLS](docs/networking-tls.md) — ingress ports, routing, Let's Encrypt, and Cloudflare Origin TLS
- [Troubleshooting](docs/troubleshooting.md)
- [Current limitations](docs/limitations.md)

## Typical workflow

```text
install → secret set (when needed) → release → release list → deploy → rollback or remove
```

`apps.yml` maps stable app names to ordinary Compose-shaped `app.yml` files. Add `x-noops` only for No Oops behavior;
standard Compose and Swarm fields remain the application's source of truth. Lifecycle commands require
`--service <name>` or `--all`; `--all` uses a stable dependency order and stops at the first failure.

`release` creates and pushes an immutable timestamped image. `release list` shows the recorded releases for a service,
newest first. Every build runs in a short-lived Swarm task; for `x-noops.build.source.git`, No Oops first fetches the
source in a temporary workspace. Private-source fetches use a short-lived Swarm Git-fetch task and mount an
environment-scoped source secret only into that task. Build toolchains such as Maven, Java, Node, or Go belong in the
Dockerfile—not on the deployment host. `deploy` uses the latest
recorded release by default (and creates one when none exists), waits for the configured health and rollout result, and
records
the final Swarm outcome. `install` waits for the managed registry and nginx services to become ready; `status` reports
their task readiness and marks partially running services as degraded.

## Development

```bash
make build
make test
make release-snapshot
```

`make build` writes the local executable to `.bin/noops`; `make install` installs it to `~/.local/bin/noops`.
`make release-snapshot` verifies the release archive locally. `make release VERSION=0.0.1` creates and pushes the
corresponding `v0.0.1` tag; GitHub then publishes the tagged release.

## Current capabilities

The project currently supports a local Docker Swarm, a plain-HTTP internal registry, Git-backed container builds,
environment-scoped Swarm secrets, imported origin certificates, and a shared nginx ingress with manifest-driven HTTP,
Let's Encrypt TLS, or Cloudflare Origin TLS routes. `noops cleanup --apply` removes selected release
manifests and metadata, then runs offline registry garbage collection when needed.
See [Current limitations](docs/limitations.md).

## License

This project is licensed under the terms in [LICENSE](LICENSE).
