# 🍞 Calorie Tracker

This is a small personal calorie tracker.

You send a message to a Telegram bot with what you ate. The app sends that text to an LLM, gets back an estimated nutrition breakdown, stores it in BoltDB, and shows it in a simple web UI.

Everything runs as one app:

- Go backend
- embedded Svelte frontend
- BoltDB for storage

## What it does

- stores calorie entries by day
- shows a day view and week view
- supports manual entry through the API
- reads messages from Telegram using long polling

## Configuration

The app reads config from environment variables.

Main ones:

- `HTTP_ADDR` - HTTP listen address, default `:8080`
- `BOLTDB_PATH` - BoltDB file path, default `data/calorie-tracker.db`
- `TELEGRAM_BOT_TOKEN` - required for Telegram ingestion
- `TELEGRAM_ALLOWED_CHAT_ID` - optional, recommended
- `LLM_PROVIDER` - `mistral`, `ollama`, or `openai`
- `LLM_API_KEY` - usually required for hosted providers
- `LLM_BASE_URL` - optional
- `LLM_MODEL` - required for LLM-backed ingestion

If `LLM_MODEL` is not set, ingestion that depends on the LLM will not work.

## Run locally

Run the server:

```bash
go run ./cmd/server
```

Or with Docker Compose:

```bash
docker compose up --build
```

The app serves the frontend and API on port `8080` by default.

## Frontend development

Install frontend deps:

```bash
cd frontend
npm install
```

Run the dev server:

```bash
npm run dev
```

The frontend dev server runs on `http://localhost:5173` and proxies `/api` to the Go server on `http://localhost:8080`.

## Manual API example

```bash
curl -X POST http://localhost:8080/api/entries \
  -H 'Content-Type: application/json' \
  -d '{
    "text": "chicken sandwich and fries",
    "timestamp": "2026-04-14T12:00:00Z"
  }'
```
