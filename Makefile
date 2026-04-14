APP=calorie-tracker

.PHONY: run build fmt tidy docker-build up down

run:
	go run ./cmd/server

build:
	CGO_ENABLED=0 go build ./...

fmt:
	gofmt -w ./cmd ./internal ./web

tidy:
	go mod tidy

docker-build:
	docker build -t $(APP):latest .

up:
	docker compose up --build

down:
	docker compose down
