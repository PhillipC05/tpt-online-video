#!/usr/bin/env bash
# TPT Online Video — Linux Self-Contained Installer
#
# Installs all services under /opt/tpt and registers them with systemd.
# Must be run as root from the extracted release tarball directory.
#
# Usage:
#   sudo ./install.sh                              # interactive prompts
#   sudo ./install.sh \
#       --admin-email admin@example.com \
#       --admin-password "Str0ngP@ss!" \
#       --db-password "Str0ngDB@ss!"              # non-interactive
set -euo pipefail

# ── Parse arguments ───────────────────────────────────────────────────────────
ADMIN_EMAIL=""
ADMIN_PASSWORD=""
DB_PASSWORD=""
TPT_HOME="/opt/tpt"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --admin-email)    ADMIN_EMAIL="$2";    shift 2 ;;
        --admin-password) ADMIN_PASSWORD="$2"; shift 2 ;;
        --db-password)    DB_PASSWORD="$2";    shift 2 ;;
        --tpt-home)       TPT_HOME="$2";       shift 2 ;;
        *) echo "Unknown argument: $1" >&2; exit 1 ;;
    esac
done

# ── Checks ────────────────────────────────────────────────────────────────────
if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
    echo "ERROR: install.sh must be run as root (use sudo)" >&2
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ── Interactive prompts ───────────────────────────────────────────────────────
if [[ -z "${ADMIN_EMAIL}" ]]; then
    read -rp "Admin email:    " ADMIN_EMAIL
fi
if [[ -z "${ADMIN_PASSWORD}" ]]; then
    read -rsp "Admin password: " ADMIN_PASSWORD; echo
fi
if [[ -z "${DB_PASSWORD}" ]]; then
    read -rsp "DB password:    " DB_PASSWORD; echo
fi

[[ -z "${ADMIN_EMAIL}" ]]    && { echo "ERROR: admin email is required"    >&2; exit 1; }
[[ -z "${ADMIN_PASSWORD}" ]] && { echo "ERROR: admin password is required" >&2; exit 1; }
[[ -z "${DB_PASSWORD}" ]]    && { echo "ERROR: db password is required"    >&2; exit 1; }

