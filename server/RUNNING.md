# Running Museum

The Docker Compose setup runs Museum, Postgres, and MinIO in an isolated cluster. This is the easiest way to get started.

You can also run the static Go binary directly on your machine.

This document covers both approaches and Museum's configuration.

- [Run using pre-built Docker images](docs/quickstart.md)
- [Run using Docker, building image from source](#build-and-run-using-docker)
- [Use individual pre-built images](#pre-built-images)
- [Run without Docker](#running-without-docker)
- [Configuration](#configuration)

## Run using pre-built Docker images

```sh
sh -c "$(curl -fsSL https://raw.githubusercontent.com/ente/ente/main/server/quickstart.sh)"
```

For more details, see [docs/quickstart.md](docs/quickstart.md).

## Build and run using Docker

From `ente/server`, start the cluster:

```sh
docker compose up --build
```

Once the cluster has started, call Museum:

```sh
curl http://localhost:8080/ping
```

Or connect from the [web app](../web):

```sh
NEXT_PUBLIC_ENTE_ENDPOINT=http://localhost:8080 npm run dev
```

Or connect from the [mobile app](../mobile):

```sh
flutter run --dart-define=endpoint=http://localhost:8080
```

Connect to Postgres:

```sh
docker compose exec postgres env PGPASSWORD=pgpass psql -U pguser -d ente_db
```

Use the MinIO S3 API:

```sh
AWS_ACCESS_KEY_ID=changeme AWS_SECRET_ACCESS_KEY=changeme1234 \
    aws s3 --endpoint-url http://localhost:3200 ls s3://b2-eu-cen
```

Or open the MinIO dashboard at http://localhost:3201.

> [!NOTE]
>
> To avoid exposing unnecessary services, this port is not exposed by default. You'll need to uncomment the corresponding port in your `compose.yaml` first.

> [!WARNING]
>
> The sample credentials are user `changeme` and password `changeme1234`. Change them before any non-local use.

> [!NOTE]
>
> For production, we recommend using external S3-compatible storage instead of the bundled MinIO service.

### Cleanup

Persistent data is stored in Docker volumes and survives container restarts. Inspect the volumes with `docker volume ls`.

To remove stopped containers, use `docker compose rm`. To also remove volumes, use `docker compose down -v`.

### Multiple clusters

You can spin up independent clusters, each with its own volumes, by using the `-p` Docker Compose flag to specify different project names for each one.

### Pruning images

Each source build creates a new image and leaves the old one dangling. Remove dangling images with `docker image prune --force`, or use `docker system prune` for a broader cleanup.

## Pre-built images

### Server

If you have provisioned Postgres and object storage separately, pull the server image from `ghcr.io/ente/server`.

```sh
docker pull ghcr.io/ente/server
```

### Web

The image at `ghcr.io/ente/web` contains all the web apps.

```sh
docker pull ghcr.io/ente/web
```

For details about configuring the web image, see [web/docs/docker.md](../web/docs/docker.md).

## Running without Docker

The following development setup runs Museum directly on macOS. Adapt the package installation commands for other operating systems.

### Install [Go](https://golang.org/dl/)

```sh
brew install go
```

### Install Postgres

```sh
brew install postgresql@15
```

> [!NOTE]
>
> This installs the same Postgres major version used in production. Newer versions should also work.

Run the service:

```sh
brew services run postgresql@15
```

> [!TIP]
>
> To stop, use
>
> ```sh
> brew services stop postgresql@15
> ```
>
> You can also tell brew to automatically start the service on login by using `start` instead of `run`
>
> ```sh
> brew services start postgresql@15
> ```

Create the database and user once:

```sh
psql postgres -c "CREATE USER pguser WITH PASSWORD 'pgpass';"
psql postgres -c "CREATE DATABASE ente_db OWNER pguser;"
```

> [!CAUTION]
>
> Since these are dev instructions, we're using the default username and password. For any non-trivial use, change the credentials.

> [!TIP]
>
> To inspect the DB, you can
>
> ```sh
> psql ente_db
> ```
### Install Local S3

If you don't have a test S3 bucket, you can run a S3 compatible API locally. This section outlines using minio. Garage is also a newer alternative.

```sh
brew install minio minio-mc
brew services run minio
```

Create the bucket once:

```sh
mc alias set local http://127.0.0.1:9000 minioadmin minioadmin
mc mb local/b2-eu-cen
mc ls local
```

> [!CAUTION]
>
> Since these are dev instructions, we're using the default username and password. For any non-trivial use, change the credentials.

### Create config

Create `museum.yaml` in `ente/server`.

```yaml
db:
    host: localhost
    port: 5432
    name: ente_db
    user: pguser
    password: pgpass

s3:
    are_local_buckets: true
    b2-eu-cen:
        key: minioadmin
        secret: minioadmin
        endpoint: localhost:9000
        region: eu-central-2
        bucket: b2-eu-cen
```

### Start museum

From `ente/server`,

```sh
go run ./cmd/museum
```

### Testing

For quick local iteration without Postgres-backed tests, run `go test ./...` or a narrower package like `go test ./pkg/controller/email`. Tests that require Postgres skip unless the test environment is explicitly enabled.

To run the full `server/` Go test suite, including Postgres-backed tests, use:

```sh
./scripts/test-with-postgres.sh docker
./scripts/test-with-postgres.sh host -v
```

- `docker` runs the tests against a throwaway Docker Postgres
- `host` runs them against the existing Postgres on your machine
- any other args after `<docker|host>` are forwarded to the `go test` invocation

The script creates a temporary `ente_test_*` database on the selected Postgres instance, runs the tests, and drops the database when it exits.

The database and repository integration tests also run against a disposable libSQL
HTTP server. Docker and curl are required:

```sh
./scripts/test-with-libsql.sh
```

These cover binding, commit/rollback, migration atomicity, dirty-version handling,
repository behavior, and concurrency on libSQL.

### Choosing PostgreSQL or Turso

PostgreSQL remains the default. Set `db.provider: turso` or
`ENTE_DB_PROVIDER=turso` to use Turso's libSQL service. Museum repositories select
explicit queries for the configured provider; libSQL stores JSON and arrays as
JSON, encrypted byte values as BLOBs, and IDs and timestamps as 64-bit integers.

From `server/`, verify a hosted libSQL connection with:

```sh
export ENTE_DB_PROVIDER=turso
export ENTE_DB_TURSO_URL=libsql://your-database.turso.io
# Set ENTE_DB_TURSO_AUTH_TOKEN through your shell or secret manager.
go run ./cmd/database
```

The token is a separate setting; credentials and query parameters in the URL are
rejected. Remote connections require TLS. Local development may use
`http://127.0.0.1:8080` without a token. This targets Turso's libSQL service using
its [official Go client](https://github.com/tursodatabase/libsql-client-go).

`go run ./cmd/database -migrate` selects `migrations/postgres` or
`migrations/sqlite`. PostgreSQL's existing migration files and versions are
unchanged. The SQLite baseline includes all Museum and Space tables. Future
changes must update both tracks, with matching
versions and an explicit no-op where only one backend changes. Changing the
provider does not transfer data between databases.

Run migrations before starting Museum. Turso migration runners acquire a database
lease; a crashed runner's lease expires after ten minutes.
Each SQLite migration runs in one transaction. Do not include explicit transaction
control statements in the SQL file. Turso has an
[HTTP transaction limit](https://docs.turso.tech/sdk/http/reference)
of five seconds. Larger migrations need to be split into bounded steps. A failed
migration rolls back its SQL but leaves the version dirty for manual inspection.

To move an existing installation between PostgreSQL and libSQL, stop Museum and
all jobs first. Keep the same Museum encryption, hashing, and JWT keys and the
same object storage. Configure the source database, then export it:

```sh
go run ./cmd/database -export /private/path/museum.snapshot.jsonl
```

Configure the empty destination database, migrate it to the same version, and
import the snapshot:

```sh
go run ./cmd/database -migrate
go run ./cmd/database -import /private/path/museum.snapshot.jsonl
```

The snapshot contains private account data and is created with mode `0600`.
Export refuses to overwrite a file. Import rejects populated destinations and
schema mismatches, keeps constraints enabled, verifies row checksums before
committing, and restores identity sequences. The source database remains intact.
Verify the installation before restarting Museum against the destination.
Remote libSQL imports must fit its transaction deadline; use a local libSQL
instance and the provider's supported migration or replication workflow for
larger datasets.

## Configuration

By default, Museum uses the values in `configurations/local.yaml`.

To override these values, you can create a file named `museum.yaml` in the current directory. This path is git-ignored for convenience. Note that if you run the Docker compose cluster without creating this file, Docker will create an empty directory named `museum.yaml` which you can `rmdir` if you need to provide a config file later on.

The keys and values supported by this configuration file are documented in [configurations/local.yaml](configurations/local.yaml).

> [!TIP]
>
> If your mobile app can connect to your self-hosted instance but cannot view or upload images, see [ente.com/help/self-hosting/administration/object-storage](https://ente.com/help/self-hosting/administration/object-storage).

For Cloudflare Workers, see [the Workers deployment guide](workers/README.md).
