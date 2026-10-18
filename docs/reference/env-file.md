# Environment-file reference

When declared, the environment file is YAML and is rendered into a Docker-compatible `.env` file during deployment. A
service with no environment values or secret bindings can omit `x-noops.env` entirely.

```yaml
sections:
  - name: app
    items:
      - key: SERVER_PORT
        value: "8080"
      - key: SPRING_PROFILES_ACTIVE
        values:
          dev: dev
          prod: prod
      - key: DATABASE_URL
        from_secret: DATABASE_URL
```

For ordinary values, `values[environment]` takes precedence over `value`. When neither is provided, that key is omitted.

An app can also materialize its ordinary values into a dotenv file in No Oops' temporary build context. This makes
compile-time configuration available to tools such as `next build` without Dockerfile-specific configuration:

```yaml
env:
  file: app.env.yml
  build:
    file: .env.production
```

No Oops writes the generated file before `docker build`, temporarily overrides `.dockerignore` if necessary so it is
included, and restores the build context afterward. Frameworks that load dotenv files conventionally, including Next.js,
need no Dockerfile changes.

Only ordinary values are supplied this way. `from_secret` values are never present in the build context and are injected
only when Swarm starts the service. Do not use a managed secret for a client-side `NEXT_PUBLIC_*` value: anything
embedded in a browser bundle is public by design.

## BuildKit secret mounts

For a genuinely private credential required by a build tool, explicitly select the `from_secret` keys that the
Dockerfile consumes with a standard BuildKit secret mount:

```yaml
env:
  file: app.env.yml
  build:
    file: .env
    secrets:
      - SENTRY_AUTH_TOKEN
```

```yaml
sections:
  - name: telemetry
    items:
      - key: SENTRY_AUTH_TOKEN
        from_secret: SENTRY_AUTH_TOKEN
```

No Oops runs this build in a one-shot Swarm task, mounting the existing Swarm secret only into that task. It passes the
value to BuildKit as secret ID `SENTRY_AUTH_TOKEN`; the value is never written to the Git checkout or emitted into the
final image. The Dockerfile must consume it on the exact build instruction:

```dockerfile
RUN --mount=type=secret,id=SENTRY_AUTH_TOKEN,env=SENTRY_AUTH_TOKEN \
    npm run build
```

Use this only for private build credentials. `NEXT_PUBLIC_*` values are browser-public and should remain ordinary build
values or be served through a runtime configuration endpoint.

`from_secret` does not put a secret in `.env`. To activate a secret reference, list its environment key under
`env.secrets.resolvable` in the app manifest:

```yaml
env:
  file: app.env.yml
  secrets:
    resolution: file
    resolvable:
      - DATABASE_URL
```

Every resolvable key must exist in the environment file and be backed by `from_secret`. The referenced secret name is
the value of `from_secret`; it must have been set for the deployment environment with `noops secret set`.

With `resolution: file`, the secret is mounted at `/run/secrets/DATABASE_URL` and no secret value is exported as an
environment variable. With `resolution: env`, a generated wrapper reads the secret file and exports `DATABASE_URL`
before the app starts.

Neither mode sets a `DATABASE_URL_FILE` variable. To offer both, declare the variable yourself in the environment file
alongside the secret-backed key:

```yaml
      - key: DATABASE_URL_FILE
        value: "/run/secrets/DATABASE_URL"

      - key: DATABASE_URL
        from_secret: DATABASE_URL_SECRET
```

## Reusable configs

Install shared environment files (database, cache, mail, or any other category):

```sh
noops --workspace /srv/apps config install database ./database.env.yml
noops --workspace /srv/apps config upgrade database ./database.env.yml
```

Install creates `/srv/apps/configs/database.yml`; upgrade replaces an existing
installation atomically. Install refuses to overwrite an existing config, and
upgrade requires it to exist. These commands only manage local files; redeploy
consuming services to apply an upgrade (release again for build-time values).
Configs use the same `sections` / `items` schema as service env files. Select
individual keys in the service's env file using `from_config`:

```yaml
# apps/api/env.yml
sections:
  - name: database
    items:
      - key: DATABASE_HOST
        from_config: database
      - key: DATABASE_PORT
        from_config: database
        values:
          dev: "5433"
      - key: DATABASE_PASSWORD
        from_secret: api-database-password
```

The first argument to `config install` is the alias. Installing
`noops config install database ./database.env.yml` makes `database` available to
all services in the selected workspace. `config upgrade database <file>` replaces
its contents while keeping the alias.

A scalar `from_config` selects the item with the same key from that installed config.
Only explicitly referenced keys are included; other config items are never
injected. Aliases resolve from `<workspace>/configs/<alias>.yml`, using `--workspace`
(the current directory by default). Resolution is independent of the manifest
location and `--state-dir`; separate workspaces can install different contents
under the same alias. Missing aliases report how to install them. Aliases accept
letters, digits, underscores and hyphens, starting with a letter or digit.
Nested config references use the same selected workspace.

Legacy file paths remain accepted in scalar references and the `{file: ..., key: ...}`
mapping. They resolve relative to the referencing env/config file; new references
should use aliases. A mapping must specify either `alias` or `file`, never both.

To rename a config key, use the mapping form:

```yaml
- key: APP_DATABASE_HOST
  from_config:
    alias: database
    key: DATABASE_HOST
```

Local `values.<environment>` entries override matching shared entries, keeping
shared values for other environments. A local `value` replaces the shared
default and all shared environment entries, so it applies everywhere unless a
local `values.<environment>` entry overrides it. Explicit empty strings are
valid overrides. Shared config files can reference other configs; missing files,
missing or duplicate selected keys, and reference cycles fail before release or
deployment. Even with a local override, the reference must be valid.

The service manifest declares only its env file:

```yaml
x-noops:
  env:
    file: env.yml
```

Whole-file `x-noops.env.configs` imports are rejected with a migration error.
Replace each imported key with a `from_config` item, and remove the manifest
list. Config install/upgrade commands remain unchanged.

Build env generation and runtime deployment use the same selected values.
`from_secret` references selected through a config remain subject to the usual
runtime/build secret allow-lists; unselected secrets never reach the service.
Do not combine `from_config` and `from_secret` on the same item. To replace a
shared secret reference, declare a local `from_secret` item instead.
