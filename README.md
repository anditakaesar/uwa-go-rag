# Uwa Go Rag
Based on uwa go fullstack to implement the RAG server.

## Setup
Use golang 1.25

### Using goenv
- Install `curl -sfL https://raw.githubusercontent.com/go-nv/goenv/main/install.sh | bash`
- Install the required golang version `goenv install 1.25.5`

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
- Run `~/development$ docker compose up -d`. It uses port 5435, same as `Makefile` for migration

Database that is use is `PostgreSQL`. PostgreSQL is used here with these reasons:
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
- To seed initial users: run `$ make seed-database`. Seed file can be done using separate `.csv` files:
    - `db/seed/users.csv` contains seed data for `users` table
    - `db/seed/*.sql` contains seed data that executed directly into the database.
- To generate new mocks `$ make mockery`
- To test the code `$ make test`

### Env
Environment variables are located in file `.env-example`. This file eventualy will be `.env` on the deployemnt environment or loaded through different manners.
```
ENV=development
PORT=:3000
DB_URL=postgres://postgres:password@localhost:5433/backend_db?sslmode=disable
COOKIE_SECRET=Rm57qySVRliOZg5WqJ5GyKHKY6f4sJ41
CSRF_SECRET=4eQWYCt7WjxLwPmL06MhOW5FS96wxOk6
JWT_SECRET=4eQWYCt7WjxLwPmL06MhOW5FS96wxOk6
JWT_EXPIRE=15
UPLOAD_DIR=../../uploads
HOSTNAME=http://localhost:3000
CORS_OPT_AllowedOrigins=http://localhost:3000;http://localhost:8081;http://localhost:5173
CORS_OPT_AllowedMethods=GET;POST;PUT;DELETE;PATCH;OPTIONS
CORS_OPT_AllowedHeaders=Accept;Authorization;Content-Type;X-CSRF-Token
CORS_OPT_ExposedHeaders=Link;Set-Cookie
CORS_OPT_AllowCredentials=true
CORS_OPT_MaxAge=300
LOG_LEVEL=ERROR
```

### Run on TLS
The web server is able to run on TLS/Secure.
- Install `mkcert`
- Run `$ mkcert -install`
- Run `$ mkcert localhost`. This will geenerate *.pem files
- On web server, run on TLS by loading these *.pem files
```go
err := w.server.ListenAndServeTLS("localhost.pem", "localhost-key.pem")
```

## Docs `/docs`
- Run interactive API documentation using [`scalarapi/api-reference`](https://github.com/ScalaR/ScalaR) or `swagger/swagger-ui`. 

`$ podman run -d -p 8081:8080 -v "$(pwd)/docs/openapi:/docs" scalarapi/api-reference:latest`

- Open browser at `http://localhost:8081`
