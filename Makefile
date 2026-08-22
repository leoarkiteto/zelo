.PHONY: run build dev migrate templ tailwind test

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
