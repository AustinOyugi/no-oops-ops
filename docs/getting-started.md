# Getting started

## Requirements

- Go 1.25 or later when building from source
- Docker running locally
- Docker Swarm manager authority on the machine that runs `noops install` and `noops deploy`
- Docker configured to allow the internal registry at `127.0.0.1:5000`

The bundled registry uses HTTP. For Docker Desktop, add the following to the Docker Engine configuration and restart Docker Desktop:

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

This installs `noops` in `~/.local/bin`. Ensure that directory is on your `PATH`. To use a repository-local executable instead:

```bash
make build
./.bin/noops status
```

## Initialize the local platform

```bash
noops install
noops doctor
```

Installation verifies Docker, initializes Swarm when necessary, creates the shared network, deploys the internal registry, and writes installation metadata.

## Release and deploy an app

Create an app manifest and environment file next to one another. The examples in `examples/` show the expected shape.

```bash
noops release prod path/to/app.yml
noops deploy prod path/to/app.yml
```

For apps that reference a Swarm secret, set it before deploying:

```bash
noops secret set prod DATABASE_URL
```

See the [manifest reference](reference/app-manifest.md) and [environment-file reference](reference/env-file.md) before creating a production manifest.
