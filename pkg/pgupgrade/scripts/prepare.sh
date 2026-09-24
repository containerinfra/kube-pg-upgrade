#!/bin/sh
set -eu

OLD_DATA="${OLD_DATA:-/old/data}"
NEW_DATA="${NEW_DATA:-/new/data}"

mkdir -p "${OLD_DATA}" "${NEW_DATA}"

# we require a postgresql config file to exist
touch "${OLD_DATA}/postgresql.conf"

# if an old postmaster.pid is still present, remove it
rm -f "${OLD_DATA}/postmaster.pid"

# Make sure postgresql.conf is located in the old directory.
# Bitnami installations do not have this at the default location:
# https://docs.bitnami.com/aws/infrastructure/postgresql/get-started/understand-default-config/
# this postgresql.conf is temporary and is only necessary for pg_upgrade.
touch "${OLD_DATA}/pg_hba.conf"
echo "local all all trust" > "${OLD_DATA}/pg_hba.conf"
echo "host all all all md5" >> "${OLD_DATA}/pg_hba.conf"

run_pg_ctl() {
	if [ "$(id -u)" = "0" ]; then
		su postgres -c "$*"
	else
		# Non-root: run as the configured UID (e.g. Bitnami 1001 / Docker Hub 999).
		# FSGroup owns the volume; do not chown to the image's postgres user.
		sh -c "$*"
	fi
}

# fix permissions so we can start postgres (root only; non-root relies on FSGroup)
if [ "$(id -u)" = "0" ]; then
	chown postgres "${OLD_DATA}" -R
fi

# Fix source cluster was not shut down cleanly
run_pg_ctl "${PGBINOLD}/pg_ctl start -w -D ${OLD_DATA}"
run_pg_ctl "${PGBINOLD}/pg_ctl stop -w -D ${OLD_DATA}"

# Show database size
echo database size:
df -h "${OLD_DATA}"
