# No Oops Ops

`No Oops Ops` is a lightweight self-hosted deployment manager for Docker-based applications. It is designed for people
who have a server and want safe, repeatable deployments without assembling a full DevOps platform.

Install on a server, point it at a repo or image, and get safe deployments with built-in health checks, rollback,
domains, TLS, and a private registry.

## What the project is for

`No Oops Ops` is aimed at:

- solo founders running production apps on one server
- small engineering teams with a few services
- agencies deploying similar customer stacks
- internal teams that want simple operations without platform-team overhead

It is intentionally opinionated:

- Docker-first
- single-server first, small-fleet later
- interactive CLI-first
- safe rollouts by default
- minimal persistent state and low operational footprint

## Core v1

The v1 design is centered on a narrow but high-value workflow:

- bootstrap a server quickly
- deploy services safely from Git or prebuilt images
- perform rolling updates with health checks
- roll back automatically on failure
- manage domains, TLS, and an internal registry

The primary operator experience is an interactive terminal interface, so users do not need to memorize a large command
set before becoming productive.

## v1 capabilities

- server bootstrap with Docker runtime, reverse proxy, internal registry, and local state store
- app deployment from Git repositories or prebuilt images
- release tracking with concrete image tags and rollout history
- start-first rolling updates with health-gated completion
- automatic and manual rollback flows
- environment-aware app configuration and secret references
- HTTP routing, TLS issuance and renewal, and direct port exposure when needed
- bundled private registry with retention and cleanup rules

## Architecture direction

- a background control service
- a CLI client with interactive and explicit command modes
- SQLite for v1 state storage
- a dynamic reverse proxy such as Traefik or Caddy
- a managed internal Docker registry
- Docker Swarm as the recommended v1 runtime for rolling updates and rollback behavior

## License

This project is licensed under the terms in [
`LICENSE`](/Users/odu/Documents/alien/code-innate/personal/no-oops-ops/LICENSE).
