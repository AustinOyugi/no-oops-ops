# Current limitations

- The internal registry is a local, plain-HTTP registry and requires Docker insecure-registry configuration.
- The platform installs nginx ingress on ports 80 and 443, but route generation, TLS, and public service exposure are not implemented yet.
- There is no release-list command.
- Registry garbage collection is manual.
- No Oops Ops manages a local Docker Swarm deployment platform; it is not a multi-host control plane.
- Docker Swarm determines health and automatic update rollback. No Oops Ops reports the final rollout outcome and task diagnostics.
