#!/bin/bash

echo "validating the database is able to start..."

# validate we are able to start the database
su postgres -c "${PGBINNEW}/pg_ctl start -w -D /new"
su postgres -c "${PGBINNEW}/pg_ctl stop -w -D /new"

# Show database size
echo database size:
df -h /new

echo "completed posthook script.."
