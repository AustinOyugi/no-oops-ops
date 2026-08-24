# App manifest reference

The manifest is YAML. `source.context` and `source.dockerfile` are resolved relative to the manifest unless absolute. `env.file` is resolved relative to the manifest directory.

## Manifest format

No Oops Ops accepts a Compose-shaped manifest containing one service. Standard Docker fields such as `image`, `build`, `command`, `healthcheck`, `deploy.replicas`, `networks`, and `volumes` are mapped into the deployment. No Oops Ops features remain under `x-noops`.

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
      env:
        file: api.env.yml
```

Manifests currently require exactly one service. Multi-service selection and raw Compose pass-through are the next extension. The earlier top-level `name`, `image`, and `service` manifest format is no longer supported.

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
| `expose.blue_green` | No | `true` | Promotes a healthy release-specific candidate through nginx instead of updating the active service in place; requires `expose.enabled: true`. Set it to `false` to use an in-place Swarm update. |
| `expose.domain` | When enabled | — | HTTP host name served by nginx. |
| `expose.domains` | No | `[]` | Additional host names served by the same route, such as `www.example.com`. |
| `expose.tls` | No | `false` | Obtains and serves a Let's Encrypt certificate for `expose.domain`; all routes sharing that domain must use the same setting. |
| `expose.tls_certificate` | No | — | Name of an imported TLS certificate. Mutually exclusive with `expose.tls`. |
| `expose.path_prefix` | No | `/` | HTTP path prefix forwarded to the app. |
| `expose.proxy.websocket` | No | `false` | Enables WebSocket upgrade forwarding. |
| `expose.proxy.client_max_body_size` | No | — | nginx request body limit, for example `100m`. |
| `env.file` | Effectively yes for deploy | — | Environment YAML file, relative to the manifest. |
| `env.secrets` | No | — | Allow-listed secret references and delivery mode. |

`healthcheck.test` must be an array accepted by Docker. Duration values use Go duration syntax, such as `30s` or `2m`.

## Rollout defaults

The defaults are: update `order: start-first`, `parallelism: 1`, `delay: 10s`, `failure_action: rollback`; restart `condition: on-failure`, `delay: 10s`, `max_attempts: 5`, `window: 70s`; rollback `order: start-first`, `parallelism: 1`, `delay: 0s`, `failure_action: pause`; and `convergence_timeout: 5m`.

`rollout.monitor` defaults to `healthcheck.start_period + retries × interval + timeout + 10s`. Override it only when the application needs a longer or shorter Swarm monitoring window. `max_failure_ratio` for both update and rollback must be between 0 and 1.

For development feedback loops, `noops deploy --quick <environment> <manifest>` temporarily uses `healthcheck.start_period` as the monitor window while retaining the manifest's `rollout.convergence_timeout`. It does not change the manifest; a later normal deploy uses the configured monitor again.

## Public routing

After the application successfully converges, `noops deploy` writes its enabled route to the platform-managed nginx ingress. nginx forwards requests to the application's private Swarm service and `service.internal_port`; no application port is published directly. `noops remove` removes the route before stopping the stack.

Blue/green promotion is enabled by default for stateless exposed apps. Named volumes are rejected in blue/green mode; set `expose.blue_green: false` to use an in-place Swarm update for stateful services. Set `expose.tls: true` to serve HTTPS. The domain's DNS A/AAAA record must resolve to the Swarm manager and allow inbound ports 80 and 443. On its first deployment, No Oops Ops makes the HTTP-01 challenge path available, obtains a Let's Encrypt certificate, and then enables the HTTPS virtual host with HTTP-to-HTTPS redirects. Subsequent certificate renewals reload nginx automatically. The current renewal service mounts the Docker daemon socket to force that reload; this grants it root-equivalent Docker-host access and is suitable only for the trusted single-node platform. A given domain/path-prefix pair can be owned by only one deployed app, and all routes sharing a domain must agree on its TLS setting. `service.external_port` and `depends_on` remain unsupported.

For an existing Cloudflare Origin certificate, import a rotated certificate/key pair with `noops certificate import <name> <certificate.pem> <private-key.pem>`, then set `expose.tls_certificate: <name>`. Imported keys are stored in the platform nginx data directory with owner-only permissions and mounted read-only in nginx. Cloudflare must remain proxied with SSL/TLS mode set to Full (strict).

## Internal routing

Services on the shared network can call exposed applications through the nginx alias `ingress.noops.internal`, without depending on a release-specific Swarm service name. The internal URL is `http://ingress.noops.internal/<environment>/<app>/...`; nginx strips the internal prefix before proxying the request. For example, the development `lango` app is reachable internally as `http://ingress.noops.internal/dev/lango/health` and receives `/health`.

Nginx keeps generated external routes in one file per domain and internal routes in one file per environment/app. This keeps each route owner isolated while nginx still loads one effective configuration.
