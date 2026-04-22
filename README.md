# No Oops Ops

`No Oops Ops` is a lightweight self-hosted deployment manager for Docker-based applications.

The current implementation focuses on a practical local/server workflow:

- bootstrap Docker Swarm and an internal registry
- build and publish immutable app images
- generate deployment artifacts from manifests
- deploy apps with Docker Stack
- wait for service readiness and surface useful diagnostics

The goal is to keep simple deployments repeatable without building a full DevOps platform.

## Current Workflow

The current happy path is:

```bash
go run ./cmd/noops install
go run ./cmd/noops release prod examples/lango.app.yml
go run ./cmd/noops deploy prod examples/lango.app.yml
```

You can also inspect the platform:

```bash
go run ./cmd/noops doctor
go run ./cmd/noops status
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
go run ./cmd/noops install
```

The install state is written under `.noops/`.

## Release

Release builds and publishes an immutable image for an app/environment.

Run:

```bash
go run ./cmd/noops release prod examples/lango.app.yml
```

Release currently does:

- loads the app manifest
- runs the optional pre-build command from `source.build.command`
- builds the Docker image
- tags it with a generated timestamp tag
- tags it for the internal registry
- pushes it to the internal registry
- writes release metadata

Example generated metadata:

```text
.noops/apps/lango/prod/release.json
```

Deploy reads this file later, so the manifest does not need to be manually updated with a new image tag on every release.

## Deploy

Deploy consumes the latest release metadata for an app/environment.

Run:

```bash
go run ./cmd/noops deploy prod examples/lango.app.yml
```

Deploy currently does:

- loads the app manifest
- loads the referenced env YAML file
- resolves environment-specific env values
- writes `.env`
- writes `stack.yml`
- reads `release.json`
- renders the stack with the released registry image
- runs `docker stack deploy`
- verifies the Swarm service exists
- waits for running tasks
- prints task diagnostics on readiness timeout

Generated app artifacts are written under:

```text
.noops/apps/<app>/<environment>/
```

For example:

```text
.noops/apps/lango/prod/.env
.noops/apps/lango/prod/stack.yml
.noops/apps/lango/prod/release.json
```

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

The internal registry is currently exposed at:

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

You can verify the registry with:

```bash
curl http://127.0.0.1:5000/v2/
```

## Current Limitations

- Router and TLS are not implemented yet.
- Release history and rollback commands are not implemented yet.
- Deploy consumes the latest `release.json` for the selected app/environment.
- The internal registry currently uses an insecure local HTTP registry.
- App readiness currently checks Swarm running tasks, not router-level HTTP availability.

## Direction

The next major areas are:

- release history
- rollback
- registry cleanup and GC policy
- router/exposure
- richer deploy status and app lifecycle commands

## License

This project is licensed under the terms in [`LICENSE`](/Users/odu/Documents/alien/code-innate/personal/no-oops-ops/LICENSE).
