# Command reference

```text
noops
noops install
noops uninstall [--purge]
noops doctor [--deploy-ready]
noops status
noops release <environment> <manifest>
noops deploy [--quick] <environment> <manifest> [release-tag]
noops rollback <environment> <manifest>
noops remove <environment> <manifest>
noops secret set <environment> <key>
noops secret list <environment>
```

Running `noops` without arguments prints its name and build version.

## Platform commands

| Command | Behavior |
| --- | --- |
| `install` | Initializes Swarm when required, creates the shared network, deploys the registry, and records installation metadata. |
| `doctor` | Checks Docker, Swarm, installation artifacts, network, and registry. |
| `doctor --deploy-ready` | Checks only the runtime prerequisites used by `deploy`. |
| `status` | Reports recorded installation metadata and component status. |
| `uninstall` | Removes managed app stacks, registry stack, shared network when Docker allows it, generated state, and installation metadata. It keeps persistent registry data. |
| `uninstall --purge` | Performs uninstall and removes persistent registry data. |

`uninstall` does not remove the installed CLI executable. `make uninstall` additionally removes the repository-local `.bin/noops` after teardown succeeds.

## Application commands

| Command | Behavior |
| --- | --- |
| `release <env> <manifest>` | Builds or snapshots an image, pushes an immutable registry image, and records release metadata. |
| `deploy [--quick] <env> <manifest> [tag]` | Deploys the latest recorded release, or the specified release tag. When no release exists, it creates one and retries deployment. `--quick` uses the health-check start period as its monitor window and a short convergence deadline for faster development feedback. |
| `rollback <env> <manifest>` | Redeploys the previous successful deployment, including its pinned secret versions. |
| `remove <env> <manifest>` | Removes the app stack, recorded release and wrapper manifests, and generated app state. Named volumes and environment secrets are preserved. |

## Secret commands

`secret set` reads a value from a hidden terminal prompt or standard input:

```bash
printf '%s' "$DATABASE_URL" | noops secret set prod DATABASE_URL
noops secret list prod
```

Each update creates a versioned Swarm secret such as `noops_prod_DATABASE_URL_v2`. `secret list` shows metadata, not values.
