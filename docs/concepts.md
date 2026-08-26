# Concepts

## Platform installation

`noops install` prepares one local Swarm deployment platform: the `noops-platform` overlay network, the
`noops-registry` internal registry, and the `noops-nginx` ingress, unless overridden by the workspace catalog. It is
safe to rerun and waits for the registry and ingress services to become ready.

## Releases

A release is an immutable image tagged with a UTC timestamp (`YYYYMMDD-HHMMSS`) and pushed to the internal registry. A
Compose service with `build` is built from its declared context and Dockerfile; an image-only service is snapshotted
into the internal registry. A digest image reference is also accepted as an upstream source.

Release metadata is retained per app and environment. A release does not change a running service.

### Git-backed container builds

A service can declare `x-noops.build.source.git` to build from a repository rather than from a checkout on the server.
No Oops creates a temporary workspace under `.noops/data`, starts a one-shot Swarm Git-fetch task, and removes both the
task and workspace after Docker has built the image. For a private repository, the Git token is a versioned Swarm secret
mounted only at `/run/secrets/git-token` in that one-shot task; No Oops stores secret metadata, never its value.

The Dockerfile owns the build toolchain. For example, a Java service uses a Maven build stage and can share a BuildKit
cache mount such as `id=noops-maven-repository,target=/root/.m2,sharing=locked`. Caches are local to the Docker builder:
they reuse dependencies between builds but do not contain application source or become part of the runtime image.

## Deployments and rollouts

A deployment renders a Docker Stack using a recorded release. It first runs deploy-readiness checks, then waits for
Swarm to converge and records the outcome and the secret versions used.

Docker Swarm controls scheduling, health checks, task replacement, and automatic rollback. No Oops preserves the
service's existing Compose deployment policy, waits for the configured result, and does not run a competing health
controller. The default convergence timeout is two minutes and can be overridden with `rollout.convergence_timeout`.

For exposed apps, No Oops Ops uses blue/green promotion by default after the first deployment: it creates a
deployment-specific candidate service, waits for Docker Swarm to report its existing Docker health check as converged,
then changes nginx to target the candidate before removing the previous active service. This happens even when the
release tag is unchanged. Blue/green is limited to stateless services: manifests with named volumes are rejected because
candidate and active releases must not receive independent stack-scoped state. Set `expose.blue_green: false` to use an
in-place Swarm update for stateful services. If the candidate fails, only that candidate is removed. Direct
service-to-service callers should use the stable nginx internal URL rather than a release-specific application service
name.

## Rollback

`noops rollback` deploys the preceding successful deployment for the app and environment. It pins the secret versions
recorded by that deployment. At least two successful deployments are required.

## Secrets

Secrets are immutable, versioned Docker Swarm secrets scoped to an environment. `secret set` creates a new version and
stores metadata only; secret values are never saved in No Oops Ops state or retrievable through the CLI.

With `env.secrets.resolution: file`, the app receives `<KEY>_FILE=/run/secrets/<KEY>`. With `resolution: env`, No Oops
Ops builds a small wrapper image that reads the mounted secret and exports `<KEY>` before starting the original image
command. Prefer `file` when the application supports it.

## State

No Oops Ops uses workspace-local state and persistent registry data below `.noops/`. Paths and retention are described
in [Configuration and generated state](reference/configuration.md).
