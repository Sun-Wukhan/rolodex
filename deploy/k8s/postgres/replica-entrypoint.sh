#!/bin/sh
# Runs a read-only streaming replica (hot standby) of the profiles primary.
#
# On every start it makes sure the replication role exists on the primary with
# the current password; on first start (empty PGDATA) it clones the primary
# with pg_basebackup. The password is kept in a passfile under /tmp rather
# than in postgresql.auto.conf or on the command line.
#
# Required env: PRIMARY_HOST, POSTGRES_USER, POSTGRES_PASSWORD, POSTGRES_DB,
# REPLICATION_USER, REPLICATION_PASSWORD, PGDATA, HBA_FILE.
set -eu

: "${PRIMARY_HOST:?}" "${POSTGRES_USER:?}" "${POSTGRES_PASSWORD:?}" "${POSTGRES_DB:?}"
: "${REPLICATION_USER:?}" "${REPLICATION_PASSWORD:?}" "${PGDATA:?}" "${HBA_FILE:?}"

passfile=/tmp/replication.pgpass
escape() { printf '%s' "$1" | sed 's/[\\:]/\\&/g'; }
umask 077
printf '*:*:*:%s:%s\n' "$(escape "$REPLICATION_USER")" "$(escape "$REPLICATION_PASSWORD")" >"$passfile"

until pg_isready -q -h "$PRIMARY_HOST" -U "$POSTGRES_USER" -d "$POSTGRES_DB"; do
	echo "waiting for primary $PRIMARY_HOST"
	sleep 2
done

PGPASSWORD="$POSTGRES_PASSWORD" psql -q -h "$PRIMARY_HOST" -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
	-v ON_ERROR_STOP=1 -v role="$REPLICATION_USER" -v pass="$REPLICATION_PASSWORD" <<'SQL'
SELECT format('CREATE ROLE %I', :'role') WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'role') \gexec
SELECT format('ALTER ROLE %I WITH REPLICATION LOGIN PASSWORD %L', :'role', :'pass') \gexec
SQL

if [ ! -s "$PGDATA/PG_VERSION" ]; then
	echo "cloning primary $PRIMARY_HOST into $PGDATA"
	rm -rf "${PGDATA:?}"/* 2>/dev/null || true
	PGPASSFILE="$passfile" pg_basebackup -h "$PRIMARY_HOST" -U "$REPLICATION_USER" \
		-D "$PGDATA" -X stream --checkpoint=fast --no-password
	touch "$PGDATA/standby.signal"
fi

exec postgres \
	-c hba_file="$HBA_FILE" \
	-c hot_standby=on \
	-c primary_conninfo="host=$PRIMARY_HOST user=$REPLICATION_USER passfile=$passfile application_name=$(hostname)"