# Minimum password length check
[[ ${#ADMIN_PASSWORD} -lt 12 ]] && { echo "ERROR: admin password must be at least 12 characters" >&2; exit 1; }
[[ ${#DB_PASSWORD} -lt 12 ]]    && { echo "ERROR: db password must be at least 12 characters"    >&2; exit 1; }

TPT_USER="tpt"
TPT_GROUP="tpt"
SYSTEMD_DIR="/etc/systemd/system"

echo ""
echo "==> Installing TPT Online Video to ${TPT_HOME}"

# ── Step 1: System user ───────────────────────────────────────────────────────
echo "==> Creating system user: ${TPT_USER}"
if ! id -u "${TPT_USER}" &>/dev/null; then
    useradd --system --no-create-home \
            --home-dir "${TPT_HOME}" \
            --shell /usr/sbin/nologin \
            "${TPT_USER}"
fi

# ── Step 2: Directory structure ───────────────────────────────────────────────
echo "==> Creating directories"
mkdir -p \
    "${TPT_HOME}/bin" \
    "${TPT_HOME}/pgsql/bin" \
    "${TPT_HOME}/pgsql/lib" \
    "${TPT_HOME}/web" \
    "${TPT_HOME}/config" \
    "${TPT_HOME}/scripts" \
    "${TPT_HOME}/data/storage" \
    "${TPT_HOME}/data/pgsql" \
    "${TPT_HOME}/data/redis" \
    "${TPT_HOME}/data/minio" \
    "${TPT_HOME}/data/hls" \
    "${TPT_HOME}/data/tmp" \
    "${TPT_HOME}/data/nginx-cache" \
    "${TPT_HOME}/logs/postgresql" \
    "${TPT_HOME}/logs/redis" \
    "${TPT_HOME}/logs/minio" \
    "${TPT_HOME}/logs/mediamtx"

# ── Step 3: Install application binaries ─────────────────────────────────────
echo "==> Installing application binaries"
for bin in tpt-api tpt-worker tpt-live; do
    install -m 0755 -o "${TPT_USER}" -g "${TPT_GROUP}" \
        "${SCRIPT_DIR}/bin/${bin}" "${TPT_HOME}/bin/${bin}"
done

# ── Step 4: Install infrastructure binaries ───────────────────────────────────
echo "==> Installing infrastructure binaries"
for bin in ffmpeg ffprobe redis-server redis-cli minio mediamtx; do
    install -m 0755 -o "${TPT_USER}" -g "${TPT_GROUP}" \
        "${SCRIPT_DIR}/bin/${bin}" "${TPT_HOME}/bin/${bin}"
done

# PostgreSQL binaries + shared libs
for bin in initdb pg_ctl psql pg_isready postgres pg_dump pg_restore; do
    if [[ -f "${SCRIPT_DIR}/pgsql/bin/${bin}" ]]; then
        install -m 0755 "${SCRIPT_DIR}/pgsql/bin/${bin}" "${TPT_HOME}/pgsql/bin/${bin}"
    fi
done
if [[ -d "${SCRIPT_DIR}/pgsql/lib" ]]; then
    cp -r "${SCRIPT_DIR}/pgsql/lib/." "${TPT_HOME}/pgsql/lib/"
fi

# ── Step 5: Install frontend static assets ───────────────────────────────────
echo "==> Installing frontend assets"
cp -r "${SCRIPT_DIR}/web/." "${TPT_HOME}/web/"

# ── Step 6: Install configuration files ──────────────────────────────────────
echo "==> Installing configuration templates"
install -m 0644 -o "${TPT_USER}" -g "${TPT_GROUP}" \
    "${SCRIPT_DIR}/config/config.linux.yaml" \
    "${TPT_HOME}/config/config.yaml.template"

# Redis config (don't overwrite an existing customised config)
if [[ ! -f "${TPT_HOME}/config/redis.conf" ]]; then
    install -m 0644 -o "${TPT_USER}" -g "${TPT_GROUP}" \
        "${SCRIPT_DIR}/config/redis.conf" "${TPT_HOME}/config/redis.conf"
fi

# MediaMTX config
if [[ ! -f "${TPT_HOME}/config/mediamtx.yml" ]]; then
    install -m 0644 -o "${TPT_USER}" -g "${TPT_GROUP}" \
        "${SCRIPT_DIR}/config/mediamtx.yml" "${TPT_HOME}/config/mediamtx.yml"
fi

# ── Step 7: Install scripts ───────────────────────────────────────────────────
echo "==> Installing scripts"
install -m 0755 "${SCRIPT_DIR}/setup-db.sh"    "${TPT_HOME}/scripts/setup-db.sh"
install -m 0755 "${SCRIPT_DIR}/healthcheck.sh" "${TPT_HOME}/scripts/healthcheck.sh"
install -m 0755 "${SCRIPT_DIR}/upgrade.sh"     "${TPT_HOME}/scripts/upgrade.sh"
install -m 0755 "${SCRIPT_DIR}/uninstall.sh"   "${TPT_HOME}/scripts/uninstall.sh"

# ── Step 8: Fix ownership ─────────────────────────────────────────────────────
chown -R "${TPT_USER}:${TPT_GROUP}" "${TPT_HOME}"
# root owns the install root itself
chown root:root "${TPT_HOME}"

# ── Step 9: Install systemd units ────────────────────────────────────────────
echo "==> Installing systemd units"
UNITS=(tpt-postgresql tpt-redis tpt-minio tpt-mediamtx tpt-api tpt-worker tpt-live)
for unit in "${UNITS[@]}"; do
    install -m 0644 \
        "${SCRIPT_DIR}/systemd/${unit}.service" \
        "${SYSTEMD_DIR}/${unit}.service"
    echo "  installed: ${unit}.service"
done

# ── Step 10: Install nginx site config ───────────────────────────────────────
echo "==> Configuring nginx"
if ! command -v nginx &>/dev/null; then
    echo "  nginx not found — attempting package install"
    if command -v apt-get &>/dev/null; then
        apt-get install -y --no-install-recommends nginx
    elif command -v dnf &>/dev/null; then
        dnf install -y nginx
    elif command -v yum &>/dev/null; then
        yum install -y nginx
    else
        echo "  WARNING: could not install nginx automatically — install it manually and place"
        echo "           ${SCRIPT_DIR}/config/nginx.conf in /etc/nginx/conf.d/tpt.conf"
    fi
fi

if command -v nginx &>/dev/null; then
    install -m 0644 "${SCRIPT_DIR}/config/nginx.conf" /etc/nginx/conf.d/tpt.conf
    # Disable the default site if present
    rm -f /etc/nginx/sites-enabled/default 2>/dev/null || true
    nginx -t && echo "  nginx config OK"
fi

# ── Step 11: Initialise database, generate secrets, write config.yaml ────────
echo "==> Initialising database"
bash "${TPT_HOME}/scripts/setup-db.sh" \
    "${TPT_HOME}" \
    "${ADMIN_EMAIL}" \
    "${ADMIN_PASSWORD}" \
    "${DB_PASSWORD}"

# ── Step 12: Reload systemd, enable and start services ───────────────────────
echo "==> Starting services"
systemctl daemon-reload

INFRA_SERVICES=(tpt-postgresql tpt-redis tpt-minio tpt-mediamtx)
APP_SERVICES=(tpt-api tpt-worker tpt-live)

for svc in "${INFRA_SERVICES[@]}" "${APP_SERVICES[@]}"; do
    systemctl enable "${svc}"
done

for svc in "${INFRA_SERVICES[@]}"; do
    systemctl start "${svc}"
done

# Run database migrations before starting application services
echo "==> Running database migrations"
sudo -u "${TPT_USER}" "${TPT_HOME}/bin/tpt-api" migrate \
    --config "${TPT_HOME}/config/config.yaml"

for svc in "${APP_SERVICES[@]}"; do
    systemctl start "${svc}"
done

# Start nginx
if command -v nginx &>/dev/null; then
    systemctl enable nginx 2>/dev/null || true
    systemctl restart nginx
fi

# ── Step 13: Firewall rules ───────────────────────────────────────────────────
echo "==> Configuring firewall"
if command -v ufw &>/dev/null && ufw status | grep -q "Status: active"; then
    ufw allow 80/tcp   comment "TPT HTTP"
    ufw allow 8080/tcp comment "TPT API"
    ufw allow 1935/tcp comment "TPT RTMP"
    ufw allow 8888/tcp comment "TPT HLS"
    ufw allow 8889/tcp comment "TPT WebRTC"
    echo "  ufw rules added"
elif command -v firewall-cmd &>/dev/null && systemctl is-active --quiet firewalld; then
    for port in 80/tcp 8080/tcp 1935/tcp 8888/tcp 8889/tcp; do
        firewall-cmd --permanent --add-port="${port}"
    done
    firewall-cmd --reload
    echo "  firewalld rules added"
else
    echo "  No active firewall detected — ensure ports 80, 8080, 1935, 8888, 8889 are open"
fi

# ── Step 14: Health check ─────────────────────────────────────────────────────
echo "==> Running health check"
bash "${TPT_HOME}/scripts/healthcheck.sh" "${TPT_HOME}"
if [[ $? -ne 0 ]]; then
    echo ""
    echo "WARNING: Installation completed but health check failed."
    echo "         Check logs in ${TPT_HOME}/logs/ and review:"
    echo "           journalctl -u tpt-api -n 50"
    exit 1
fi

echo ""
echo "============================================================"
echo "  TPT Online Video installed successfully!"
echo ""
echo "  Web interface:  http://$(hostname -I | awk '{print $1}'):80"
echo "  API:            http://$(hostname -I | awk '{print $1}'):8080"
echo "  Admin login:    ${ADMIN_EMAIL}"
echo ""
echo "  Manage services:  systemctl {start|stop|status} tpt-api"
echo "  Upgrade:          sudo ${TPT_HOME}/scripts/upgrade.sh --new-bin <dir>"
echo "  Uninstall:        sudo ${TPT_HOME}/scripts/uninstall.sh"
echo "  Health check:     sudo ${TPT_HOME}/scripts/healthcheck.sh"
echo "============================================================"
