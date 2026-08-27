# Getting started

## Requirements

- Go 1.25 or later when building from source
- Docker running locally
- Docker Swarm manager authority on the machine that runs `noops install` and `noops deploy`
- Docker configured to allow the internal registry at `127.0.0.1:5000`
- A No Oops workspace directory containing `apps.yml` and application manifests

For a public TLS route, the domain's DNS A/AAAA record must point to the Swarm manager and ports 80 and 443 must be
reachable from the internet. The first TLS deployment prompts for the ACME contact email and stores it in the
workspace configuration.

The bundled registry uses HTTP. For Docker Desktop, add the following to the Docker Engine configuration and restart
Docker Desktop:

```json
{
  "insecure-registries": ["127.0.0.1:5000"]
}
```

## Install the CLI

From a source checkout:

```bash
make install
noops status
```

This installs `noops` in `~/.local/bin`. Ensure that directory is on your `PATH`. To use a repository-local executable
instead:

```bash
make build
./.bin/noops status
```

## Create a workspace and initialize the platform

No Oops does not use a global state directory. Run `init` once for each
deployment workspace; it creates the Git-ignored `.noops/` runtime store.

```bash
noops init /srv/cranium/noops
cd /srv/cranium/noops
# Edit apps.yml with the workspace platform settings and app aliases.
noops install
noops doctor
```

Installation verifies Docker, initializes Swarm when necessary, creates the shared network, deploys the internal
registry and nginx ingress, waits for both services to be ready, and writes installation metadata.

## Release and deploy an app

Create an app manifest next to an environment file when the service needs environment values or secret bindings. Static
services with neither can omit the environment file. The examples in `examples/` show both shapes.

```bash
noops release prod api --service api
noops deploy prod api --service api
```

`api` is an alias declared in the workspace `apps.yml`. Use
`--workspace /srv/cranium/noops` from another directory.

An `app.yml` may contain several Compose services. Use `--all` to process them in stable `x-noops.depends_on` order;
processing stops at the first failure.

For apps that reference a Swarm secret, set it before deploying:

```bash
noops secret set prod DATABASE_URL
```

For a private Git build source, create a fine-grained GitHub token with repository Contents read access and save only
the token value as a normal environment secret. No Oops creates a versioned Swarm secret; it does not save the token in
the
workspace or application manifest.

```bash
noops secret set prod github-readonly
```

Use that secret key from `x-noops.build.source.git.environments.<environment>.secret`; see the [manifest
reference](reference/app-manifest.md).

See the [manifest reference](reference/app-manifest.md) and [environment-file reference](reference/env-file.md) before
creating a production manifest.
