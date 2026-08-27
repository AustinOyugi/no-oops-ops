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

For a source build, those ordinary values are also passed to `docker build` as `--build-arg KEY=value`. This makes
public compile-time configuration available to tools such as `next build`. A Dockerfile must declare the argument and
export it in the build stage when the build tool reads it:

```dockerfile
ARG NEXT_PUBLIC_APP_URI
ENV NEXT_PUBLIC_APP_URI=$NEXT_PUBLIC_APP_URI
RUN npm run build
```

Only ordinary values are supplied this way. `from_secret` values are never Docker build arguments and are injected only
when Swarm starts the service. Do not use a managed secret for a client-side `NEXT_PUBLIC_*` value: anything embedded in
a browser bundle is public by design.

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
