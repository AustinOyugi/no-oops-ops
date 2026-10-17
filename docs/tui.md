# Terminal dashboard

Use `noops ui` as the preferred interface for daily operations. The CLI remains
available for scripts, automation, noninteractive sessions, and explicit recovery.
The TUI uses the same commands and validations; it does not require a separate server.

```bash
cd /srv/noops/workspace
noops ui
# From another directory:
noops --workspace /srv/noops/workspace ui
```

## Choose a workspace and environment

Initialize the workspace and add app aliases to `apps.yml` before opening the
TUI. Docker must be reachable and support Swarm service queries. Both stdin and
stdout must be interactive terminals; use the [CLI](commands.md) in scripts or
when piping output.

```bash
noops --workspace /srv/noops/workspace --environment prod ui
# Override the configured runtime store:
noops --workspace /srv/noops/workspace --state-dir /srv/noops/prod ui
```

`--environment` selects the installation mapping in
`settings.state.environments`; `NOOPS_DEFAULT_ENV` supplies its default.
`--state-dir` overrides the store path. Actions launched by the dashboard retain
its workspace and selected store, and pass the form environment with
`--environment` rather than a positional argument. When an environment is selected at startup,
command forms reject a different environment; open another dashboard to operate
that environment. See [Separate installation stores](reference/configuration.md#separate-installation-stores).

## First release

1. Add the app alias and manifest path to `apps.yml`. Declare the service in that
   manifest and create any required secrets.
2. Open `noops ui` and press `r`. A service does not need a running Swarm task to
   appear in the release form.
3. Choose **Environment**, **App**, and **Service**. Enable **All services** to
   process the app's services in dependency order, or select one service.
4. Enable **Deploy after release** to launch the release after it is built.
5. Review the command preview, select **Run**, then confirm **Run** in the
   confirmation dialog.
6. Follow the output and rollout panes. After completion, press `o` to review
   retained output, or select the service and press `e` for subsequent actions.

For an existing release, use **Deploy** from `e` or `:`. **Quick rollout** uses
its healthcheck start period as the monitoring window for this deployment;
see [rollout settings](reference/app-manifest.md#rollout-defaults).

## Navigation

| Key          | Action                                                        |
|--------------|---------------------------------------------------------------|
| Up / Down    | Select a service, task, or palette result in the focused pane |
| Tab          | Cycle services, tasks, and output when visible                |
| `:`          | Search the command palette                                    |
| `e`          | Open actions for the selected service                         |
| `r`          | Release a catalog service, including one not yet running      |
| `o`          | Open or focus retained command output                         |
| `q` / Ctrl-C | Quit the dashboard and cancel its active command              |

In forms, use Tab to move to the next field or button and Shift-Tab to move back.
Press Enter to open a dropdown, use arrows to choose, and Enter to accept. Space
toggles a checkbox. To execute, Tab to **Run** and press Enter, or click Run with
the mouse. Escape closes the form or palette. Confirmation defaults to Cancel.

To deploy a new service, declare it in a catalog-referenced Compose manifest, press
`r`, choose its app, service, and environment, and enable **Deploy after release**.
For an existing service, select its row and press `e` to open its action form.

## Services and tasks

Run `noops ui` in an initialized workspace, or use `noops --workspace <workspace> ui`.
The terminal dashboard lists managed Swarm services with environment,
app/deployment name, Docker service name, running/desired replicas, and state.
It includes registry and ingress services, plus app services identified by the
workspace's generated stack manifests, including blue/green candidates. Deployment history also identifies services
whose generated stack files are gone.
Older Noops service names can be matched to a unique catalog service when local
records are missing; both stable and release-specific blue/green names are supported.
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
service logs, environment logs, release list, release, deploy, rollback, and remove. Services with
`x-noops.deploy: false` offer release operations without deploy, rollback, or remove. The catalog resolves the app
alias and Compose service; ambiguous or missing mappings are rejected rather than
guessed. Platform services offer only status and doctor. Every command shows its
exact arguments with Cancel selected before execution.

Commands run through the current Noops executable with the explicit workspace and
service target, without a shell. Noninteractive commands stream into the dashboard;
commands that need terminal input temporarily suspend it. After a terminal handoff,
press Enter to return. Quitting cancels an active command; it does not roll back
work already performed.

### Command palette

Press `:` in the main dashboard to search the available operations. Type to filter,
use up/down to choose a result while typing, and Enter to open its form. Tab switches focus to the results.
Escape closes the palette or form. Commands include version, initialization,
installation, uninstall, status, doctor, upgrades, cleanup, app lifecycle operations,
secrets, service and environment logs, and certificate import. Config install/upgrade
and release options such as `--force` are currently available through the
[CLI](commands.md); the palette does not expose every CLI option.

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
Use `X` (Shift+x) to kill the command and close its output pane. Escape returns focus to services. Completion shows
success or failure
and the CLI exit error; the output pane closes automatically and restores the services/tasks layout.
Press `o` to reopen the retained output for review.
One command can run at a time per dashboard.

Secret set, installation, and interactive upgrades retain the terminal handoff.
Deploy/release-with-deploy also use handoff when a selected TLS service needs an
ACME email that has not been configured. Cancellation first sends SIGTERM to Noops,
allowing its cancelled context to remove the temporary build or private Git-fetch
Swarm service. Each removal has a ten-second timeout and reports failures as warnings.
A command that cannot exit gracefully is forcibly stopped after thirty seconds.
Docker removes tasks asynchronously; completed deployment changes are not reversed.
Quitting the dashboard also cancels the running command. Old orphaned build services
are not removed automatically by this fix; see [Troubleshooting](troubleshooting.md).

The services table includes AGE, measured from Docker's service creation timestamp,
separately from task uptime. Updating an existing service preserves its age;
recreating a service or creating a blue/green candidate starts a new service age.

Press `r` to release a new service. The form lists all apps and Compose services
from the current workspace catalog, including services not yet deployed to Swarm.
Choose the app, service, and environment; enable deploy to release and launch it.
A service must be declared in a catalog-referenced manifest first. You can also
open Release through `:`. New catalog entries are read whenever the form opens.

### Service and environment logs

Select a managed application service, press `e`, and choose **Logs** for that
service or **Environment logs** for all deployed Noops services in its
environment. Both commands are also available through `:` with explicit target
fields. Environment logs include catalog-managed deployments and their active
blue/green service names; unrelated Swarm services are excluded.

While logs stream, task and task-detail panes are hidden. Tab switches between
services and output. With output focused:

| Key | Action |
|-----|--------|
| `p` | Pause or resume automatic scrolling; collection continues |
| Arrow / PageUp / PageDown | Scroll retained output |
| `x` | Stop the stream and keep output open |
| `X` (Shift+x) | Stop the stream and close output |
| Escape | Return focus to services without stopping the stream |

Tasks return after the stream ends. Only one command or log stream runs at a
time. Output retains the latest 256 KB and can be reopened with `o` until another
command replaces it; use CLI redirection when you need a saved log file.

Log targets are resolved when streaming starts. Restart after a blue/green
promotion or after adding services to refresh the targets. Docker must use a
logging driver supported by `docker service logs`. Use the CLI for log options
such as `--tail`, `--since`, `--timestamps`, or `--follow=false`; see
[Service logs](commands.md#service-logs) and [Environment logs](commands.md#environment-logs).

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
