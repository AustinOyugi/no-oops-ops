# Concepts

## Platform installation

`noops install` prepares one local Swarm deployment platform: the `noops-net` overlay network and the `noops-registry` internal registry, unless overridden by configuration. It is safe to rerun.

## Releases

A release is an immutable image tagged with a UTC timestamp (`YYYYMMDD-HHMMSS`) and pushed to the internal registry. For a source build, No Oops Ops optionally runs `source.build.command` and then builds the specified Dockerfile. For `image.build: false`, it snapshots `image.repository:image.tag` into the internal registry.

Release metadata is retained per app and environment. A release does not change a running service.

## Deployments and rollouts

A deployment renders a Docker Stack using a recorded release. It first runs deploy-readiness checks, then waits for Swarm to converge and records the outcome and the secret versions used.

Docker Swarm controls scheduling, health checks, task replacement, and automatic rollback. No Oops Ops supplies the rendered update/rollback policy and reports the result; it does not run a competing health controller.

For exposed apps, No Oops Ops uses blue/green promotion by default for a new release: it creates a release-specific candidate service, waits for Docker Swarm to report its existing Docker health check as converged, then changes nginx to target the candidate before removing the previous active service. Set `expose.blue_green: false` to use an in-place Swarm update instead. If the candidate fails, only that candidate is removed. Direct service-to-service callers should use the stable nginx internal URL rather than a release-specific application service name.

## Rollback

`noops rollback` deploys the preceding successful deployment for the app and environment. It pins the secret versions recorded by that deployment. At least two successful deployments are required.

## Secrets

Secrets are immutable, versioned Docker Swarm secrets scoped to an environment. `secret set` creates a new version and stores metadata only; secret values are never saved in No Oops Ops state or retrievable through the CLI.

With `env.secrets.resolution: file`, the app receives `<KEY>_FILE=/run/secrets/<KEY>`. With `resolution: env`, No Oops Ops builds a small wrapper image that reads the mounted secret and exports `<KEY>` before starting the original image command. Prefer `file` when the application supports it.

## State

No Oops Ops uses the XDG state directory by default (`$XDG_STATE_HOME/noops`) and the XDG data directory for persistent registry data. Paths and retention are described in [Configuration and generated state](reference/configuration.md).
