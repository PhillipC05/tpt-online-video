#!/usr/bin/env bash
set -euo pipefail

# TPT Online Video — Linux Installer
# Installs systemd services for API, Worker, and Live processes.

TPT_USER="${TPT_USER:-tpt}"
TPT_GROUP="${TPT_GROUP:-tpt}"
TPT_HOME="${TPT_HOME:-/opt/tpt}"
TPT_BIN="${TPT_BIN:-/usr/local/bin}"

echo "==> Creating system user: ${TPT_USER}"
id -u "${TPT_USER}" &>/dev/null || useradd --system --no-create-home --home-dir "${TPT_HOME}" --shell /usr/sbin/nologin "${TPT_USER}"

echo "==> Creating directories"
mkdir -p "${TPT_HOME}"/{bin,data,config}
mkdir -p "${TPT_HOME}/data"/{storage,postgres,redis}

echo "==> Installing binaries"
install -m 0755 -o "${TPT_USER}" -g "${TPT_GROUP}" ./bin/tpt-api   "${TPT_HOME}/bin/"
install -m 0755 -o "${TPT_USER}" -g "${TPT_GROUP}" ./bin/tpt-worker "${TPT_HOME}/bin/"
install -m 0755 -o "${TPT_USER}" -g "${TPT_GROUP}" ./bin/tpt-live  "${TPT_HOME}/bin/"

echo "==> Installing systemd service units"
for unit in tpt-api tpt-worker tpt-live; do
  install -m 0644 "./infra/installer/linux/${unit}.service" "/etc/systemd/system/${unit}.service"
done

echo "==> Reloading systemd"
systemctl daemon-reload

echo "==> Enabling services"
systemctl enable tpt-api tpt-worker tpt-live

echo "==> Starting services"
systemctl start tpt-api tpt-worker tpt-live

echo "==> Done. Check status with: systemctl status tpt-api tpt-worker tpt-live"