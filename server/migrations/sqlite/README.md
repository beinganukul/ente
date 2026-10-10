# Turso migration track

This directory contains Museum's libSQL schema. Migration 151 creates the
baseline matching PostgreSQL migration 151, including Space tables, constraints,
indexes, generated columns, and triggers. Later migrations use the same logical
version numbers as the PostgreSQL track.

Keep PostgreSQL migration files and version numbers unchanged in `../postgres`.
Future logical schema changes need migrations in both directories. Backend-only
changes need an intentional no-op in the other track.

Run migrations before starting Museum. A database lease prevents concurrent
libSQL migration runners; a crashed runner's lease expires after ten minutes.
Each migration file runs in a transaction and must fit the remote service's
transaction deadline. Failed migrations roll back their SQL and leave a dirty
version for inspection.
