include .env
export

TOOLS_MIGRATE="$$(pwd)/tools/migrate"
TOOLS_MOCKERY="$$(pwd)/tools/mockery"
MIGRATION_PATH="$$(pwd)/db/migrations"
DEV_DATABASE="postgres://postgres:password@localhost:5435/backend_db?sslmode=disable"

create-migration:
	$(TOOLS_MIGRATE) create -ext sql -dir $(MIGRATION_PATH) -seq $(name)

migrate:
	$(TOOLS_MIGRATE) -path $(MIGRATION_PATH) -database $(DEV_DATABASE) up

migrate-down:
	$(TOOLS_MIGRATE) -path $(MIGRATION_PATH) -database $(DEV_DATABASE) down 1

seed-database:
	DB_URL=$(DEV_DATABASE) go run ./cmd/seed/main.go

mockery:
	$(TOOLS_MOCKERY)

test:
	go test -v ./internal/...

run:
	go run ./cmd/web