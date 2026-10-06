# Workspace storage

No Oops is workspace-based. Initialize a workspace with `noops init <directory>`.
After initialization, the CLI writes runtime files below the selected store,
which defaults to the workspace's `.noops/` directory. `init` may also create the workspace's initial `apps.yml`
catalog when it is absent. Use the workspace as the current directory or
provide it with `--workspace`.

```text
workspace/
  apps.yml
  apps/
  .noops/
    config.yml
    state/
    data/
```

Commit `apps.yml` and `apps/`; add `.noops/` to `.gitignore`.

## Separate installation stores

Use `settings.state` in the shared `apps.yml` to select a store for each
environment without duplicating manifests or environment files:

```yaml
settings:
  state:
    directory: .noops
    environments:
      dev: .noops-dev
      prod: /srv/rolengi/prod/.noops
      corp: .noops-corp
```

Each selected directory contains its own `config.yml`, `state/`, and `data/`.
This isolates installation metadata, release/deployment history, locks,
secret metadata, certificates, and registry data. Relative paths are resolved
from the source workspace, not the shell's working directory. Add your selected
local store directories to `.gitignore`; stores outside the workspace are not
part of the source checkout.

Select the environment for initialization, platform commands, and the dashboard:

`-e` is the shorthand for `--environment`.

Set `NOOPS_DEFAULT_ENV` to avoid repeating the flag for platform commands and
the dashboard:

```sh
export NOOPS_DEFAULT_ENV=dev
noops install
noops ui
```

An explicit `-e`/`--environment` or a positional command environment overrides
this default. An explicitly empty `-e ""` disables the environment default for
that command. Lifecycle commands retain their existing positional arguments.

```sh
noops -e dev init .
noops -e prod init .
noops -e dev install
noops -e prod ui
```

Lifecycle and secret commands already name their environment and automatically
use its configured store:

```sh
noops release prod lango --all
noops secret set dev DATABASE_PASSWORD
noops logs prod
```

`--state-dir <path>` overrides the catalog selection for every command, including
`init`, `install`, `ui`, cleanup, and uninstall. This option names the complete
runtime store, rather than just its `state/` child:

```sh
noops --state-dir /srv/rolengi/dev/.noops init .
noops --state-dir /srv/rolengi/dev/.noops --environment dev ui
```

The selection order is: `--state-dir`, the environment mapping, `directory`, then
`.noops`. A directory may contain `{environment}`; a command must select an
environment before that template can be expanded. A selected uninitialized
store produces an error and never falls back to another store. An explicit
`--environment` must agree with any positional command environment. Dashboard
child commands preserve the selected store and environment.

Existing state is not moved automatically. To preserve an installation when
changing its path, move its complete store (`config.yml`, `state/`, and `data/`)
while noops operations are stopped, then select the new directory. `init` creates
an empty store when it does not exist. Separate stores do not rename Docker
resources: installations sharing one Docker host also require distinct platform
network, registry, and ingress names and published ports. Separate servers can
reuse the same platform settings.

`init` writes an initial `apps.yml` when none exists. It is the source of
truth for platform settings and app aliases:

```yaml
version: 0.1.0

settings:
  upgrade:
    repository: AustinOyugi/no-oops-ops
  platform:
    network:
      name: noops-platform
    registry:
      name: noops-registry
      port: 5000
    ingress:
      name: noops-ingress
      http_port: 80
      https_port: 443
      # Trust Cloudflare's client-IP header only from Cloudflare networks.
      cloudflare: true
    networks:
      default: "noops-{environment}"
      environments:
        prod: noops-prod
        staging: noops-staging

apps:
  api:
    manifest: ./apps/api/app.yml
```

`upgrade.repository` selects the GitHub repository used to find and download No Oops releases. Use `owner/name`
format. It defaults to `AustinOyugi/no-oops-ops` when omitted, allowing forks to opt into their own release stream
without rebuilding the CLI.

`platform.network.name` is used only by No Oops platform services. Application
deployments use `platform.networks`: an explicit environment mapping wins, and
otherwise `{environment}` in `default` is replaced by the selected environment.
No Oops creates each environment's overlay network on demand, so applications
in `dev` and `prod` have independent networks. The managed nginx ingress is a
single shared public listener, but it joins an environment network only when it
serves an exposed app in that environment. It does not make application
networks reachable from one another.

Use distinct domains or path prefixes for routes in different environments. A
domain and path-prefix pair can have only one owner across the shared listener,
so the same public request cannot target both a dev and a prod app.

Set `platform.ingress.cloudflare: true` only when public ingress hostnames are
Cloudflare-proxied. No Oops then generates Nginx `set_real_ip_from` directives
for Cloudflare's published IPv4 and IPv6 proxy networks and uses
`CF-Connecting-IP` as the verified client address. This makes the generated
`X-Real-IP` and `X-Forwarded-For` headers contain the visitor address while
preventing direct callers from spoofing it. Keep the proxy enabled (orange
cloud) for every hostname served by that ingress. Cloudflare mode never uses
Let's Encrypt or prompts for an ACME email. Each HTTPS app must instead set
`x-noops.ingress.tls_certificate` to the name of a certificate imported with
`noops certificate import`.

The `version` value must exactly match `noops --version`. A release binary
creates a matching value during `noops init`; update the catalog in the same
change as the CLI when adopting a new No Oops release.

The workspace state directory contains paths such as:

```text
install.json
registry/config.yml
registry/stack.yml
nginx/stack.yml
apps/<app>/<environment>/.env
apps/<app>/<environment>/stack.yml
apps/<app>/<environment>/rollout.json
apps/<app>/<environment>/release.json
apps/<app>/<environment>/releases/<timestamp>.json
apps/<app>/<environment>/deployments/<timestamp>.json
secrets/<environment>/<key>/v<version>.json
```

Secret files contain metadata only; values are stored by Docker Swarm. `remove` deletes an app environment's generated
state but preserves its secrets and named volumes. `uninstall --purge` also deletes the configured persistent registry
data.

`apps/<app>/<environment>/rollout.json` contains best-effort blue/green display
telemetry for the TUI. It is separate from the durable deployment recovery journal;
missing display telemetry does not determine deployment success.
