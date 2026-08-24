# Command reference

```text
noops
noops init <workspace>
noops [--workspace <workspace>] install
noops uninstall [--purge]
noops doctor [--deploy-ready]
noops status
noops [--workspace <workspace>] release <environment> <app> (--service <name> | --all)
noops [--workspace <workspace>] deploy [--quick] <environment> <app> (--service <name> | --all)
noops [--workspace <workspace>] rollback <environment> <app> (--service <name> | --all)
noops [--workspace <workspace>] remove <environment> <app> (--service <name> | --all)
noops secret set <environment> <key>
noops secret list <environment>
noops cleanup [--apply] [--orphaned] [--keep <count>]
```

Running `noops` without arguments prints its name and build version.

## Platform commands

| Command                 | Behavior                                                                                                                                                         |
|-------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `init <workspace>`      | Creates the workspace-local `.noops/state` and `.noops/data` stores.                                                                                             |
| `install`               | Initializes Swarm when required, creates the shared network, deploys the registry, and records installation metadata.                                            |
| `doctor`                | Checks Docker, Swarm, installation artifacts, network, and registry.                                                                                             |
| `doctor --deploy-ready` | Checks only the runtime prerequisites used by `deploy`.                                                                                                          |
| `status`                | Reports recorded installation metadata and component status.                                                                                                     |
| `uninstall`             | Removes managed app stacks, registry stack, shared network when Docker allows it, generated state, and installation metadata. It keeps persistent registry data. |
| `uninstall --purge`     | Performs uninstall and removes persistent registry data.                                                                                                         |

`uninstall` does not remove the installed CLI executable. `make uninstall` additionally removes the repository-local
`.bin/noops` after teardown succeeds. `uninstall --purge` removes only the workspace `.noops/state` and `.noops/data`
directories.

## Application commands

| Command                                                    | Behavior                                                                                                                                                                                                                                                                                                       |
|------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `release <env> <app> (--service <name> \| --all)`          | Builds or snapshots selected services, pushes immutable images, and records release metadata.                                                                                                                                                                                                                  |
| `deploy [--quick] <env> <app> (--service <name> \| --all)` | Deploys selected services using their latest recorded releases. `--quick` uses the health-check start period as its monitor window.                                                                                                                                                                            |
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
values.

## Certificate commands

`certificate import <name> <certificate.pem> <private-key.pem>` imports a supplied TLS certificate for nginx routes that
use `expose.tls_certificate`. It is intended for trusted origin certificates such as Cloudflare Origin CA certificates.
