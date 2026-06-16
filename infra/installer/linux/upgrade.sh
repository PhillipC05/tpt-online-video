#!/usr/bin/env bash
# In-place upgrade: stop app services, replace binaries and frontend,
# run database migrations, then restart.
#
# Usage: upgrade.sh --new-bin <dir> [--new-web <dir>] [--tpt-home <dir>]
#   --new-bin   directory containing new tpt-api, tpt-worker, tpt-live binaries
#   --new-web   directory containing new frontend static assets (optional)
#   --tpt-home  install root (default /opt/tpt)
set -euo pipefail

TPT_HOME="/opt/tpt"
NEW_BIN=""
NEW_WEB=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --new-bin)   NEW_BIN="$2";  shift 2 ;;
        --new-web)   NEW_WEB="$2";  shift 2 ;;
        --tpt-home)  TPT_HOME="$2"; shift 2 ;;
        *) echo "Unknown argument: $1" >&2; exit 1 ;;
    esac
done

[[ -z "${NEW_BIN}" ]] && { echo "ERROR: --new-bin is required" >&2; exit 1; }

if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
    echo "ERROR: upgrade.sh must be run as root" >&2
    exit 1
fi

# -- Validate new binaries ----------------------------------------------------
for bin in tpt-api tpt-worker tpt-live; do
    [[ -f "${NEW_BIN}/${bin}" ]] || { echo "ERROR: missing binary: ${NEW_BIN}/${bin}" >&2; exit 1; }
done

# -- Stop application services (infra keeps running) --------------------------
echo "==> Stopping application services"
systemctl stop tpt-live tpt-worker tpt-api

# -- Replace binaries ---------------------------------------------------------
echo "==> Installing new binaries"
for bin in tpt-api tpt-worker tpt-live; do
    install -m 0755 -o tpt -g tpt "${NEW_BIN}/${bin}" "${TPT_HOME}/bin/${bin}"
    echo "  updated: ${bin}"
done

# -- Replace frontend assets --------------------------------------------------
if [[ -n "${NEW_WEB}" && -d "${NEW_WEB}" ]]; then
    echo "==> Installing new frontend assets"
    rm -rf "${TPT_HOME}/web"
    cp -r "${NEW_WEB}" "${TPT_HOME}/web"
    chown -R tpt:tpt "${TPT_HOME}/web"
fi

# -- Run database migrations --------------------------------------------------
echo "==> Running database migrations"
sudo -u tpt "${TPT_HOME}/bin/tpt-api" migrate --config "${TPT_HOME}/config/config.yaml"

# -- Restart application services ---------------------------------------------
echo "==> Starting application services"
systemctl start tpt-api
systemctl start tpt-worker
systemctl start tpt-live

# -- Quick health check -------------------------------------------------------
echo "==> Running health check"
bash "${TPT_HOME}/scripts/healthcheck.sh" "${TPT_HOME}" 6
if [[ $? -ne 0 ]]; then
    echo "WARNING: health check failed after upgrade — check logs in ${TPT_HOME}/logs/"
    exit 1
fi

echo "==> Upgrade complete."
