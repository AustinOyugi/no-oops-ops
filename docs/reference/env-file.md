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
Configs use the same `sections` / `items` schema as service env files, including
`values` per environment and `from_secret` references.

Select files relative to the service manifest, or use absolute paths:

```yaml
services:
  api:
    image: example/api:latest
    x-noops:
      env:
        configs:
          - ../configs/database.yml
          - ../configs/cache.yml
        file: api.env.yml
```

Configs merge in listed order; later files win. The service's `env.file` always
wins over shared configs. Overrides replace the entire item by key, including
secret references and environment-specific values. `env.file` is optional when
only shared configs are needed. The same merged inputs feed runtime deployment,
build env files, and explicitly allowed build secrets. Existing secret allow-list
and delivery rules still apply; store credentials as managed secrets.
