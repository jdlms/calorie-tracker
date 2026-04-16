## Project Notes

### Architecture

- This project is a single Go service with an embedded Svelte frontend.
- Telegram ingestion uses long polling. No webhooks or public inbound connectivity are required.
- Storage is BoltDB in a single file.
- Frontend build output lives in `web/dist/` and is embedded into the Go binary.

### Frontend

- Frontend source lives in `frontend/`.
- Build the frontend with `cd frontend && npm run build`.
- The UI is intentionally dark-mode-only.
- Prefer simple layout and visual changes over introducing UI libraries.

### Running

- Local backend run: `go run ./cmd/server`
- Frontend dev server: `cd frontend && npm run dev`
- Full container run: `docker compose up --build`

### Deployment

- Deployment is manual.
- The app is intended to live on the server as a checked out repo that is also the Docker Compose project directory.
- Recommended server layout is documented in `deploy.md`.
- Standard deploy flow on the server:

```bash
cd /opt/apps/calorie-tracker
git pull --ff-only origin main
docker compose up -d --build
```
