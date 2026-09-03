# App manifest reference

The manifest is Docker Compose YAML. Existing Compose is the source of truth: No Oops Ops retains fields it does not
own, including later compatible Compose fields, and removes `x-noops` before handing the generated stack to Docker.
Relative `env_file`, bind-mount, config, and secret file paths are made absolute when the generated stack is written to
state.

## Manifest format

No Oops Ops accepts a Compose-shaped manifest with one or more services. Standard Docker and Swarm fields remain owned
by the application. No Oops Ops features remain under `x-noops`.

```yaml
services:
  api:
    image: registry.example.com/api:1.2.3
    command: ["serve"]
    healthcheck:
      test: ["CMD", "true"]
    deploy:
      replicas: 2
    x-noops:
      service:
        internal_port: 8080
      ingress:
        enabled: true
        domains: [api.example.com]
      env:
        file: api.env.yml
```

Lifecycle operations target an app alias from the workspace `apps.yml` and a service explicitly:
`noops release <env> api`, `noops deploy <env> api`, or an explicit `--service api` / `--all`. A service selector can be
omitted when the manifest has exactly one service. `--all` uses a stable
dependency order from `x-noops.depends_on` and stops at the first failure. Compose `depends_on` is preserved, but is not
a Swarm readiness guarantee; applications must retry dependencies at runtime. The earlier top-level `name`, `image`, and
`service` format is not supported.

No Oops rejects `container_name` because Swarm schedules service tasks, rejects public `ports` on ingress-managed
services, and rejects plainly embedded credential-like environment values. Move such values to managed No Oops secrets.
`x-noops.expose` remains accepted as a compatibility alias for `x-noops.ingress`.

## Supported fields

| Field                                           | Required        | Default   | Meaning                                                                                                                                                          |
|-------------------------------------------------|-----------------|-----------|------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `services.<name>`                               | Yes             | —         | A Compose service. Its `image`, `build`, execution, environment, networks, volumes, configs, secrets, health check, labels, and `deploy` settings are preserved. |
| `services.<name>.image`                         | Yes             | —         | Upstream image reference. It is replaced in the generated stack with the recorded immutable release image.                                                       |
| `services.<name>.build`                         | No              | —         | Standard Compose build configuration. When present, No Oops builds it before release.                                                                            |
| `services.<name>.x-noops.service.internal_port` | Yes for ingress | —         | Private application port used by managed nginx ingress.                                                                                                          |
| `services.<name>.x-noops.build.source.git`      | No              | —         | Environment-scoped Git repository used as the isolated build context.                                                                                            |
| `services.<name>.x-noops.build.resources`       | No              | —         | CPU and memory limits applied to Docker build steps.                                                                                                             |
| `services.<name>.x-noops.build.timeout`         | No              | —         | Maximum duration of a build.                                                                                                                                     |
| `services.<name>.x-noops.build.no-cache`        | No              | `false`   | Passes `--no-cache` to Docker. Use for cache-warming builds whose mounted-cache side effects must run every release.                                             |
| `services.<name>.x-noops.env.file`              | No              | —         | Environment YAML file, relative to the manifest. Omit `x-noops.env` entirely when the service has no environment values or secret bindings.                      |
| `services.<name>.x-noops.env.build.file`        | No              | —         | Relative dotenv file to generate in the temporary build context from ordinary environment values.                                                                |
| `services.<name>.x-noops.env.secrets`           | No              | —         | Allow-listed versioned secret references and delivery mode.                                                                                                      |
| `services.<name>.x-noops.ingress.*`             | No              | disabled  | Managed nginx route, TLS, and blue/green settings.                                                                                                               |
| `services.<name>.x-noops.rollout.*`             | No              | See below | No Oops convergence monitoring settings. It does not replace existing `deploy.update_config`, `rollback_config`, or restart policy.                              |
| `services.<name>.x-noops.depends_on`            | No              | `[]`      | Deployment ordering for `--all`; not a runtime readiness guarantee.                                                                                              |

`healthcheck.test` must be an array accepted by Docker. Duration values use Go duration syntax, such as `30s` or `2m`.

## Git build contexts

When `x-noops.build.source.git` is present, No Oops fetches the configured environment's repository/ref into a temporary
workspace. Private-source tokens are mounted only into a one-shot Swarm Git-fetch task as a Swarm secret. The Compose
`build.context` and `build.dockerfile` paths are resolved from that checkout; every resulting Docker build runs in a
one-shot Swarm build task, and no application toolchain or Git installation is required on the host.

