# Installer

## Purpose

Self-contained installer scripts for deploying TPT Online Video on Linux and Windows.

## Structure

- `linux/` — Systemd-based deployment for Ubuntu/Debian/RHEL
  - `install.sh` — installs binaries, creates system user, configures systemd services
  - `tpt-api.service` — systemd unit for the API process
  - `tpt-worker.service` — systemd unit for the transcoding worker
  - `tpt-live.service` — systemd unit for the live helper process
- `windows/` — Windows service-based deployment
  - `install.ps1` — PowerShell installer script
  - `tpt-online-video.xml` — winsw service configuration
- `common/` — shared assets and helper scripts

## Usage

```bash
# Linux
sudo bash infra/installer/linux/install.sh

# Windows (Admin PowerShell)
.\infra\installer\windows\install.ps1