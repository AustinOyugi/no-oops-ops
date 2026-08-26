# Troubleshooting

Run the diagnostic commands first:

```bash
noops doctor
noops doctor --deploy-ready
noops status
```

`status` reports the registry and nginx task counts. A `degraded` service has fewer running tasks than Docker Swarm
desires; use the task diagnostic in its status message to investigate the failed task.

## Registry errors during release

Confirm Docker allows `127.0.0.1:5000` as an insecure registry and restart Docker after changing that setting. The
registry is plain HTTP.

On Docker Desktop, a host process can occupy port 5000 while the registry is healthy inside Docker's network. Use
`noops doctor` rather than a host-level `curl` request as the registry health check.

## Private Git source cannot be fetched

Save a source credential in the same environment as the release. Enter only the provider token—not a URL or
credential-store entry:

```bash
noops source credential set prod github-readonly
```

For GitHub, use a fine-grained token restricted to the repository with Contents read access. Confirm the manifest's
`credential` key matches `github-readonly`. The source token is a versioned Swarm secret and is available only to the
temporary Git-fetch task.

## Deploy fails before a stack is applied

`deploy` runs the deploy-readiness profile. Resolve the failed remediation reported by the command: Docker must be
running, Swarm must be active, the current node must be a manager, and the shared network and registry must be
available.

## A referenced secret cannot be resolved

Create it in the same environment and verify its name:

```bash
noops secret list prod
noops secret set prod DATABASE_URL
```

The app manifest's `resolvable` value is the environment variable key; the environment file's `from_secret` value is the
stored secret key.

## Rollback is unavailable

There must be at least two successful recorded deployments for the same app and environment. A release alone does not
create rollback history.

## Registry disk usage remains after remove

`remove` deletes registry manifests, making layers eligible for garbage collection. It does not run registry garbage
collection; reclaiming disk space requires a separate GC operation while the registry is stopped.
