#!/usr/bin/env bash
# Stops and deregisters all TPT services, removes systemd units and nginx
# site config, then removes /opt/tpt.
#
# Data is NOT deleted by default — pass --purge-data to also remove
# /opt/tpt/data (irreversible).
#
# Usage: uninstall.sh [--purge-data]
set -uo pipefail

PURGE_DATA=false
for arg in "$@"; do
    [[ "${arg}" == "--purge-data" ]] && PURGE_DATA=true
done

# Must run as root
if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
    echo "ERROR: uninstall.sh must be run as root" >&2
    exit 1
fi

TPT_HOME="/opt/tpt"
SYSTEMD_DIR="/etc/systemd/system"

SERVICES=(tpt-live tpt-worker tpt-api tpt-mediamtx tpt-minio tpt-redis tpt-postgresql)

# -- Stop and disable services ------------------------------------------------
echo "==> Stopping TPT services"
for svc in "${SERVICES[@]}"; do
    if systemctl is-active --quiet "${svc}" 2>/dev/null; then
        systemctl stop "${svc}" && echo "  stopped: ${svc}" || echo "  warn: could not stop ${svc}"
    fi
    if systemctl is-enabled --quiet "${svc}" 2>/dev/null; then
        systemctl disable "${svc}" && echo "  disabled: ${svc}" || true
    fi
done

# -- Remove systemd units -----------------------------------------------------
echo "==> Removing systemd units"
for svc in "${SERVICES[@]}"; do
    unit="${SYSTEMD_DIR}/${svc}.service"
    [[ -f "${unit}" ]] && rm -f "${unit}" && echo "  removed: ${unit}"
done
systemctl daemon-reload

# -- Remove nginx site config -------------------------------------------------
echo "==> Removing nginx config"
for path in /etc/nginx/conf.d/tpt.conf /etc/nginx/sites-enabled/tpt /etc/nginx/sites-available/tpt; do
    if [[ -f "${path}" || -L "${path}" ]]; then
        rm -f "${path}"
        echo "  removed: ${path}"
    fi
done
if command -v nginx &>/dev/null && systemctl is-active --quiet nginx 2>/dev/null; then
    nginx -t 2>/dev/null && systemctl reload nginx || true
fi

# -- Firewall rules -----------------------------------------------------------
echo "==> Removing firewall rules"
if command -v ufw &>/dev/null; then
    ufw delete allow 8080/tcp  2>/dev/null || true
    ufw delete allow 1935/tcp  2>/dev/null || true
    ufw delete allow 8888/tcp  2>/dev/null || true
    ufw delete allow 8889/tcp  2>/dev/null || true
elif command -v firewall-cmd &>/dev/null; then
    for port in 8080/tcp 1935/tcp 8888/tcp 8889/tcp; do
        firewall-cmd --permanent --remove-port="${port}" 2>/dev/null || true
    done
    firewall-cmd --reload 2>/dev/null || true
fi

# -- Remove application files -------------------------------------------------
if $PURGE_DATA; then
    echo "==> Removing ${TPT_HOME} (including data)"
    rm -rf "${TPT_HOME}"
else
    echo "==> Removing ${TPT_HOME} (preserving data)"
    # Remove everything except data/
    find "${TPT_HOME}" -maxdepth 1 \( -name bin -o -name pgsql -o -name web -o \
         -name config -o -name logs \) -exec rm -rf {} + 2>/dev/null || true
    echo "  Data preserved at ${TPT_HOME}/data — remove manually when no longer needed"
fi

# -- Remove tpt system user ---------------------------------------------------
if id tpt &>/dev/null; then
    userdel tpt 2>/dev/null || true
    echo "  removed system user: tpt"
fi

echo "==> Uninstall complete."
