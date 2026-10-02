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

- The TUI requires an interactive terminal. Use the [CLI](commands.md) for automation
  and noninteractive sessions. Each dashboard runs one command at a time.
- The output pane retains the latest 256 KB of command output, not a durable log
  archive or a general service-log viewer. Interactive prompts use terminal handoff.
- Host metrics cover the local Linux Docker host, not individual containers or the
  entire Swarm. Remote/VM host resource metrics are unavailable.
- Blue/green progress shows readiness and configured ingress ownership; it does not
  measure HTTP traffic or implement percentage-based traffic shifting.
- Cancellation requests cleanup of temporary build/fetch services, but daemon
  outages or forced process termination can leave workloads requiring manual removal.
