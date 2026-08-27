# Command reference

```text
noops
noops version
noops --version
noops init <workspace>
noops [--workspace <workspace>] install
noops [--workspace <workspace>] uninstall [--purge]
noops [--workspace <workspace>] doctor [--deploy-ready]
noops [--workspace <workspace>] status
noops [--workspace <workspace>] release <environment> <app> (--service <name> | --all)
noops [--workspace <workspace>] deploy [--quick] <environment> <app> (--service <name> | --all)
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

| Command                 | Behavior                                                                                                                                                            |
|-------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `init <workspace>`      | Creates the workspace-local `.noops/state` and `.noops/data` stores, plus an initial version-matched `apps.yml` when absent.                                        |
| `install`               | Initializes Swarm when required, creates the shared network, deploys the registry and nginx ingress, waits for both to be ready, and records installation metadata. |
| `doctor`                | Checks Docker, Swarm, installation artifacts, network, and registry.                                                                                                |
| `doctor --deploy-ready` | Checks only the runtime prerequisites used by `deploy`.                                                                                                             |
| `status`                | Reports recorded installation metadata and component status, including registry/nginx task readiness; partially running services are reported as degraded.          |
| `uninstall`             | Removes managed app stacks, registry stack, shared network when Docker allows it, generated state, and installation metadata. It keeps persistent registry data.    |
| `uninstall --purge`     | Performs uninstall and removes persistent registry data.                                                                                                            |

`uninstall` does not remove the installed CLI executable. `make uninstall` additionally removes the repository-local
`.bin/noops` after teardown succeeds. `uninstall --purge` removes only the workspace `.noops/state` and `.noops/data`
directories.

## Application commands

| Command                                                    | Behavior                                                                                                                                                                                                                                                                                                       |
|------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `release <env> <app> (--service <name> \| --all)`          | Builds selected services or snapshots image-only services, pushes immutable images, and records release metadata. Git-backed builds fetch into a temporary workspace and run the Dockerfile's build stages; no language runtime is installed on the host.                                                      |
| `deploy [--quick] <env> <app> (--service <name> \| --all)` | Deploys selected services using their latest recorded releases. `--quick` uses the health-check start period as its monitor window. The normal rollout convergence timeout defaults to four minutes unless the manifest overrides it.                                                                          |
| `rollback <env> <app> (--service <name> \| --all)`         | Redeploys each selected service's previous successful deployment, including pinned secret versions.                                                                                                                                                                                                            |
| `remove <env> <app> (--service <name> \| --all)`           | Removes selected app stacks and generated state. Named volumes and environment secrets are preserved.                                                                                                                                                                                                          |
| `cleanup [--apply] [--orphaned] [--keep <count>]`          | Plans retention cleanup across releases, deployments, and registry images. `--orphaned` selects app environments with no actually running recorded service, overriding normal retention. Dry-run is the default; `--apply` deletes selected manifests and metadata, then runs offline registry GC when needed. |

## Secret commands

`secret set` reads a value from a hidden terminal prompt or standard input:

```bash
printf '%s' "$DATABASE_URL" | noops secret set prod DATABASE_URL
noops secret list prod
```

Each update creates a versioned Swarm secret such as `noops_prod_DATABASE_URL_v2`. `secret list` shows metadata, not
values. `secret delete` removes every version of a key and its local metadata. Docker refuses to remove a secret that
is currently referenced by a service; deploy a replacement first, then delete the old key.

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
