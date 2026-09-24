#!/bin/bash

echo "validating the database is able to start..."

run_pg_ctl() {
	if [ "$(id -u)" = "0" ]; then
		su postgres -c "$*"
	else
		# Non-root: run as the configured UID (e.g. Bitnami 1001 / Docker Hub 999).
		sh -c "$*"
	fi
}

# validate we are able to start the database
run_pg_ctl "${PGBINNEW}/pg_ctl start -w -D /new"
run_pg_ctl "${PGBINNEW}/pg_ctl stop -w -D /new"

# Show database size
echo database size:
df -h /new

echo "completed posthook script.."
