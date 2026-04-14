APP=calorie-tracker

.PHONY: run build fmt tidy frontend-install frontend-build docker-build up down

run: frontend-build
	go run ./cmd/server

build: frontend-build
	CGO_ENABLED=0 go build ./...

fmt:
	gofmt -w ./cmd ./internal ./web

frontend-install:
	cd frontend && npm install

frontend-build:
	cd frontend && npm run build

tidy:
	go mod tidy

docker-build:
	docker build -t $(APP):latest .

up:
	docker compose up --build

down:
	docker compose down
