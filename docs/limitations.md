# Current limitations

- The internal registry is a local, plain-HTTP registry and requires Docker insecure-registry configuration.
- Router, TLS, and public service exposure are not implemented.
- There is no release-list command.
- Registry garbage collection is manual.
- No Oops Ops manages a local Docker Swarm deployment platform; it is not a multi-host control plane.
- Docker Swarm determines health and automatic update rollback. No Oops Ops reports the final rollout outcome and task diagnostics.
