# Current limitations

- The internal registry is a local, plain-HTTP registry and requires Docker insecure-registry configuration.
- Blue/green deployments (the default for exposed apps) currently reject manifests with named volumes, because concurrently running releases must not receive independent stack-scoped volume names.
- There is no release-list command.
- Registry garbage collection is manual.
- No Oops Ops manages a local Docker Swarm deployment platform; it is not a multi-host control plane.
- Docker Swarm determines health and automatic update rollback. No Oops Ops reports the final rollout outcome and task diagnostics.
