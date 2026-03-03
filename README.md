# Uwa Go Rag
Based on uwa go fullstack to implement the RAG server.

## Setup
Use golang 1.25
Libraries:
- github.com/go-chi/chi/v5
- github.com/gorilla/csrf
- github.com/gorilla/sessions
- github.com/jackc/pgx/v5
- golang.org/x/crypto/argon2
- golang-migrate

Run `$ go mod vendor` to resolve references

Prepare [`mockery`](https://github.com/vektra/mockery/releases) and [`migrate`](https://github.com/golang-migrate/migrate/releases). Place those under `./tools`

### Database
Database can be deployed using podman/docker on `development` folder.
`~/development$ docker compose up -d`
Database that is used is `PostgreSQL`. PostgreSQL is used here with these reasons:
- I expect the schema of this project to evolve given time.
- I rely on complex queries such as to aggregate. This means that I pushed logic near the data related to it.
- This is the safer long-term database because its extensibility. Who knows I might need it to use extension like PostGIS (geospatial) or simply use its capability for full-text search and query-able JSONB data type.
- Install `river`: `$ go install github.com/riverqueue/river/cmd/river@latest`
- Migrate river: `$ river migrate-up --database-url "$DATABASE_URL"`

### Migration
Check `Makefile`

- To create migration: `$ make create-migration name=new_migration`
- To migrate all pending migration: `$ make migrate`
- To migrate down one previous migration: `$ make migrate-down`
- To seed initial users: run `$ make seed-database` OR run manually
`$ DB_URL="postgres://here" go run ./cmd/seed/main.go`. Seed file can be done using separate `.csv` files:
    - `cmd/seed/users.csv` contains seed data for `users` table
- To generate new mocks `$ make mockery`
- To test the code `$ make test`

### Env
Environment variables are located in file `.env-example`. This file eventualy will be `.env` on the deployemnt environment or loaded through different manners.
```
ENV="development"
PORT=":3000"
DB_URL="postgres://postgres:password@localhost:5433/backend_db?sslmode=disable"
COOKIE_SECRET="Rm57qySVRliOZg5WqJ5GyKHKY6f4sJ41"
CSRF_SECRET="4eQWYCt7WjxLwPmL06MhOW5FS96wxOk6"
JWT_SECRET="4eQWYCt7WjxLwPmL06MhOW5FS96wxOk6"
UPLOAD_DIR="../../uploads"
HOSTNAME="http://localhost:3000"
```

## Docs `/docs`
- TBD