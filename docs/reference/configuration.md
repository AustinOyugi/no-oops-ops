# Workspace storage

No Oops is workspace-based. Initialize a workspace with `noops init <directory>`.
The CLI may write only below that workspace's `.noops/` directory. Use the
workspace as the current directory or provide it with `--workspace`.

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

The workspace state directory contains paths such as:

```text
install.json
registry/config.yml
registry/stack.yml
nginx/stack.yml
apps/<app>/<environment>/.env
apps/<app>/<environment>/stack.yml
apps/<app>/<environment>/release.json
apps/<app>/<environment>/releases/<timestamp>.json
apps/<app>/<environment>/deployments/<timestamp>.json
secrets/<environment>/<key>/v<version>.json
```

Secret files contain metadata only; values are stored by Docker Swarm. `remove` deletes an app environment's generated state but preserves its secrets and named volumes. `uninstall --purge` also deletes the configured persistent registry data.
