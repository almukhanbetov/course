GOOSE = go run github.com/pressly/goose/v3/cmd/goose@v3.27.3

.PHONY: run build migrate-up migrate-down migrate-status

run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api

migrate-up:
	$(GOOSE) postgres "$(DATABASE_URL)" -dir migrations up

migrate-down:
	$(GOOSE) postgres "$(DATABASE_URL)" -dir migrations down

migrate-status:
	$(GOOSE) postgres "$(DATABASE_URL)" -dir migrations status
