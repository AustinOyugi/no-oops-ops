# Compose-preserving deployment model

## Goal

Make No Oops Ops a minimal-intervention deployment layer for Docker Compose
and Docker Swarm configurations. Existing Compose YAML is the source of truth;
No Oops Ops must preserve it and change only the parts it owns.

This model is intended to make migrations from existing platforms, including
the Cranium infrastructure, mostly a matter of adding No Oops metadata rather
than rewriting working Compose files into a separate application format.

## Manifest model

`app.yml` uses the Docker Compose shape:

```yaml
services:
  api:
    image: registry.example.com/team/api:latest
    entrypoint: ./entrypoint.sh
    command: ["serve"]
    env_file: [.env]
    networks: [platform]
    deploy:
      replicas: 2

    x-noops:
      service:
        internal_port: 8080
      ingress:
        domains: [api.example.com]
```

Standard Compose and Swarm fields belong to the application owner. No Oops
Ops metadata lives in `x-noops` and is removed before the generated stack is
sent to Docker.

## No Oops-owned changes

When releasing or deploying a selected service, No Oops Ops may only:

- replace `image` with the recorded immutable release image;
- inject versioned Docker Swarm secrets and generated environment wiring;
- create or remove managed ingress routes and TLS configuration;
- record release/deployment history and wait for the configured health and
  rollout result;
- normalize paths that would otherwise break after the generated stack is
  written to No Oops state;
- upgrade settings required for secure, supported Docker Swarm deployment.

## Compose fields to preserve

No Oops Ops must retain fields it does not own, including:

- `entrypoint`, `command`, `environment`, `env_file`, and `labels`;
- `networks`, `volumes`, `configs`, and `secrets`;
- `deploy` placement, resources, restart policy, update policy, and rollback
  policy;
- service health checks, restart behavior, hostnames, and compatible ports;
- top-level networks, volumes, configs, and secrets.

This preservation must work for fields introduced by later compatible Compose
or Swarm versions without requiring a No Oops schema change.

## Explicit validation and upgrades

No Oops Ops must report each incompatible or security-relevant setting clearly.
It may upgrade or reject:

- local plain-HTTP registry references, replacing them with immutable images
  in the configured secure registry;
- plaintext credentials, which must move to managed secrets;
- `container_name`, which is incompatible with Swarm service scheduling;
- `depends_on` as a readiness guarantee, because Swarm does not honor it that
  way;
- public HTTP application ports when No Oops-managed nginx ingress owns the
  route;
- deployment settings that conflict with the selected No Oops rollout mode.

## Multi-service operation

One `app.yml` can declare multiple services. Lifecycle commands require an
explicit target:

```text
noops release <env> app.yml --service <name>
noops deploy <env> app.yml --service <name>
noops release <env> app.yml --all
noops deploy <env> app.yml --all
```

Each service has independent immutable releases, deployment history, rollback
state, secrets, and ingress ownership. `--all` uses a stable order and stops
at the first failure. Dependency declarations may order deployment work, but
applications remain responsible for runtime readiness and retries.

## Migration outcome

With this model, Cranium services can retain their existing Compose structure.
Migration consists of selecting a secure registry, adding `x-noops` metadata
for release/secret/ingress behavior, rotating exposed credentials and keys,
and deploying services incrementally. Stateful systems such as Neo4j, Redis,
and Directus still require backup, restore, and cutover plans.
