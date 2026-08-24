# App manifest reference

The manifest is Docker Compose YAML. Existing Compose is the source of truth: No Oops Ops retains fields it does not own, including later compatible Compose fields, and removes `x-noops` before handing the generated stack to Docker. Relative `env_file`, bind-mount, config, and secret file paths are made absolute when the generated stack is written to state.

## Manifest format

No Oops Ops accepts a Compose-shaped manifest with one or more services. Standard Docker and Swarm fields remain owned by the application. No Oops Ops features remain under `x-noops`.

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

Lifecycle operations target a service explicitly: `noops release <env> app.yml --service api`, `noops deploy <env> app.yml --service api`, or `--all`. `--all` uses a stable dependency order from `x-noops.depends_on` and stops at the first failure. Compose `depends_on` is preserved, but is not a Swarm readiness guarantee; applications must retry dependencies at runtime. The earlier top-level `name`, `image`, and `service` format is not supported.

No Oops rejects `container_name` because Swarm schedules service tasks, rejects public `ports` on ingress-managed services, and rejects plainly embedded credential-like environment values. Move such values to managed No Oops secrets. `x-noops.expose` remains accepted as a compatibility alias for `x-noops.ingress`.

## Supported fields

| Field | Required | Default | Meaning |
| --- | --- | --- | --- |
| `services.<name>` | Yes | — | A Compose service. Its `image`, `build`, execution, environment, networks, volumes, configs, secrets, health check, labels, and `deploy` settings are preserved. |
| `services.<name>.image` | Yes | — | Upstream image reference. It is replaced in the generated stack with the recorded immutable release image. |
| `services.<name>.build` | No | — | Standard Compose build configuration. When present, No Oops builds it before release. |
| `services.<name>.x-noops.service.internal_port` | Yes for ingress | — | Private application port used by managed nginx ingress. |
| `services.<name>.x-noops.source.build.command` | No | — | Command run in the build context before a Compose build. |
| `services.<name>.x-noops.env.file` | When using No Oops environment values | — | Environment YAML file, relative to the manifest. |
| `services.<name>.x-noops.env.secrets` | No | — | Allow-listed versioned secret references and delivery mode. |
| `services.<name>.x-noops.ingress.*` | No | disabled | Managed nginx route, TLS, and blue/green settings. |
| `services.<name>.x-noops.rollout.*` | No | See below | No Oops convergence monitoring settings. It does not replace existing `deploy.update_config`, `rollback_config`, or restart policy. |
| `services.<name>.x-noops.depends_on` | No | `[]` | Deployment ordering for `--all`; not a runtime readiness guarantee. |

`healthcheck.test` must be an array accepted by Docker. Duration values use Go duration syntax, such as `30s` or `2m`.

## Rollout defaults

The defaults are: update `order: start-first`, `parallelism: 1`, `delay: 10s`, `failure_action: rollback`; restart `condition: on-failure`, `delay: 10s`, `max_attempts: 5`, `window: 70s`; rollback `order: start-first`, `parallelism: 1`, `delay: 0s`, `failure_action: pause`; and `convergence_timeout: 5m`.

`rollout.monitor` defaults to `healthcheck.start_period + retries × interval + timeout + 10s`. Override it only when the application needs a longer or shorter Swarm monitoring window. `max_failure_ratio` for both update and rollback must be between 0 and 1.

For development feedback loops, `noops deploy --quick <environment> <manifest>` temporarily uses `healthcheck.start_period` as the monitor window while retaining the manifest's `rollout.convergence_timeout`. It does not change the manifest; a later normal deploy uses the configured monitor again.

## Public routing

After the application successfully converges, `noops deploy` writes its enabled route to the platform-managed nginx ingress. nginx forwards requests to the application's private Swarm service and `service.internal_port`; no application port is published directly. `noops remove` removes the route before stopping the stack.

Blue/green promotion is enabled by default for stateless ingress-managed apps. Named volumes are rejected in blue/green mode; set `x-noops.ingress.blue_green: false` to use an in-place Swarm update for stateful services. Set `x-noops.ingress.tls: true` to serve HTTPS. The domain's DNS A/AAAA record must resolve to the Swarm manager and allow inbound ports 80 and 443. On its first deployment, No Oops Ops makes the HTTP-01 challenge path available, obtains a Let's Encrypt certificate, and then enables the HTTPS virtual host with HTTP-to-HTTPS redirects. Subsequent certificate renewals reload nginx automatically. The current renewal service mounts the Docker daemon socket to force that reload; this grants it root-equivalent Docker-host access and is suitable only for the trusted single-node platform. A given domain/path-prefix pair can be owned by only one deployed app, and all routes sharing a domain must agree on its TLS setting. Compose `depends_on` is preserved but should not be used as a readiness guarantee.

For an existing Cloudflare Origin certificate, import a rotated certificate/key pair with `noops certificate import <name> <certificate.pem> <private-key.pem>`, then set `expose.tls_certificate: <name>`. Imported keys are stored in the platform nginx data directory with owner-only permissions and mounted read-only in nginx. Cloudflare must remain proxied with SSL/TLS mode set to Full (strict).

## Internal routing

Services on the shared network can call exposed applications through the nginx alias `ingress.noops.internal`, without depending on a release-specific Swarm service name. The internal URL is `http://ingress.noops.internal/<environment>/<app>/...`; nginx strips the internal prefix before proxying the request. For example, the development `lango` app is reachable internally as `http://ingress.noops.internal/dev/lango/health` and receives `/health`.

Nginx keeps generated external routes in one file per domain and internal routes in one file per environment/app. This keeps each route owner isolated while nginx still loads one effective configuration.
