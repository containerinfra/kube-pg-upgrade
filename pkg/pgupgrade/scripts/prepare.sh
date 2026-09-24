#!/bin/sh

# we require a postgresql config file to exist
touch /old/postgresql.conf

# if an old postmaster.pid is still present, remove it
rm -f /old/postmaster.pid

# TODO: use --old-datadir=configdir 

# Make sure postgresql.conf is located in the old directory.
# Bitnami installations do not have this at the default location:
# https://docs.bitnami.com/aws/infrastructure/postgresql/get-started/understand-default-config/
# this postgresql.conf is temporarly and is only necessary to be present in order to allow pg_upgrade to function.
# TODO: check if file is present, if not, do this:
touch /old/pg_hba.conf
echo "local all all trust" > /old/pg_hba.conf
echo "host all all all md5" >> /old/pg_hba.conf

# create default pg_hba file in new location also, optional (cli flag required)
# touch /new/pg_hba.con
# host     all             all             0.0.0.0/0               md5
# host     all             all             ::/0                    md5
# local    all             all                                     md5
# host      replication     all             0.0.0.0/0               md5

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
	chown postgres /old -R
fi

# Fix source cluster was not shut down cleanly
run_pg_ctl "${PGBINOLD}/pg_ctl start -w -D /old"
run_pg_ctl "${PGBINOLD}/pg_ctl stop -w -D /old"

# Show database size
echo database size:
df -h /old
