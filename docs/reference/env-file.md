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
