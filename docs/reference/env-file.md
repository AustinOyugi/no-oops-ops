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
