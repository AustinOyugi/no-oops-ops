# Current limitations

- The internal registry is a local, plain-HTTP registry and requires Docker insecure-registry configuration.
- Blue/green deployments (the default for exposed apps) currently reject manifests with named volumes, because
  concurrently running releases must not receive independent stack-scoped volume names.
- There is no release-list command.
- Registry garbage collection runs offline when `noops cleanup --apply` removes registry images. The registry is
  temporarily unavailable while that collection runs.
- TLS certificate renewal currently mounts the Docker daemon socket into certbot so its renewal hook can force an nginx
  service reload. Access to that socket is effectively root-equivalent on the Docker host; this is accepted only for the
  trusted, single-node local platform and should be replaced by a host-side reload controller.
- No Oops Ops manages a local Docker Swarm deployment platform; it is not a multi-host control plane.
- Docker Swarm determines health and automatic update rollback. No Oops Ops reports the final rollout outcome and task
  diagnostics.
- BuildKit dependency caches are local to the Docker builder. They can be shared between builds on that server using a
  common cache ID, but No Oops does not currently export or synchronize them between servers.
