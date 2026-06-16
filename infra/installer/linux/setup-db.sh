#!/usr/bin/env bash
# Called by install.sh to initialise PostgreSQL, create the tpt DB/user,
# write final config.yaml, and generate MinIO credentials.
#
# Usage: setup-db.sh <tpt-home> <admin-email> <admin-password> <db-password>
set -euo pipefail

TPT_HOME="${1:?Usage: setup-db.sh <tpt-home> <admin-email> <admin-password> <db-password>}"
ADMIN_EMAIL="${2:?}"
ADMIN_PASSWORD="${3:?}"
DB_PASSWORD="${4:?}"

TPT_USER="tpt"
PGSQL_BIN="${TPT_HOME}/pgsql/bin"
DATA_DIR="${TPT_HOME}/data/pgsql"
CONFIG_DIR="${TPT_HOME}/config"
LOGS_DIR="${TPT_HOME}/logs"

export LD_LIBRARY_PATH="${TPT_HOME}/pgsql/lib${LD_LIBRARY_PATH:+:${LD_LIBRARY_PATH}}"

# -- Generate secrets ---------------------------------------------------------
JWT_SECRET=$(openssl rand -hex 48)
MINIO_SECRET=$(openssl rand -hex 32)

# -- Initialise PostgreSQL cluster --------------------------------------------
if [[ ! -f "${DATA_DIR}/PG_VERSION" ]]; then
    echo "  initdb: creating cluster at ${DATA_DIR}"
    mkdir -p "${DATA_DIR}"
    chown "${TPT_USER}:${TPT_USER}" "${DATA_DIR}"
    sudo -u "${TPT_USER}" "${PGSQL_BIN}/initdb" \
        --pgdata="${DATA_DIR}" \
        --encoding=UTF8 \
        --username=postgres \
        --auth-local=trust \
        --auth-host=md5
fi

# -- Start PostgreSQL temporarily to create DB and user -----------------------
mkdir -p "${LOGS_DIR}/postgresql"
chown -R "${TPT_USER}:${TPT_USER}" "${LOGS_DIR}/postgresql" "${DATA_DIR}"

sudo -u "${TPT_USER}" "${PGSQL_BIN}/pg_ctl" start \
    -D "${DATA_DIR}" \
    -l "${LOGS_DIR}/postgresql/setup.log" \
    -w -t 60

for i in $(seq 1 30); do
    sudo -u "${TPT_USER}" "${PGSQL_BIN}/pg_isready" -h localhost -p 5432 &>/dev/null && break
    sleep 1
done
sudo -u "${TPT_USER}" "${PGSQL_BIN}/pg_isready" -h localhost -p 5432 \
    || { echo "ERROR: PostgreSQL did not become ready"; exit 1; }

# -- Create tpt role and database ---------------------------------------------
sudo -u "${TPT_USER}" "${PGSQL_BIN}/psql" -U postgres -h localhost <<SQL
DO \$\$ BEGIN
  IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname='tpt') THEN
    CREATE ROLE tpt LOGIN PASSWORD '${DB_PASSWORD}';
  END IF;
END \$\$;
SELECT 'CREATE DATABASE tpt OWNER tpt'
  WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname='tpt')\gexec
SQL

# -- Stop temporary PostgreSQL (systemd takes over) ---------------------------
sudo -u "${TPT_USER}" "${PGSQL_BIN}/pg_ctl" stop -D "${DATA_DIR}" -m fast

# -- Write config.yaml from template ------------------------------------------
cp "${CONFIG_DIR}/config.yaml.template" "${CONFIG_DIR}/config.yaml"
sed -i \
    -e "s|{{ADMIN_EMAIL}}|${ADMIN_EMAIL}|g" \
    -e "s|{{ADMIN_PASSWORD}}|${ADMIN_PASSWORD}|g" \
    -e "s|{{DB_PASSWORD}}|${DB_PASSWORD}|g" \
    -e "s|{{JWT_SECRET}}|${JWT_SECRET}|g" \
    -e "s|{{MINIO_SECRET}}|${MINIO_SECRET}|g" \
    "${CONFIG_DIR}/config.yaml"
chown "${TPT_USER}:${TPT_USER}" "${CONFIG_DIR}/config.yaml"
chmod 0640 "${CONFIG_DIR}/config.yaml"

# -- Write MinIO env file (read by tpt-minio.service) -------------------------
cat > "${CONFIG_DIR}/minio.env" <<ENV
MINIO_ROOT_USER=tptminio
MINIO_ROOT_PASSWORD=${MINIO_SECRET}
ENV
chown "${TPT_USER}:${TPT_USER}" "${CONFIG_DIR}/minio.env"
chmod 0600 "${CONFIG_DIR}/minio.env"

echo "  setup-db complete"
