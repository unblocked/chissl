# Database

Persistent storage for users, tunnels, and sessions.

## Options
- SQLite (default): simple file DB, great for single-node setups
- PostgreSQL: production-ready, multi-node friendly

## Quick start
```bash
# SQLite (default)
chissl server --db-type sqlite --db-file ./chissl.db ...

# PostgreSQL
chissl server \
  --db-type postgres \
  --db-host localhost \
  --db-port 5432 \
  --db-name chissl \
  --db-user chissl_user \
  --db-pass secure_password \
  --db-ssl require ...
```

## Environment variables
```bash
export CHISSL_DB_TYPE=postgres
export CHISSL_DB_HOST=localhost
export CHISSL_DB_PORT=5432
export CHISSL_DB_NAME=chissl
export CHISSL_DB_USER=chissl_user
export CHISSL_DB_PASS=secure_password
export CHISSL_DB_SSL=require

chissl server ...
```

## Notes
- SQLite is ideal to start; migrate to PostgreSQL for scale/high availability
- Backups: copy the SQLite file or use standard pg_dump/psql for PostgreSQL
- Ensure DB connectivity from the server and restrict access via firewall/VPC
