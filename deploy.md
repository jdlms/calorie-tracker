# Deployment

This project is deployed manually from a checked-out git repository on the server.

## Deployment model

The app is a local, build-from-source Docker Compose project. That means the server needs a real project directory containing:

- `docker-compose.yml`
- `Dockerfile`
- Go source
- frontend source
- `.env`
- `data/`

Unlike apps that use only a prebuilt image, this app must exist on disk as a full repo checkout because deployment uses:

```bash
docker compose up -d --build
```

## Recommended server layout

Use a directory under the broader apps area so the same directory is both:

- the git checkout
- the deploy directory

Recommended path:

```bash
/opt/apps/calorie-tracker
```

Example layout:

```text
/opt/apps/calorie-tracker/
  .env
  docker-compose.yml
  Dockerfile
  data/
  cmd/
  internal/
  frontend/
  web/
  go.mod
  go.sum
  readme.md
```

## What stays on the server

These should remain server-local:

- `.env`
- `data/`

The rest comes from git.

## Manual deploy process

From the server:

```bash
cd /opt/apps/calorie-tracker
git pull --ff-only origin main
docker compose up -d --build
```

### Why `--ff-only`

`git pull --ff-only` avoids accidental merge commits on the server and keeps deployment state predictable.

## Notes

- No GitHub Actions deployment is required.
- CI can be added later if desired, but deployment remains manual.
- This app can live alongside other apps under `/opt/apps` even though it is home-grown.
- It is normal for the same directory to be both the source checkout and the Docker Compose deployment directory.

## Current priority

Right now the priority is iterating on the app's appearance, so deployment stays intentionally simple.
