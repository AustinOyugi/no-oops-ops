# Networking and TLS

No Oops runs one shared Nginx ingress service for all exposed applications. Applications stay on private Swarm overlay
networks; Nginx is the only managed service that publishes HTTP and HTTPS to the host.

## Platform ingress

The optional `settings.platform.ingress` block in `apps.yml` configures the shared platform service. HTTP and HTTPS use
ports 80 and 443 by default, so only specify alternative ports when the host requires them:

```yaml
settings:
  platform:
    ingress:
      name: noops-nginx
      # http_port: 80
      # https_port: 443
```

An app exposes a route independently in its own manifest:

```yaml
x-noops:
  service:
    internal_port: 3000
  ingress:
    enabled: true
    domain: app.example.com
    path_prefix: /
```

`internal_port` is never published directly. Nginx forwards the route to the private Swarm service. Services can also
reach an exposed app through `http://ingress.noops.internal/<environment>/<app>/...`.

## Let's Encrypt TLS

Use the default TLS flow when the domain resolves directly to the Swarm manager and ports 80 and 443 are reachable from
the internet:

```yaml
x-noops:
  ingress:
    enabled: true
    domain: app.example.com
    tls: true
```

On the first deployment, No Oops prompts for an ACME contact email, exposes the HTTP-01 challenge path, and obtains a
Let's Encrypt certificate. The managed certbot service renews it and reloads Nginx. Do not use this mode with
`tls_certificate`.

## Wildcard hostnames

Ingress accepts a leftmost wildcard such as `*.vybes.africa`. nginx always selects an exact hostname first, so a route
for `api.vybes.africa` overrides the wildcard route for that host. Wildcard HTTPS routes require an imported
`tls_certificate`; No Oops does not use ACME HTTP-01 for wildcards.

## Cloudflare Origin TLS

Use Cloudflare mode when every public hostname served by the shared ingress is Cloudflare-proxied. Enable it once in
the workspace catalog:

```yaml
settings:
  platform:
    ingress:
      cloudflare: true
```

Cloudflare mode has these effects:

- No Oops does not prompt for an ACME email or obtain Let's Encrypt certificates.
- Nginx trusts `CF-Connecting-IP` only from Cloudflare's published IPv4 and IPv6 proxy networks, then forwards the
  verified visitor address as `X-Real-IP` and `X-Forwarded-For`.
- Generated HTTPS virtual hosts enable HTTP/2, allow TLS 1.2 and TLS 1.3, prefer client cipher ordering, and use the
  X25519 and P-256 ECDH curves.

Create a Cloudflare Origin certificate for the app hostname (or a suitable wildcard) in the Cloudflare dashboard. Copy
its certificate and private key to the server securely, then import them:

```bash
noops certificate import cranium-cloudflare origin-cert.pem origin-key.pem
```

Reference its import name from every HTTPS app:

```yaml
x-noops:
  ingress:
    enabled: true
    domain: app.example.com
    tls_certificate: cranium-cloudflare
```

Do not set `tls: true` alongside `tls_certificate`. The third legacy Nginx directive,
`ssl_trusted_certificate cloudflare_origin_ca.pem`, is not needed for normal Cloudflare Origin TLS: Nginx is presenting
a server certificate to Cloudflare, not validating client certificates.

Keep the Cloudflare DNS record proxied (orange cloud) and set its SSL/TLS encryption mode to **Full (strict)**. Direct
connections to the origin will not trust a Cloudflare Origin certificate, which is intentional.

## Security boundary

`set_real_ip_from` is what makes the Cloudflare client-IP header safe. Nginx applies `CF-Connecting-IP` only if the
source address is in a trusted Cloudflare network. Do not enable `cloudflare: true` for an ingress that accepts direct,
non-Cloudflare public traffic; otherwise application logs and IP-based controls may be incorrect.
