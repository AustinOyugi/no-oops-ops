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

## Live service dashboard

Run `noops ui` in an initialized workspace, or use `noops --workspace <workspace> ui`.
The tview terminal dashboard lists managed Swarm services with environment,
app/deployment name, Docker service name, running/desired replicas, and state.
It includes registry and ingress services, plus app services identified by the
workspace's generated stack manifests, including blue/green candidates. Deployment history also identifies services whose generated stack files are gone.
All unmatched live Swarm services are shown as `untracked`, with environment `—`;
they support task inspection but offer only status/doctor in the service actions
menu. The global palette still permits explicitly selected catalog targets.
Undeployed catalog entries remain available through Release.

Use the up/down arrows to select a row and `q` or Ctrl-C to exit. The list refreshes
every five seconds; selection follows the service across refreshes. Docker errors
appear in the dashboard, retaining previous results until a successful refresh.
`running` means all desired replicas are running, not that application health has
been verified; zero desired replicas show `scaled down`. An interactive terminal
is required. Quitting restores the terminal and cancels pending Docker queries.

The dashboard also shows tasks for the selected service, including task ID, node,
state, running uptime, and errors. Retained stopped/failed tasks remain visible;
only running tasks have uptime, measured from their running-state timestamp.
Press `Tab` to switch focus between services and tasks, then use up/down to select.
The selected task's full ID and failure message appear below the task table.
Changing services cancels the previous task query; task errors retain previous
results for the same service. Tables scroll within their panes and resize with the terminal.

Press `e` to open commands for the selected service: platform status, doctor,
release list, release, deploy, rollback, and remove. The catalog resolves the app
alias and Compose service; ambiguous or missing mappings are rejected rather than
guessed. Platform services offer only status and doctor. Every command shows its
exact arguments with Cancel selected before execution.

Commands run through the current Noops executable with the explicit workspace and
service target, without a shell. The dashboard temporarily suspends so ordinary
CLI prompts and live output work normally. Press Enter after completion to return;
the dashboard reports success or failure and refreshes. Ctrl-C exits the dashboard
and cancels an active command; it does not roll back work already performed.

### Command palette

Press `:` anywhere in the dashboard to search all CLI operations. Type to filter,
use up/down to choose a result while typing, and Enter to open its form. Tab switches focus to the results.
Escape closes the palette or form. Commands include version, initialization,
installation, uninstall, status, doctor, upgrades, cleanup, app lifecycle operations,
secrets, and certificate import. The `ui` command itself is excluded.

Forms show a live argument preview and disable Run until inputs are valid.
App choices come from the catalog, and changing the app updates service choices.
All services disables individual service selection. Environment is an editable
field; the selected service supplies defaults when its catalog mapping is unique.
The `e` menu opens these same forms for service actions. Cleanup defaults to a dry
run; purge, apply, and skip-confirmation options default off. Secret set asks for
its value in the CLI hidden prompt, never in the form or preview. All commands
still pass through the existing CLI validation and confirmation prompts.

After upgrading, exit and restart the dashboard to load the new version. Init
creates the specified workspace; the dashboard remains attached to its original
workspace. Certificate paths are literal local paths (no shell expansion).

### Streaming command output

Noninteractive commands stream stdout and stderr into an output pane while the
service/task dashboard continues refreshing. Press `o` to focus output, or use
Tab to cycle through services, tasks, and output. Output is batched every 100 ms
and retains the most recent 256 KB. Use `p` in the output pane to pause/resume
auto-follow, arrow/PageUp/PageDown keys to scroll, and `x` to kill the active command while keeping its output open.
Use `X` (Shift+x) to kill the command and close its output pane. Escape returns focus to services. Completion shows success or failure
and the CLI exit error; the output pane closes automatically and restores the services/tasks layout.
Press `o` to reopen the retained output for review.
One command can run at a time per dashboard.

Secret set, installation, and interactive upgrades retain the terminal handoff.
Deploy/release-with-deploy also use handoff when a selected TLS service needs an
ACME email that has not been configured. Cancellation stops the command process
group; it does not reverse completed changes or guarantee remote Docker work
has stopped. Quitting the dashboard cancels the running command.

The services table includes AGE, measured from Docker's service creation timestamp,
separately from task uptime. Updating an existing service preserves its age;
recreating a service or creating a blue/green candidate starts a new service age.

Press `r` to release a new service. The form lists all apps and Compose services
from the current workspace catalog, including services not yet deployed to Swarm.
Choose the app, service, and environment; enable deploy to release and launch it.
A service must be declared in a catalog-referenced manifest first. You can also
open Release through `:`. New catalog entries are read whenever the form opens.

### Host health

A persistent strip above services samples CPU, RAM, disk capacity/free space,
load averages, uptime, and Docker connectivity every two seconds independently
of service refresh and command output. Overlays leave the strip visible. For a
local Linux Docker daemon with a matching hostname, disk usage covers Docker's
reported data directory. Missing readings show `—`; old snapshots are marked
STALE. When Docker is unavailable, metrics are explicitly labeled LOCAL HOST
and disk usage covers `/`.

Remote/VM Docker daemons display their hostname and connectivity, with resource
metrics marked unavailable. Local computer readings are never presented as remote
server usage; run the dashboard on the server for those metrics. These are host
metrics, not per-container limits or a whole-Swarm aggregate.

### Blue/green rollout progress

Blue/green deployments write display-only rollout telemetry separately from the
recovery journal. The dashboard polls it every second and displays old/new
services, release tags, latest replica counts, candidate readiness, promotion,
ingress reconciliation, and old-stack cleanup requests. Active rollouts are shown
regardless of whether the deployment was launched from this dashboard or another
CLI process sharing the workspace. Selecting an old/new service chooses its
rollout; otherwise the newest active rollout is shown.

Traffic switches in one ingress promotion, not percentage increments. During
promotion, traffic ownership is marked unresolved; after reconciliation, NEW is
shown as the configured ingress target. This is not an HTTP traffic probe.
Cleanup is requested asynchronously, so old service disappearance is reflected
by subsequent service snapshots. Failures show their last stage and error;
an unfinished record whose operation lock is released is marked interrupted.
Finished records remain visible for 90 seconds; interrupted records remain until
the next deployment updates them. Display writes are best-effort
and do not change deployment success or recovery behavior.
