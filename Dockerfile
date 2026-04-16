FROM node:22-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm ci
COPY frontend ./
RUN npm run build

FROM golang:1.25-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY web ./web
COPY --from=frontend-builder /app/web/dist ./web/dist

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/calorie-tracker ./cmd/server

FROM alpine:3.22
WORKDIR /app
RUN adduser -D -u 10001 appuser
COPY --from=builder /out/calorie-tracker /app/calorie-tracker
RUN mkdir -p /app/data && chown -R appuser:appuser /app
USER appuser
EXPOSE 8080
VOLUME ["/app/data"]
CMD ["/app/calorie-tracker"]
