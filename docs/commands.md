# Command reference

```text
noops
noops version
noops --version
noops init <workspace>
noops [--workspace <workspace>] upgrade --check
noops [--workspace <workspace>] upgrade --to <vMAJOR.MINOR.PATCH|latest> [--dry-run] [--yes]
noops [--workspace <workspace>] install
noops [--workspace <workspace>] uninstall [--purge]
noops [--workspace <workspace>] doctor [--deploy-ready]
noops [--workspace <workspace>] status
noops [--workspace <workspace>] ui
noops [--workspace <workspace>] logs <environment> <app> [--service <name>] [--tail <count|all>] [--since <timestamp|duration>] [--timestamps] [--follow=false]
noops [--workspace <workspace>] release [--deploy] <environment> <app> [--service <name> | --all]
noops [--workspace <workspace>] release list <environment> <app> [--service <name> | --all]
noops [--workspace <workspace>] deploy [--quick] <environment> <app> [--service <name> | --all]
noops [--workspace <workspace>] rollback <environment> <app> (--service <name> | --all)
noops [--workspace <workspace>] remove <environment> <app> (--service <name> | --all)
noops [--workspace <workspace>] secret set <environment> <key>
noops [--workspace <workspace>] secret delete <environment> <key>
noops [--workspace <workspace>] secret list <environment>
noops [--workspace <workspace>] certificate import <name> <certificate.pem> <private-key.pem>
noops [--workspace <workspace>] cleanup [--apply] [--orphaned] [--keep <count>]
```

Running `noops` without arguments prints its name and build version. `version`, `--version`, and `-v` print the same
version without loading a workspace. Every other command runs in the current directory's workspace unless
`--workspace <workspace>` is supplied.

## Platform commands

| Command                 | Behavior                                                                                                                                                                                                                                          |
|-------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `init <workspace>`      | Creates the workspace-local `.noops/state` and `.noops/data` stores, plus an initial version-matched `apps.yml` when absent.                                                                                                                      |
| `upgrade --check`       | Fetches and displays the latest release tag from `settings.upgrade.repository` without changing the CLI or workspace.                                                                                                                             |
| `upgrade --to <tag>`    | Shows the current and target versions, asks for confirmation, verifies the release checksum and binary, adopts the version in the current catalog, and atomically replaces the CLI executable. Use `--yes` only for explicitly pinned automation. |
| `install`               | Initializes Swarm when required, creates the shared network, deploys the registry and nginx ingress, waits for both services to be ready, and records installation metadata.                                                                      |
| `doctor`                | Checks Docker, Swarm, installation artifacts, network, and registry.                                                                                                                                                                              |
| `doctor --deploy-ready` | Checks only the runtime prerequisites used by `deploy`.                                                                                                                                                                                           |
| `status`                | Reports recorded installation metadata and component status, including registry/nginx task readiness; partially running services are reported as degraded.                                                                                        |
| `uninstall`             | Removes managed app stacks, registry stack, shared network when Docker allows it, generated state, and installation metadata. It keeps persistent registry data.                                                                                  |
| `uninstall --purge`     | Performs uninstall and removes persistent registry data.                                                                                                                                                                                          |

Bare `noops upgrade` never changes the executable: pass `--to` or `--check`. `--to latest` resolves the exact tag and
shows it in the confirmation prompt before downloading. Downgrades require `--allow-downgrade`. The command validates
the downloaded binary before changing `apps.yml`; if executable replacement fails, it restores the previous catalog
version. Non-interactive `--yes` upgrades require an exact tag and cannot use `--to latest`. After a successful CLI
upgrade, run `noops install` to reconcile managed platform services.

`uninstall` does not remove the installed CLI executable. `make uninstall` additionally removes the repository-local
`.bin/noops` after teardown succeeds. `uninstall --purge` removes only the workspace `.noops/state` and `.noops/data`
directories.

## Application commands

