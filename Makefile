.PHONY: run build dev migrate templ tailwind test check

# Load configuration from the gitignored .env (see .env.example) and export it
# to sub-commands, so `make run`/`make dev`/`make migrate` work from any shell.
-include .env
export DATABASE_URL SESSION_SECRET PASSWORD_PEPPER APP_ENV HTTP_ADDR REDIS_URL

run:
	go run ./cmd/web

build:
	go build -o bin/web ./cmd/web

dev:
	air

migrate:
	go run ./cmd/web -migrate

templ:
	templ generate

tailwind:
	npx tailwindcss -i ./assets/css/input.css -o ./web/static/css/output.css

test:
	go test ./...

check:
	go test ./...
	scripts/check-feature-boundaries.sh
	scripts/check-template-atomic-boundaries.sh
