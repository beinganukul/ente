# Museum on Cloudflare Workers

This builds the existing Museum API as Go WebAssembly using
[workers-go](https://github.com/syumai/workers-go). A singleton SQLite-backed
Durable Object hosts Museum, its database pools, durable rate limits, and alarm
scheduler. The clients continue to use the existing API routes.

The outer Worker uses Workers Free's default 10 ms CPU budget. The Go API runs
inside a Durable Object, whose [default CPU budget is 30 seconds per invocation](https://developers.cloudflare.com/durable-objects/platform/limits/),
including alarms. The configuration does not request a custom CPU allowance or
upgrade the account plan. CPU usage has not been benchmarked for production.
The uncompressed bundle is approximately 62 MiB.
Cloudflare's [size and memory limits](https://developers.cloudflare.com/workers/platform/limits/)
apply. The build rejects artifacts at or above 64 MiB. Do not replace the pinned
workers-go revision without checking the generated runtime and running the tests.

The pinned Miniflare dependency overrides `sharp` to 0.35.5 to fix
[GHSA-wq5f-xc86-pv6w](https://github.com/advisories/GHSA-wq5f-xc86-pv6w).
Remove the override when the pinned upstream dependency includes this patch.

## Build and configure

Install Go matching `server/go.mod` and Node.js supported by Wrangler:

```sh
cd server/workers
npm ci
npm run build
```

The committed `wrangler.jsonc` uses a workers.dev URL. For personal domains or
other deployment settings, copy it to the ignored `wrangler.local.jsonc` and
add your route there. Pass `--config wrangler.local.jsonc` to Wrangler commands,
including secret updates and deployment. Preserve the existing Worker name,
Durable Object binding, and migration history when updating a deployment.

Configuration uses the existing Museum YAML schema and `ENTE_` environment
variables. `MUSEUM_CONFIG` is an optional secret containing the contents of
`museum.yaml`. Only string bindings beginning with `ENTE_` are copied into Go's
environment. File bindings are staged in the request's in-memory `/tmp` filesystem.

Museum requires explicit secrets; the example YAML keys are never accepted as
Worker defaults:

```sh
npx wrangler secret put ENTE_KEY_ENCRYPTION
npx wrangler secret put ENTE_KEY_HASH
npx wrangler secret put ENTE_JWT_SECRET
npx wrangler secret put MUSEUM_CONFIG
```

For an existing deployment, **reuse its encryption, hashing, and JWT keys**.
Changing these keys can make existing account data inaccessible. For a new
installation, generate keys using `go run tools/gen-random-keys/main.go` from
`server` and copy the values into the three secrets.

Example `MUSEUM_CONFIG` (substitute your real settings):

```yaml
db:
  provider: postgres
  host: database.example.com
  port: 5432
  user: museum
  name: museum
  password: YOUR_DATABASE_PASSWORD
  sslmode: verify-full
smtp:
  host: smtp.example.com
  port: 587
  username: YOUR_SMTP_USERNAME
  password: YOUR_SMTP_PASSWORD
apps:
  public-albums: https://albums.example.com
  accounts: https://accounts.example.com
```

Merge your existing S3, app URL, subscription, and other settings into this YAML.
For Turso, replace the `db` section with:

```yaml
db:
  provider: turso
  turso:
    url: libsql://YOUR_DATABASE.turso.io
```

Set `ENTE_DB_TURSO_AUTH_TOKEN` with `npx wrangler secret put`; keep it outside
public configuration. You can also configure `ENTE_DB_PROVIDER=turso` and
`ENTE_DB_TURSO_URL` in Wrangler `vars`. The Worker uses Turso's HTTP transport;
it does not require a PostgreSQL TCP connection. Each provider requires its
own migration track to be applied before deployment.

Public bucket names and other nonsecret settings may instead go into `vars` in
`wrangler.jsonc`. Store passwords and storage credentials as secrets. PostgreSQL
must be reachable from Cloudflare's TCP socket API; loopback addresses work only
in local development. TLS is verified with bundled public roots; `CA_CERTIFICATES`
can supply additional PEM roots for a private database or SMTP certificate.

Optional file bindings:

| Binding | Contents |
| --- | --- |
| `FCM_SERVICE_ACCOUNT` | Firebase service account JSON |
| `PLAYSTORE_SERVICE_ACCOUNT` | Play Store service account JSON |
| `BILLING_US` / `BILLING_IN` | Existing `data/billing/us.json` / `in.json` |
| `BILLING_OFFERS` | Existing `data/billing/black-friday.json` |

Each secret/text binding has a 5 KB limit. Larger public pricing files can be
placed in `server/data/billing` before building; the three named JSON files above
are bundled when present. Never put service account keys in these public files.

Keep `credentials-dir` and `billing-config-dir` at their defaults for these file
bindings. Incoming HTTPS is terminated by Cloudflare. SMTP uses ports 465
(`smtp.encryption: tls`) or 587 (STARTTLS); Cloudflare blocks port 25.

Database-backed health can work before SMTP and storage are configured. To use
email signup/login, configure `smtp.host`, `smtp.port`, `smtp.email`, and any
required SMTP username/password. Without SMTP, the shared Museum email helper
skips delivery, so no verification code reaches users. To upload photos,
configure the primary `s3.b2-eu-cen` bucket's `key`, `secret`, `endpoint`, `region`,
and `bucket`. These identifiers support arbitrary S3-compatible providers,
including Cloudflare R2. Space media additionally needs `space.assets.primaryBucket`.

For a single R2 bucket, start from [museum.r2.example.yaml](museum.r2.example.yaml)
and configure the bucket CORS policy using [r2-cors.json](r2-cors.json). Uploads
are presigned S3 requests made by clients; the Worker needs a bucket-scoped R2
S3 access key and secret, even if the bucket belongs to the same Cloudflare
account. Store the key/secret in Worker secrets, never in the public example.

## Migrate and deploy

### Deploy the full stack on Workers

The `Deploy (Cloudflare Workers)` GitHub workflow publishes Museum, all 15 web
apps, and the documentation site to Workers. Frontends use Workers Static
Assets with separate origins; no Pages project or Next.js server adapter is
required. The inherited Pages publishing workflows run only in `ente/ente`.

In your fork's Actions settings, add `CLOUDFLARE_API_TOKEN` as a secret using
Cloudflare's **Edit Cloudflare Workers** template, scoped to your account and
any zone used by Museum's custom domain. Add these repository variables:

| Variable | Value |
| --- | --- |
| `CLOUDFLARE_ACCOUNT_ID` | Your existing Cloudflare account ID |
| `ENTE_WORKERS_API_ORIGIN` | Museum's HTTPS origin, without a path |
| `ENTE_WORKERS_SUBDOMAIN` | Your account's workers.dev subdomain |
| `ENTE_WEB_WORKER_PREFIX` | `ente-web`, or your own Worker name prefix |
| `ENTE_WORKERS_DEPLOY` | `true`, after credentials and Museum are configured |

Push relevant changes to `main`, or dispatch **Deploy (Cloudflare Workers)** on
`main`. The workflow builds all exports before publishing, retains Museum's
Worker name and Durable Object migration history, and leaves existing secrets
in Cloudflare. Provision the database, storage, SMTP, and encryption keys first
using the steps below; the workflow does not recreate those resources or run
database migrations.

Each frontend is served at
`https://<prefix>-<app>.<subdomain>.workers.dev`, including `photos`, `auth`,
`accounts`, `albums`, `cast`, `embed`, `ensu`, `legacy`, `locker`, `memories`,
`paste`, `payments`, `share`, `space`, `twoof3`, and `docs`. Builds use Museum's
origin for both Next.js and Vite API settings. Museum's public app URLs are
updated to these frontend origins.

For a local deployment with the same settings, install the existing dependencies,
build each app, and run `node deploy.mjs <app>` from `server/workers`. Use
`node deploy.mjs museum` for the backend. `--dry-run` validates a bundle without
publishing. The helper writes public configuration under ignored `.wrangler/`.
It preserves exported `_headers` and routing rules, replacing catch-all
`index.html` rewrites with Workers SPA fallback to avoid redirect loops.

These deployments do not provision Stripe or AI providers. Configure those
services separately before using payment or remote AI features. For account
passkeys, configure the accounts Worker origin and a compatible WebAuthn RP ID;
preserve any existing RP ID and migration settings when moving accounts domains.

Apply database migrations using the native command with the same database
configuration **before** deploying. The Worker checks that the migration table
exists, is clean, and includes this build's latest migration; it does not run migrations itself.

```sh
# From server, configured through museum.yaml or ENTE_DB_* variables:
go run ./cmd/database -migrate

# From server/workers:
npx wrangler login
npm run deploy
```

With a personal configuration, deploy using
`npm run deploy -- --config wrangler.local.jsonc`.

Set each client's Museum endpoint to the resulting Worker URL. The repository's
native server remains available with `go run ./cmd/museum`.

## Switch an existing database

Stop every Museum instance first. Back up the original database and its Museum
encryption/hash/JWT keys. Keep the same keys when switching an existing
installation; the transfer does not decrypt or reencrypt account data.

From `server`, select the source using `museum.yaml` / `ENTE_DB_*` and export:

```sh
go run ./cmd/database -export /private/path/museum.jsonl
```

Select a **new empty target**, migrate its selected provider, then import:

```sh
go run ./cmd/database -migrate
go run ./cmd/database -import /private/path/museum.jsonl
```

The snapshot is created with mode `0600` and existing files are never overwritten.
Protect it as a database backup. Import requires matching clean schema versions
and identical application table/column sets. It verifies row counts, typed data
checksums and foreign keys before committing, retains triggers/constraints, and
restores identity high-water marks. Binary values, 64-bit IDs, NULLs, JSON,
timestamps and enum arrays preserve their database meaning. Unsupported types,
dependency cycles, truncated files, nonempty targets, and trigger-induced data
changes abort the import. The original database remains available for rollback.
Switch configuration and restart Museum only after a successful import and
validation. Object storage is not copied by this command.

Transfer uses a single database transaction and pages rows in groups of 256;
remote database transaction/response quotas still apply. Large real datasets
need validation against the target's limits before switching production traffic.

## Runtime behavior

- The Durable Object removes `Accept-Encoding` before passing requests to Go,
  so Cloudflare handles response compression. This avoids compression metadata
  changing across Durable Object fetch boundaries. Native Museum retains its
  existing gzip middleware.
- Failed startup resets the Durable Object's Go runtime before retrying, so
  partially registered collectors and database pools cannot poison later requests.
  Durable storage, rate limits, and scheduled job times survive the reset.
- Durable Object alarms replace perpetual cleanup/replication goroutines and
  cron timers. Each invocation runs one oldest due job, with smaller Workers
  batches for cleanup queues. Cron due times survive restarts. Scheduled jobs
  are first activated by a request.
- The final account cleanup and stage advancement use one database transaction, so failures
  roll back earlier deletes and connection setup does not consume a subrequest
  for every table.
- Detached request work uses `waitUntil`. It still shares Cloudflare's event
  lifetime limits; long jobs belong in the alarm scheduler.
- Turso connections are retired when returned to the SQL pool, since a
  successful response can close its Hrana stream without exposing that state
  to the driver. Transactions keep their connection until commit or rollback.
  This adds a connection setup round trip per autocommit operation.
- Object replication streams from S3 into a single buffered multipart upload,
  with source-length checks and destination HEAD verification before marking a
  replica complete. No entire object is written into `/tmp`. Each part is
  limited to 16 MiB; with Scaleway's 1,000-part limit this supports objects up to
  about 15.6 GiB. Larger objects need an external copier.
- The outer Worker overwrites the trusted client-IP header from
  `CF-Connecting-IP`; clients cannot supply the header used by Museum directly.
- Play Store uses the same verification/acknowledgement/cancellation endpoints
  through a small Worker client to keep the Wasm module within the size limit.
- Web push retains address validation and pins connections to public DNS
  results, using DNS-over-HTTPS and Workers sockets.

This deployment uses one Museum Durable Object. It is a starting point for
self-hosting; production throughput and Cloudflare CPU/memory limits still need
validation with your workload. Turso selection uses the shared database package
and the SQLite migration track; PostgreSQL retains its existing migration track.

Workers Free allows [50 external subrequests per invocation](https://developers.cloudflare.com/durable-objects/platform/limits/),
including Durable Object alarms. Queue batches are reduced for Workers; native Museum keeps its
existing batch sizes. Some jobs still do unbounded work within one account,
including account deletion, inactivity checks, and storage warnings. A large
account or backlog can exceed these limits despite a small queue batch. Use a
native Museum instance for maintenance at that scale until these jobs support
resumable steps. This is an experimental deployment target, not a production
capacity guarantee.

## Verify locally

Install OpenSSL (used for the test SMTP certificate). Create a **disposable** PostgreSQL database named `ente_test_workers`, configure
its localhost port, and migrate it with `cmd/database`. Tests create accounts in
that database; never point them at production.

```sh
cd server/workers
npm run build
npm run test:wasm
ENTE_WORKERS_TEST_PG_PORT=5432 npm test
```

Alternatively, migrate a disposable local libSQL server and run
`ENTE_WORKERS_TEST_TURSO_URL=http://127.0.0.1:8080 npm test`. The Turso runtime
test rejects remote URLs to prevent test-account writes to hosted databases.

The test exercises database-backed health, routing, CORS, SMTP delivery to a local
fake TLS server, email verification/signup, account and collection APIs, presigned
uploads to a local S3 mock, Museum's authenticated outbound object-size checks,
file registration and signed download byte equality, session revocation, startup
recovery with durable state preserved, alarm
execution, and durable rate limits after a Worker restart. The S3 mock checks
request flow; production storage credentials and provider behavior need separate
verification. The test harness exists only in ignored `.wrangler` output, and is
not included in the deployed Worker.

## Check the deployed API

```sh
cd server/workers
npm run test:live -- https://museum.example.com
```

Supply an explicit HTTPS origin. This checks default, identity, gzip, and Brotli
ping decoding, unauthenticated 401, a unique nonexistent `@example.com` SRP
lookup returning 404, routing, and CORS. Ten rounds of four concurrent requests
use database-backed pings, followed by a 70-second pause and another SRP lookup
and ping. Requests time out after 30 seconds. Output includes
sanitized stages, statuses, transport identifiers, and wall latencies; these do
not measure CPU or memory. Response bodies, URLs, and error messages are omitted.
The check sends no emails and creates no accounts, collections, or S3 objects.
Normal request rate-limit counters still advance. Only two SRP lookups are sent
per run, respecting that route's 10-per-minute limit; repeated runs can still
reach API limits or the missing-SRP limiter's 100-per-hour allowance.

After deployment, inspect `npx wrangler tail ente-museum --format json` and
confirm the **Museum** entrypoint's `scriptVersion.id` matches the deployed
version before testing a fix. The outer Worker and Durable Object can run
different versions for seconds to minutes during [eventually consistent code updates](https://developers.cloudflare.com/durable-objects/platform/known-issues/#code-updates).
A passing health check alone does not prove the new Go code is serving requests
or that signup, large uploads, or production workloads are stable.
