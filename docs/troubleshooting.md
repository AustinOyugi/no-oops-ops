# Troubleshooting

Start with `noops ui`: press `:` and choose Doctor or Platform status. Select a
service to inspect its tasks and full error message. Press `o` to reopen the most
recent command output after its pane closes.

For a noninteractive session or when the dashboard cannot open, use the CLI:

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

Save the Git token as a normal secret in the same environment as the release. Enter only the provider token—not a URL or
credential-store entry:

```bash
noops secret set prod github-readonly
```

For GitHub, use a fine-grained token restricted to the repository with Contents read access. Confirm the manifest's
`x-noops.build.source.git.environments.<environment>.secret` key matches `github-readonly`. The source token is a
versioned Swarm secret and is available only to the temporary Git-fetch task.

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

## Navigating dashboard forms

Use Tab and Shift-Tab to move between fields and buttons. Space toggles checkboxes;
Enter opens a dropdown or activates the focused button. To start a command, Tab to
Run and press Enter. Mouse selection is also enabled. Escape closes the dialog.

## A live service appears as untracked

The dashboard lists all live Swarm services, even when this workspace does not own
them. Ownership comes from generated stacks, deployment history, or a unique match
between a Noops service name and the current catalog. Check that the relevant app
alias and Compose service are declared in this workspace's `apps.yml`. Duplicate
catalog identities cannot be resolved safely. Untracked rows still show tasks;
use `:` with an explicit catalog target or the CLI for lifecycle commands.
Temporary `noops-build-*` services are build workloads rather than deployable apps.

## Cancelled build leaves a temporary service

In the output pane, `x` cancels and preserves the logs; Shift+x cancels and closes
the pane. Both request graceful cleanup of the temporary Swarm service. A Docker
failure, forced termination, or an older Noops binary can leave an orphan behind.
Cleanup failures include the service name in a warning. Docker task removal can
also take a short time to appear in the dashboard.

To inspect an old leftover and remove it once you have confirmed it is no longer
an active build, use the exact name shown in the dashboard:

```bash
docker service ps --no-trunc <noops-build-service-name>
docker service logs --tail 100 <noops-build-service-name>
docker service rm <noops-build-service-name>
```

## Dashboard metrics are unavailable

Host CPU, memory, disk, load, and uptime are available for a local Linux Docker
daemon with a matching hostname. A remote daemon or Docker Desktop VM does not
expose those readings through this dashboard. Run `noops ui` on the server to see
its host metrics. `STALE` means a previous sample has not been refreshed.

## Blue/green transfer shows unresolved or interrupted

The rollout pane reports deployment stages and nginx's configured target, not
measured HTTP traffic percentages. Unresolved means promotion has begun but ingress
reconciliation has not been confirmed. Interrupted means the recorded operation no
longer holds its lock. Inspect services, tasks, ingress, and the recovery journal
before restarting a deployment; see [Concepts](concepts.md).
