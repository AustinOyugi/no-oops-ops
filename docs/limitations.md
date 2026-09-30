# Current limitations

- The internal registry is a local, plain-HTTP registry and requires Docker insecure-registry configuration.
- Blue/green deployments (the default for exposed apps) currently reject manifests with named volumes, because
  concurrently running releases must not receive independent stack-scoped volume names.
- Registry garbage collection runs offline when automatic post-release retention or `noops cleanup --apply` removes registry images. The registry is
  temporarily unavailable while that collection runs.
- No Oops Ops manages a local Docker Swarm deployment platform; it is not a multi-host control plane.
- Nginx uses two replicas with serial start-first updates and automatic rollback. Install first prepares healthy
  replicas of the existing service, then waits for the updated replicas to be healthy. Shutdown allows active requests
  up to two minutes to finish; longer connections can be closed. Two replicas on one server do not protect against
  host failure, and overlapping tasks require capacity for a third Nginx container during updates. An older shell
  supervisor may still terminate existing connections during its first migration; subsequent updates use graceful
  signal forwarding.
- Docker Swarm determines health and automatic update rollback. No Oops Ops reports the final rollout outcome and task
  diagnostics.
- BuildKit dependency caches are local to the Docker builder. They can be shared between builds on that server using a
  common cache ID, but No Oops does not currently export or synchronize them between servers.
