# No Oops Ops

`No Oops Ops` is a lightweight self-hosted deployment manager for Docker-based applications.

The current implementation focuses on a practical local/server workflow:

- bootstrap Docker Swarm and an internal registry
- build and publish immutable app images
- generate deployment artifacts from manifests
- deploy apps with Docker Stack
- wait for service readiness and surface useful diagnostics
- manage environment-scoped, versioned Docker Swarm secrets
- retain release and deployment history, including rollback to the prior deployment

The goal is to keep simple deployments repeatable without building a full DevOps platform.

## Requirements

- Go 1.25 or later
- Docker running locally
- Docker configured to allow the local registry (see [Docker Registry](#docker-registry))

Run commands from the repository root. By default, No Oops Ops stores state in the
platform XDG state directory under `noops`; set `NOOPS_STATE_DIR` to use another
location.

## Install the CLI

Build a repository-local command:

```bash
make build
./.bin/noops status
```

Or install it with one command, then use `noops` from any directory:

```bash
make install
noops status
```

This installs into `~/.local/bin`, which is the user-facing command location on
this setup. After pulling or changing the code, rerun `make install` to force a
fresh rebuild and replace the installed command.

## Releases

GoReleaser creates archives for macOS and Linux. Validate the release
configuration locally with:

```bash
make release-check
make release-snapshot
```

`release-snapshot` writes unpublished artifacts to `dist/`. Pushing a tag such
as `v0.1.0` runs the GitHub Actions release workflow and publishes the archives
and checksums to the matching GitHub Release. Until that first tag, GitHub
Actions builds snapshots only; it does not create a public release.

## Current Workflow

The current happy path is:

```bash
noops install
noops secret set prod AUTH_SERVER_API_CLIENT_SECRET
noops release prod examples/lango.app.yml
noops deploy prod examples/lango.app.yml
```

To deploy a particular previously released version instead of the most recent one:

```bash
noops deploy prod examples/lango.app.yml 20260101-120000
```

You can also inspect the platform:

```bash
noops doctor
noops doctor --deploy-ready
noops status
```

## Install

Install prepares the local platform pieces:

- verifies Docker
- initializes Docker Swarm if needed
- ensures the shared Docker network exists
- writes registry config and stack files
- deploys the internal registry
- writes install metadata

Run:

```bash
noops install
```

The install state is written under the configured state directory.

## Release

Release builds and publishes an immutable image for an app/environment.

Run:

```bash
noops release prod examples/lango.app.yml
```

Release currently does:

- loads the app manifest
- runs the optional pre-build command from `source.build.command`
- builds the Docker image directly with the internal registry name and a generated timestamp tag
- pushes it to the internal registry
- writes versioned release metadata

Example generated metadata:

```text
.noops/apps/lango/prod/releases/20260101-120000.json
```

Each release is tagged with a UTC timestamp in the format `YYYYMMDD-HHMMSS`. Deploy
reads this metadata later, so the manifest does not need to be manually updated with a
new image tag on every release.

For example, a release of `lango-service` is built and pushed as:

```text
127.0.0.1:5000/lango-service:20260101-120000
```

## Deploy

Deploy consumes the latest release metadata for an app/environment by default. Pass a
release tag as the optional third argument to deploy a specific version.

Run:

```bash
noops deploy prod examples/lango.app.yml

# deploy a specific release
noops deploy prod examples/lango.app.yml 20260101-120000
```

Deploy currently does:

- runs the deploy-readiness preflight checks
- loads the app manifest
- loads the referenced env YAML file
- resolves environment-specific env values
- writes `.env`
- writes `stack.yml`
- resolves the latest release metadata, or the supplied release tag
- renders the stack with the released registry image
- runs `docker stack deploy`
- verifies the Swarm service exists
- waits for running tasks and Docker health checks to pass
- prints task diagnostics on readiness timeout

Generated app artifacts are written under:

```text
.noops/apps/<app>/<environment>/
```

For example:

```text
.noops/apps/lango/prod/.env
.noops/apps/lango/prod/stack.yml
.noops/apps/lango/prod/releases/20260101-120000.json
.noops/apps/lango/prod/release.json
.noops/apps/lango/prod/deployments/20260101-120500.json
```

`release.json` records the release most recently deployed for that app and environment;
the versioned files under `releases/` are the release history.

Each successful rollout is also written to `deployments/`. This history is distinct from
released images: it records what was actually deployed. Roll back the current deployment
to the immediately preceding successful deployment with:

```bash
noops rollback prod examples/lango.app.yml
```

Rollback redeploys the previous recorded release and is itself recorded, so a later
rollback can restore the version it replaced. At least two successful deployments are
required before rollback is available. Unlike `deploy`, rollback does not currently run
the deploy-readiness preflight automatically.

## Doctor

`doctor` reports whether the local platform is healthy. It has two profiles:

- `noops doctor` runs the full diagnostic profile, including installation artifacts.
- `noops doctor --deploy-ready` runs only the runtime prerequisites required before a deployment: Docker, Swarm, manager authority, the shared network, and the registry service.

`deploy` runs the deploy-readiness profile automatically and stops before applying a
stack when it finds a blocking failure.

## Commands

```text
noops [install]
noops version
noops uninstall [--purge]
noops doctor
noops doctor --deploy-ready
noops status
noops release <environment> <manifest>
noops deploy <environment> <manifest> [release-tag]
noops rollback <environment> <manifest>
noops secret set <environment> <key>
noops secret list <environment>
```

Running `noops` without a command runs `install`.

## Uninstall

`noops uninstall` removes the No Oops Ops-managed application stacks, registry
stack, shared network, generated state, and installation metadata. It does not remove
the CLI executable and preserves `dataDir`, including registry data, by default.

From a source checkout, the same conservative operation is available through Make:

```bash
make uninstall
```

This removes the repository-local `.bin/noops` build artifact after the runtime
teardown succeeds. It does not remove a separately installed CLI, such as
`~/.local/bin/noops`.

Use `--purge` only when persistent data should be deleted as well:

```bash
noops uninstall --purge

# From a source checkout:
make uninstall UNINSTALL_ARGS=--purge
```

Both commands are safe to retry: resources already removed are skipped. If a runtime
removal fails, installation metadata remains so the command can be run again. The
shared network is removed only when Docker permits it; a network still in use is left
in place and reported as an error.

## Secrets

Secrets are environment-scoped Docker Swarm secrets. `secret set` uses a hidden
terminal prompt or accepts a piped standard-input value, sends it directly to
Docker, and never stores it in `.noops`.

```bash
printf '%s' "$DATABASE_URL" | noops secret set prod DATABASE_URL
# Or enter the value at a hidden prompt:
noops secret set prod DATABASE_URL
noops secret list prod
```

Each update creates an immutable, versioned Swarm secret (for example,
`noops_prod_DATABASE_URL_v2`). `secret list` displays metadata only; there is no
command to retrieve a stored secret value. Reference a shared secret from an app
environment file with `from_secret`:

```yaml
- key: AUTH_SERVER_API_CLIENT_SECRET
  from_secret: AUTH_SERVER_API_CLIENT_SECRET
```

During deployment, No Oops Ops mounts the referenced Swarm secret at
`/run/secrets/AUTH_SERVER_API_CLIENT_SECRET` and supplies
`AUTH_SERVER_API_CLIENT_SECRET_FILE` with that path. Applications must read the
`*_FILE` variable; secret values are never written to `.env`.

## Configuration

Settings can be supplied as environment variables or in
`$XDG_CONFIG_HOME/noops/.env.noops` (normally `~/.config/noops/.env.noops`):

| Variable                | Default                     | Purpose                                 |
|-------------------------|-----------------------------|-----------------------------------------|
| `NOOPS_STATE_DIR`       | XDG state directory / `noops` | Directory for install and app artifacts |
| `NOOPS_NETWORK_NAME`    | `noops-net`                 | Shared Docker Swarm network             |
| `NOOPS_REGISTRY_NAME`   | `noops-registry`            | Internal registry service name          |
| `NOOPS_REGISTRY_PORT`   | `5000`                      | Internal registry port                  |

## App Manifest

Example:

```yaml
name: lango

source:
  context: /path/to/lango-service
  dockerfile: /path/to/lango-service/Dockerfile
  build:
    command:
      - mvn
      - package
      - -DskipTests

image:
  repository: lango-service

service:
  internal_port: 8080

healthcheck:
  test:
    - CMD
    - curl
    - -f
    - http://localhost:8080/lango/liveness
  start_period: 60s
  interval: 10s
  timeout: 10s
  retries: 3

env:
  file: lango.env.yml

rollout:
  parallelism: 1
  delay: 10s
  order: start-first
  failure_action: rollback
  restart_condition: on-failure
  restart_delay: 10s
  restart_max_attempts: 5
  restart_window: 70s
  readiness_timeout: 30s
  readiness_interval: 2s
```

Notes:

- `source.context` and `source.dockerfile` may be absolute paths or paths relative to the manifest file.
- `source.build.command` is optional.
- `rollout.readiness_timeout` and `rollout.readiness_interval` are `No Oops Ops` settings, not Docker Stack fields.
- The Docker stack image is resolved from release metadata, not directly from `image.repository`.

## Env File

Env values are authored as YAML and generated into Docker-compatible `.env` files.

Example:

```yaml
sections:
  - name: app
    items:
      - key: SERVER_PORT
        value: "8080"
      - key: SPRING_PROFILES_ACTIVE
        values:
          prod: prod

  - name: environment
    items:
      - key: ENVIRONMENT
        values:
          prod: prod
```

Resolution rules:

- if `values[environment]` exists, it wins
- otherwise `value` is used
- if neither exists, the key is omitted

## Docker Registry

The Docker registry endpoint used for builds and pushes is configured as:

```text
127.0.0.1:5000
```

For Docker Desktop, configure it as an insecure registry because the current registry is plain HTTP:

```json
{
  "insecure-registries": ["127.0.0.1:5000"]
}
```

Then restart Docker Desktop.

On Docker Desktop, a host process can occupy port `5000` even while the Swarm registry
service is healthy inside Docker's network. Use `noops doctor` to inspect the registry
service rather than treating a host-level `curl` request as a definitive health check.

## Current Limitations

- Router and TLS are not implemented yet.
- There is no dedicated release-list command yet.
- The internal registry currently uses an insecure local HTTP registry.
- App readiness checks Docker container health, not router-level HTTP availability.

## Direction

The next major areas are:

- release-list command
- registry cleanup and GC policy
- router/exposure
- richer deploy status and app lifecycle commands

## License

This project is licensed under the terms in [`LICENSE`](LICENSE).