| Command                                                      | Behavior                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
|--------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `release [--deploy] <env> <app> [--service <name> \| --all]` | Builds selected services or snapshots image-only services, pushes immutable images, and records release metadata. `--deploy` then deploys the exact releases created by the command, excluding services with `x-noops.deploy: false`. When the manifest has exactly one service, omit service selection; multi-service manifests require `--service` or `--all`. Git-backed builds fetch into a temporary workspace and run the Dockerfile's build stages; no language runtime is installed on the host.                                                                                                                                                                                                                                                                                                             |
| `release list <env> <app> [--service <name> \| --all]`       | Lists recorded releases for selected services, newest first. When the manifest has exactly one service, omit service selection.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `deploy [--quick] <env> <app> [--service <name> \| --all]`   | Deploys selected services using their latest recorded releases. If none exists, it creates one first. When the manifest has exactly one service, omit service selection; multi-service manifests require `--service` or `--all`. `--quick` uses the health-check start period as its monitor window. The normal rollout convergence timeout defaults to two minutes unless the manifest overrides it.                                                                                                                                                                                                                                                                                                                                                                                                                |
| `rollback <env> <app> (--service <name> \| --all)`           | Redeploys each selected service's previous successful deployment, including pinned secret versions.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| `remove <env> <app> (--service <name> \| --all)`             | Removes selected app stacks and generated state. Named volumes and environment secrets are preserved.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| `cleanup [--apply] [--orphaned] [--keep <count>]`            | Plans retention cleanup across releases, deployments, registry images, and local image tags. Swarm service specs and non-terminal tasks protect their image references, including both sides of an active rollout. The registry is enumerated as an additional reconciliation source, so unrecorded images are also eligible only when no protected Swarm or retention digest reaches them. The default retains the three newest release and successful-deployment records. `--orphaned` removes stale No Oops history for environments whose recorded service is absent; it does not by itself make an image eligible for deletion. Dry-run is the default; `--apply` deletes selected manifests and metadata, then runs offline registry GC whenever it prunes cleanup state, allowing interrupted runs to converge. |

## Secret commands

`secret set` reads a value from a hidden terminal prompt or standard input:

```bash
printf '%s' "$DATABASE_URL" | noops secret set prod DATABASE_URL
noops secret list prod
```

Each update creates a versioned Swarm secret such as `noops_prod_DATABASE_URL_v2`. `secret list` shows metadata, not
values. `secret delete` removes every version of a key and its local metadata. Docker refuses to remove a secret that
is currently referenced by a service; deploy a replacement first, then delete the old key.

## Automatic build retention

After each successfully pushed and recorded release, No Oops runs cleanup for that service and environment, keeping the
three newest builds. This includes image-only releases and releases automatically created by `deploy`. Older builds
still used by a Swarm service, an active rollout, or Swarm's rollback specification remain protected, so a running
service can temporarily require more than three retained builds. Other apps and environments are untouched.

Cleanup removes expired release/deployment records, unreferenced generated blue/green stack manifests, registry manifests, and local build tags. It does not remove live Docker stacks; deploy reconciliation owns that operation. Registry garbage collection
briefly stops the registry; cleanup restarts it and waits for its API before continuing to `--deploy`. Cleanup failures
are logged as warnings and do not turn an already published release into a failed release. A later successful release
or `noops cleanup --apply --keep 3` can retry cleanup.

## Git build-source secrets

Private Git sources use the normal versioned Swarm-secret command. Only a secret's name and version are recorded
locally. No Oops mounts the token read-only at `/run/secrets/git-token` in a short-lived Git-fetch Swarm task, then
removes that task. For GitHub, use a fine-grained personal access token with read access to the repository's contents.

```bash
printf '%s' 'github_pat_YOUR_TOKEN' | noops secret set prod github-readonly
```

The configured secret key is referenced by `x-noops.build.source.git.environments.<environment>.secret`.
Public Git sources omit it.

## Certificate commands

`certificate import <name> <certificate.pem> <private-key.pem>` imports a supplied TLS certificate for nginx routes that
use `x-noops.ingress.tls_certificate`. It is intended for trusted origin certificates such as Cloudflare Origin CA
certificates. When `settings.platform.ingress.cloudflare: true`, imported certificates are required for every HTTPS
route; No Oops does not request an ACME email or issue Let's Encrypt certificates in that mode.

## Terminal dashboard (preferred)

Run `noops ui` for daily service inspection, releases, deployment actions, streaming
command output, host metrics, and blue/green progress. See the [TUI guide](tui.md)
for navigation, forms, cancellation, and CLI fallback. The command reference above
also applies to the operations launched from the dashboard.

## Service logs

`noops logs prod redis` streams the deployed service's stdout and stderr, starting
with the last 100 lines per task. Press Ctrl+C to stop. For multi-service apps,
select a service with `--service`; single-service apps select it automatically.
The command resolves the active service from successful deployment history, including
blue/green stack names, and falls back to the standard stack name when no history exists.

```bash
noops logs prod backend --service api
noops logs prod backend --service api --tail 200 --since 10m --timestamps
noops logs prod redis --follow=false --tail all
```

Logs come from `docker service logs` on the configured Docker Swarm manager.
The service must use a logging driver supported by Docker service logs, such as
`json-file` or `journald`. A running stream stays attached to the service selected
when the command starts; restart it after a blue/green switch to follow the new service.

In `noops ui`, select a service and press `e`, then choose **Logs**. Logs are also
available from the `:` command palette. The live stream appears in the output
pane while the tasks section and task details are hidden. Tab switches between
services and logs. Press `p` to pause/follow scrolling, `x` to stop and keep the
logs, or `X` to stop and close the output pane. Tasks return when streaming ends.
