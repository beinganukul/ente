#!/bin/sh

# Runs all server contracts against libSQL, schema parity, and offline transfer.
set -eu
cd "$(dirname "$0")/.."

container="ente-server-test-libsql-$$"
postgres="ente-server-test-libsql-postgres-$$"
trap 'docker rm -f "$container" "$postgres" >/dev/null 2>&1 || true' EXIT INT TERM
docker run --detach --name "$postgres" \
    --env POSTGRES_DB=ente_test_schema --env POSTGRES_PASSWORD=test_pass \
    --publish 127.0.0.1::5432 postgres:15 >/dev/null
pgport=$(docker port "$postgres" 5432/tcp)
pgport=${pgport##*:}
i=0
until docker exec "$postgres" pg_isready -q -h localhost -U postgres; do
    i=$((i + 1))
    [ "$i" -lt 30 ] || { echo "Postgres not ready after 30s." >&2; exit 1; }
    sleep 1
done
dsn="host=127.0.0.1 port=$pgport user=postgres password=test_pass dbname=ente_test_schema sslmode=disable"
docker run --detach \
    --name "$container" \
    --publish 127.0.0.1::8080 \
    ghcr.io/tursodatabase/libsql-server@sha256:6dd3eb276d9d3604e4a48ac4a999a2e267814732d57d7e94c04ba71482333a67 >/dev/null
port=$(docker port "$container" 8080/tcp)
url="http://127.0.0.1:${port##*:}"
i=0
until curl --fail --silent "$url/health" >/dev/null; do
    i=$((i + 1))
    [ "$i" -lt 30 ] || { echo "libSQL not ready after 30s." >&2; exit 1; }
    sleep 1
done

ENV=test ENTE_TEST_TURSO_URL="$url" ENTE_TEST_POSTGRES_DSN="$dsn" \
    CGO_ENABLED=0 go test -p 1 -count=1 "$@" ./...

# Transfer requires empty targets and separate databases from the contracts.
docker exec "$postgres" createdb -U postgres ente_transfer_source
docker exec "$postgres" createdb -U postgres ente_transfer_target
docker rm -f "$container" >/dev/null
docker run --detach --name "$container" --publish 127.0.0.1::8080 \
    ghcr.io/tursodatabase/libsql-server@sha256:6dd3eb276d9d3604e4a48ac4a999a2e267814732d57d7e94c04ba71482333a67 >/dev/null
port=$(docker port "$container" 8080/tcp)
url="http://127.0.0.1:${port##*:}"
i=0
until curl --fail --silent "$url/health" >/dev/null; do
    i=$((i + 1))
    [ "$i" -lt 30 ] || { echo "libSQL not ready after 30s." >&2; exit 1; }
    sleep 1
done
ENTE_TRANSFER_TEST_PG_SOURCE="${dsn%dbname=*}dbname=ente_transfer_source sslmode=disable" \
    ENTE_TRANSFER_TEST_PG_TARGET="${dsn%dbname=*}dbname=ente_transfer_target sslmode=disable" \
    ENTE_TRANSFER_TEST_TURSO_URL="$url" CGO_ENABLED=0 \
    go test -count=1 -run TestTransferPostgresTursoRoundTrip ./pkg/utils/database
