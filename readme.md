# Calorie Tracker -- Architecture

## Overview

Single-user calorie tracker. Text what you ate to a Telegram bot, an LLM estimates the nutritional data, entries are stored in BoltDB, and a Svelte frontend displays the data.

## Pipeline

```
Telegram message -> Go binary (long poll) -> LLM API -> BoltDB -> Svelte frontend
```

## Components (Single Go Binary)

```
┌─────────────┐     ┌──────────────────────────────────┐     ┌──────────┐
│  Telegram   │────>│  Go binary (single process)      │     │ Browser  │
│  long poll  │     │                                  │────>│ frontend │
└─────────────┘     │  - telegram poller               │     └──────────┘
                    │  - llm client (any-llm-go)       │
                    │  - boltdb storage                │
                    │  - http server (:8080)            │
                    │                                  │
                    │  /api/entries?date=...            │
                    │  /api/summary?range=7d            │
                    │  / (serves static frontend)       │
                    └──────────────────────────────────┘
```

Everything runs as a single process inside a single container.

## Input

- **Source:** Telegram bot, using long polling (no webhooks, no inbound connectivity required).
- **Flow:** User sends a free-text message ("I just ate a chicken sandwich"). The message is forwarded to an LLM API which returns structured nutritional data. The result is appended to BoltDB. No confirmation step -- fire and forget.

## LLM

- **Library:** `github.com/mozilla-ai/any-llm-go v0.9.0`
- **Behaviour:** Stateless per-message. Raw text in, structured data out.
- **Note:** Prompt engineering for accurate calorie estimation is deferred. Build first, tune later.

## Storage

- **Engine:** BoltDB (embedded, single-file, no external dependencies).
- **Buckets:**
  - `entries` for calorie entries
  - `metadata` for Telegram poller state and retry metadata
- **Key design:** `YYYY-MM-DD:unix_timestamp:entry_id` -- BoltDB's sorted key iteration allows efficient prefix-based range scans by date while avoiding collisions for entries created in the same second.
- **Value:** JSON-encoded entry struct.
- **Persistence:** BoltDB file lives on a Docker volume mount so data survives container restarts.

## Entry Schema

```json
{
  "id": "string",
  "timestamp": "unix_timestamp",
  "description": "chicken sandwich",
  "kcal": 450,
  "protein": 30,
  "fat": 18,
  "carbs": 40
}
```

## API Surface

|Method|Route|Description|
|---|---|---|
|GET|`/api/health`|Health check plus high-level service configuration state|
|GET|`/api/configured`|Show whether Telegram and LLM ingestion are configured|
|GET|`/api/entries?date=YYYY-MM-DD`|Fetch entries for a given day|
|POST|`/api/entries`|Create an entry from free text using the LLM|
|GET|`/api/summary?range=7d`|Fetch summary over a range|

Daily totals are computed either client-side in Svelte or server-side -- not a critical decision.

### Create Entry Manually

```bash
curl -X POST http://localhost:8080/api/entries \
  -H 'Content-Type: application/json' \
  -d '{
    "text": "chicken sandwich and fries",
    "timestamp": "2026-04-14T12:00:00Z"
  }'
```

## Frontend

- **Framework:** Svelte
- **Serving:** Built static assets are embedded into the Go binary using Go's `embed.FS` (standard library, Go 1.16+). Served via `http.FileServer`. No separate frontend process.
- **Scope:** Display daily entries in a table, show running totals, and allow day-by-day navigation to review current and past days.
- **Source:** `frontend/`
- **Build output:** `web/dist/`

## Containerisation

- **Multi-stage Dockerfile:**
    1. Stage 1: Build Svelte app (`npm run build`).
    2. Stage 2: Build Go binary (with Svelte output available for `embed.FS`).
    3. Stage 3: Copy binary into minimal base image (`alpine` or `scratch`).
- **Volume:** Single volume mount for the BoltDB file.
- **Result:** One image, one container, one process, one volume.

## Configuration

Environment variables:

|Variable|Required|Description|
|---|---|---|
|`HTTP_ADDR`|No|HTTP listen address. Default `:8080`|
|`BOLTDB_PATH`|No|BoltDB file path. Default `data/calorie-tracker.db`|
|`TELEGRAM_BOT_TOKEN`|Yes for Telegram ingestion|Telegram bot token|
|`TELEGRAM_ALLOWED_CHAT_ID`|Recommended|Only accept messages from this chat ID|
|`LLM_PROVIDER`|No|LLM provider ID. Currently `mistral`, `ollama`, or `openai`. Default `mistral`|
|`LLM_API_KEY`|Usually|API key for the configured LLM provider|
|`LLM_BASE_URL`|Optional|Provider base URL override, useful for self-hosted or custom endpoints|
|`LLM_MODEL`|Yes for ingestion|Model name to use for nutrition estimation|

### Running Locally

```bash
go run ./cmd/server
```

Or with Make:

```bash
make run
```

The Make targets build the Svelte frontend before starting or compiling the Go binary.

If `LLM_MODEL` is not set, LLM-backed ingestion is disabled and Telegram/manual ingestion will fail.

### Example: Mistral

```env
LLM_PROVIDER=mistral
LLM_API_KEY=your-mistral-api-key
LLM_BASE_URL=
LLM_MODEL=mistral-small-latest
```

### Example: Ollama with a local Mistral model

```env
LLM_PROVIDER=ollama
LLM_BASE_URL=http://localhost:11434
LLM_MODEL=mistral
```

## Docker

Build and run with Docker Compose:

```bash
docker compose up --build
```

Or via Make:

```bash
make up
```

The container exposes port `8080` and stores BoltDB data in `./data` on the host.

## Telegram Retry and Offset Behaviour

- The poller uses Telegram long polling via `getUpdates`.
- The current Telegram offset is persisted in BoltDB so restarts do not replay already-acknowledged updates.
- A message is only acknowledged after it is successfully processed, or after it fails 3 times.
- Failure counts are persisted in BoltDB so retries survive process restarts.
- After 3 failed attempts for the same `update_id`, the poller skips that update and continues with newer messages.

## Deployment

- **Host:** Homelab server.
- **Networking:** Tailscale. No public ingress required. Telegram long polling is outbound-only. Frontend accessed via Tailscale IP/hostname.

## Why Long Polling Over Webhooks

Telegram webhooks require Telegram servers to reach your endpoint over the public internet. Behind Tailscale on a homelab, that would require Tailscale Funnel or a reverse proxy. Long polling is outbound-only -- the Go binary pulls updates from Telegram's API -- so it works behind any NAT/firewall with no additional configuration.

## Why embed.FS

Without it, the container would need to serve Svelte build output from a directory on disk, run a separate web server, or run a separate dev server. Embedding bakes the static assets into the compiled binary. One binary contains everything: Telegram poller, LLM client, BoltDB storage, HTTP API, and the entire Svelte frontend. The tradeoff is a two-step build process (Svelte then Go), handled cleanly by the multi-stage Dockerfile.