```yaml
build:
  context: .
  dockerfile: Dockerfile
x-noops:
  build:
    source:
      git:
        url: https://github.com/example/api.git
        environments:
          prod:
            ref: refs/tags/v1.2.3
            secret: github-readonly
    resources:
      cpus: "1.5"
      memory: 2Gi
    timeout: 20m
    # Forces Dockerfile RUN steps to execute even when a prior layer is cached.
    # Useful when the step populates a BuildKit cache mount for another build.
    no-cache: true
```

No Oops resolves and records the resulting commit SHA with the release. When `x-noops.env.build.file` is configured,
ordinary values from `x-noops.env.file` are materialized into that ephemeral dotenv file before Docker builds;
`from_secret` values remain runtime-only by default. An explicit `env.build.secrets` allow-list can make a private
secret
available only as a standard BuildKit secret mount; see [environment files](env-file.md). A Git credential is optional
for public
repositories; when configured, create it for the same environment with `noops secret set` and supply only the provider
access-token value. The `secret` value is the secret key, not the token itself.
`x-noops.source.build.command` is unsupported: build commands belong in Dockerfile stages so they cannot leak language
runtimes onto the host. Builds are serialized per workspace to avoid competing for a single server's resources.

## Rollout defaults

The defaults are: update `order: start-first`, `parallelism: 1`, `delay: 10s`, `failure_action: rollback`; restart
`condition: on-failure`, `delay: 10s`, `max_attempts: 5`, `window: 70s`; rollback `order: start-first`,
`parallelism: 1`, `delay: 0s`, `failure_action: pause`; and `convergence_timeout: 2m`.

`rollout.monitor` defaults to `healthcheck.start_period + retries × interval + timeout + 10s`. Override it only when the
application needs a longer or shorter Swarm monitoring window. `max_failure_ratio` for both update and rollback must be
between 0 and 1.

For development feedback loops, `noops deploy --quick <environment> <app>` temporarily uses `healthcheck.start_period`
as the monitor window while retaining the manifest's `rollout.convergence_timeout`. It does not change the manifest; a
later normal deploy uses the configured monitor again.

## Public routing

After the application successfully converges, `noops deploy` writes its enabled route to the platform-managed nginx
ingress. nginx forwards requests to the application's private Swarm service and `service.internal_port`; no application
port is published directly. `noops remove` removes the route before stopping the stack.

Blue/green promotion is enabled by default for stateless ingress-managed apps. Named volumes are rejected in blue/green
mode; set `x-noops.ingress.blue_green: false` to use an in-place Swarm update for stateful services. Set
`x-noops.ingress.tls: true` to serve HTTPS. The domain's DNS A/AAAA record must resolve to the Swarm manager and allow
inbound ports 80 and 443. With the default platform ingress, the first deployment makes the HTTP-01 challenge path
available, obtains a Let's Encrypt certificate, and then enables the HTTPS virtual host with HTTP-to-HTTPS redirects.
Subsequent certificate
renewals reload nginx automatically and gracefully. The current renewal service mounts the Docker daemon socket to
signal that reload;
this grants it root-equivalent Docker-host access and is suitable only for the trusted single-node platform. A given
domain/path-prefix pair can be owned by only one deployed app, and all routes sharing a domain must agree on its TLS
setting. Compose `depends_on` is preserved but should not be used as a readiness guarantee.

When `settings.platform.ingress.cloudflare: true`, HTTPS routes must use an imported certificate and must not use
`x-noops.ingress.tls: true`. Cloudflare mode does not use ACME or prompt for an email. Import a Cloudflare Origin
certificate/key pair with
`noops certificate import <name> <certificate.pem> <private-key.pem>`, then set
`x-noops.ingress.tls_certificate: <name>`.
Imported keys are stored in the platform nginx data directory with owner-only permissions and mounted read-only in
nginx. Cloudflare must remain proxied with SSL/TLS mode set to Full (strict). No Oops restores the visitor IP from
`CF-Connecting-IP` only when a request originates from a published Cloudflare proxy network.

Managed HTTPS virtual hosts enable HTTP/2, allow TLS 1.2 and TLS 1.3 only, prefer the client cipher order, and use
the X25519 and P-256 ECDH curves.

## Internal routing

Services on the shared network can call exposed applications through the nginx alias `ingress.noops.internal`, without
depending on a release-specific Swarm service name. The internal URL is
`http://ingress.noops.internal/<environment>/<app>/...`; nginx strips the internal prefix before proxying the request.
For example, the development `lango` app is reachable internally as `http://ingress.noops.internal/dev/lango/health` and
receives `/health`.

Nginx keeps generated external routes in one file per domain and internal routes in one file per environment/app. This
keeps each route owner isolated while nginx still loads one effective configuration.
