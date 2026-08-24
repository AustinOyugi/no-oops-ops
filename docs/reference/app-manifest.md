# App manifest reference

The manifest is YAML. `source.context` and `source.dockerfile` are resolved relative to the manifest unless absolute. `env.file` is resolved relative to the manifest directory.

```yaml
name: my-service

source:
  context: .
  dockerfile: Dockerfile
  build:
    command: ["make", "build"]

image:
  repository: my-service
  build: true

service:
  internal_port: 8080
  replicas: 2

healthcheck:
  test: ["CMD", "curl", "-f", "http://localhost:8080/health"]

env:
  file: app.env.yml
```

## Supported fields

| Field | Required | Default | Meaning |
| --- | --- | --- | --- |
| `name` | Yes | — | Application name used in generated state and stack names. |
| `source.context` | When `image.build` is true | — | Docker build context. |
| `source.dockerfile` | When `image.build` is true | — | Dockerfile path. |
| `source.build.command` | No | — | Command array run in the build context before `docker build`. |
| `image.repository` | Yes | — | Image repository name; for an external image this is also the upstream repository. |
| `image.tag` | No | `latest` | Upstream tag when `image.build: false`. |
| `image.build` | No | `true` | Build from source when true; snapshot the upstream image when false. |
| `service.internal_port` | Yes | — | Application port used by health checks and nginx when the app is exposed; it is not published directly. |
| `service.replicas` | No | `1` | Swarm replica count. |
| `service.network` | No | `noops-net` | Existing external Swarm network used by the stack. |
| `service.command` | No | image command | Command override, also used when env-mode secrets need a wrapper. |
| `volumes` | No | `[]` | Docker short mount syntax. Named volumes are defined by the rendered stack; host paths are bind mounts. |
| `healthcheck.test` | Yes | — | Docker healthcheck command array. |
| `healthcheck.interval` | No | `10s` | Go duration. |
| `healthcheck.timeout` | No | `10s` | Go duration. |
| `healthcheck.retries` | No | `3` | Healthcheck retries. |
| `healthcheck.start_period` | No | `60s` | Go duration. |
| `rollout.*` | No | See below | Swarm update, rollback, restart, and convergence settings. |
| `expose.enabled` | No | `false` | Adds the app to the shared nginx ingress when true. |
| `expose.domain` | When enabled | — | HTTP host name served by nginx. |
| `expose.path_prefix` | No | `/` | HTTP path prefix forwarded to the app. |
| `env.file` | Effectively yes for deploy | — | Environment YAML file, relative to the manifest. |
| `env.secrets` | No | — | Allow-listed secret references and delivery mode. |

`healthcheck.test` must be an array accepted by Docker. Duration values use Go duration syntax, such as `30s` or `2m`.

## Rollout defaults

The defaults are: update `order: start-first`, `parallelism: 1`, `delay: 10s`, `failure_action: rollback`; restart `condition: on-failure`, `delay: 10s`, `max_attempts: 5`, `window: 70s`; rollback `order: start-first`, `parallelism: 1`, `delay: 0s`, `failure_action: pause`; and `convergence_timeout: 5m`.

`rollout.monitor` defaults to `healthcheck.start_period + retries × interval + timeout + 10s`. Override it only when the application needs a longer or shorter Swarm monitoring window. `max_failure_ratio` for both update and rollback must be between 0 and 1.

## Public routing

After the application successfully converges, `noops deploy` writes its enabled route to the platform-managed nginx ingress. nginx forwards requests to the application's private Swarm service and `service.internal_port`; no application port is published directly. `noops remove` removes the route before stopping the stack.

Routes are currently plain HTTP only. A given domain/path-prefix pair can be owned by only one deployed app. `service.external_port` and `depends_on` remain unsupported.

## Internal routing

Services on the shared network can call exposed applications through the nginx alias `ingress.noops.internal`, without depending on a release-specific Swarm service name. The internal URL is `http://ingress.noops.internal/<environment>/<app>/...`; nginx strips the internal prefix before proxying the request. For example, the development `lango` app is reachable internally as `http://ingress.noops.internal/dev/lango/health` and receives `/health`.
