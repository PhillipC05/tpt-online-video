#!/usr/bin/env bash
# Assemble build artefacts and produce the self-contained Linux tarball.
#
# Steps:
#   1. Build Go binaries (linux/amd64)
#   2. Build frontend with Vite
#   3. Verify third-party deps in deps/
#   4. Assemble release tarball: dist/tpt-online-video-<version>-linux-amd64.tar.gz
#
# Usage:
#   ./build-installer.sh                    # version from git tag
#   ./build-installer.sh --version 1.2.0
#   ./build-installer.sh --skip-build       # use existing dist/bin and dist/web
#
# Third-party deps required in deps/ before running (see fetch-deps.sh):
#   deps/pgsql/bin/{initdb,pg_ctl,psql,pg_isready,postgres,pg_dump,pg_restore}
#   deps/pgsql/lib/*.so*
#   deps/redis/{redis-server,redis-cli}
#   deps/minio/minio
#   deps/ffmpeg/{ffmpeg,ffprobe}
#   deps/mediamtx/{mediamtx,mediamtx.yml}
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
DIST_DIR="${SCRIPT_DIR}/dist"
DEPS_DIR="${SCRIPT_DIR}/deps"
VERSION=""
SKIP_BUILD=false
MODULE="github.com/your-org/tpt-online-video"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --version)    VERSION="$2";  shift 2 ;;
        --skip-build) SKIP_BUILD=true; shift ;;
        *) echo "Unknown argument: $1" >&2; exit 1 ;;
    esac
done

if [[ -z "${VERSION}" ]]; then
    VERSION=$(git -C "${REPO_ROOT}" describe --tags --abbrev=0 2>/dev/null || echo "1.0.0")
    VERSION="${VERSION#v}"
fi

echo "==> Building TPT Online Video installer v${VERSION} (linux/amd64)"

# ── 1. Build Go binaries ──────────────────────────────────────────────────────
if ! $SKIP_BUILD; then
    echo "==> Building Go binaries (GOOS=linux GOARCH=amd64)"
    BIN_DIST="${DIST_DIR}/bin"
    mkdir -p "${BIN_DIST}"

    for svc in api worker live; do
        pkg="${MODULE}/services/${svc}/cmd/tpt-${svc}"
        out="${BIN_DIST}/tpt-${svc}"
        echo "    tpt-${svc}"
        GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
            go build -trimpath \
                -ldflags "-s -w -X main.Version=${VERSION}" \
                -o "${out}" "${pkg}"
    done

    # ── 2. Build frontend ─────────────────────────────────────────────────────
    echo "==> Building frontend"
    WEB_DIST="${DIST_DIR}/web"
    mkdir -p "${WEB_DIST}"
    (cd "${REPO_ROOT}/apps/web" && pnpm install --frozen-lockfile && pnpm build)
    cp -r "${REPO_ROOT}/apps/web/dist/." "${WEB_DIST}/"
fi

# ── 3. Verify third-party deps ────────────────────────────────────────────────
echo "==> Checking dependencies in ${DEPS_DIR}"
MISSING=false

assert_dep() {
    local path="$1" hint="$2"
    if [[ ! -f "${path}" ]]; then
        echo "  MISSING: ${path}"
        echo "           ${hint}"
        MISSING=true
    fi
}

assert_dep "${DEPS_DIR}/pgsql/bin/initdb" \
    "Run: ./fetch-deps.sh  (downloads PGDG amd64 binaries)"
assert_dep "${DEPS_DIR}/pgsql/bin/pg_ctl"       "Same fetch-deps.sh"
assert_dep "${DEPS_DIR}/pgsql/bin/psql"         "Same fetch-deps.sh"
assert_dep "${DEPS_DIR}/pgsql/bin/pg_isready"   "Same fetch-deps.sh"
assert_dep "${DEPS_DIR}/pgsql/bin/postgres"     "Same fetch-deps.sh"
assert_dep "${DEPS_DIR}/redis/redis-server" \
    "Download static build from https://github.com/redis/redis/releases or build: make CFLAGS='-static' redis-server redis-cli"
assert_dep "${DEPS_DIR}/redis/redis-cli"        "Same as redis-server"
assert_dep "${DEPS_DIR}/minio/minio" \
    "curl -L https://dl.min.io/server/minio/release/linux-amd64/minio -o deps/minio/minio && chmod +x deps/minio/minio"
assert_dep "${DEPS_DIR}/ffmpeg/ffmpeg" \
    "Download static build from https://johnvansickle.com/ffmpeg/ (ffmpeg-release-amd64-static.tar.xz)"
assert_dep "${DEPS_DIR}/ffmpeg/ffprobe"     "Same archive as ffmpeg"
assert_dep "${DEPS_DIR}/mediamtx/mediamtx" \
    "Download from https://github.com/bluenviron/mediamtx/releases (mediamtx_vX.Y.Z_linux_amd64.tar.gz)"
assert_dep "${DEPS_DIR}/mediamtx/mediamtx.yml"  "Same MediaMTX release archive"

if $MISSING; then
    echo ""
    echo "ERROR: Missing dependencies — run ./fetch-deps.sh to download them automatically."
    exit 1
fi

# ── 4. Assemble release tarball ───────────────────────────────────────────────
echo "==> Assembling release tarball"
RELEASE_NAME="tpt-online-video-${VERSION}-linux-amd64"
RELEASE_DIR="${DIST_DIR}/release/${RELEASE_NAME}"
mkdir -p \
    "${RELEASE_DIR}/bin" \
    "${RELEASE_DIR}/pgsql/bin" \
    "${RELEASE_DIR}/pgsql/lib" \
    "${RELEASE_DIR}/web" \
    "${RELEASE_DIR}/config" \
    "${RELEASE_DIR}/systemd"

# Application binaries
for bin in tpt-api tpt-worker tpt-live; do
    install -m 0755 "${DIST_DIR}/bin/${bin}" "${RELEASE_DIR}/bin/${bin}"
done

# Infrastructure binaries
install -m 0755 "${DEPS_DIR}/redis/redis-server"       "${RELEASE_DIR}/bin/redis-server"
install -m 0755 "${DEPS_DIR}/redis/redis-cli"          "${RELEASE_DIR}/bin/redis-cli"
install -m 0755 "${DEPS_DIR}/minio/minio"              "${RELEASE_DIR}/bin/minio"
install -m 0755 "${DEPS_DIR}/ffmpeg/ffmpeg"            "${RELEASE_DIR}/bin/ffmpeg"
install -m 0755 "${DEPS_DIR}/ffmpeg/ffprobe"           "${RELEASE_DIR}/bin/ffprobe"
install -m 0755 "${DEPS_DIR}/mediamtx/mediamtx"        "${RELEASE_DIR}/bin/mediamtx"

# PostgreSQL binaries + libs
for bin in initdb pg_ctl psql pg_isready postgres pg_dump pg_restore; do
    [[ -f "${DEPS_DIR}/pgsql/bin/${bin}" ]] && \
        install -m 0755 "${DEPS_DIR}/pgsql/bin/${bin}" "${RELEASE_DIR}/pgsql/bin/${bin}"
done
[[ -d "${DEPS_DIR}/pgsql/lib" ]] && cp -r "${DEPS_DIR}/pgsql/lib/." "${RELEASE_DIR}/pgsql/lib/"

# Frontend
cp -r "${DIST_DIR}/web/." "${RELEASE_DIR}/web/"

# Configuration templates
install -m 0644 "${SCRIPT_DIR}/config.linux.yaml"  "${RELEASE_DIR}/config/config.linux.yaml"
install -m 0644 "${SCRIPT_DIR}/redis.conf"         "${RELEASE_DIR}/config/redis.conf"
install -m 0644 "${SCRIPT_DIR}/mediamtx.yml"       "${RELEASE_DIR}/config/mediamtx.yml"
install -m 0644 "${SCRIPT_DIR}/nginx.conf"         "${RELEASE_DIR}/config/nginx.conf"

# Installer scripts
install -m 0755 "${SCRIPT_DIR}/install.sh"     "${RELEASE_DIR}/install.sh"
install -m 0755 "${SCRIPT_DIR}/setup-db.sh"    "${RELEASE_DIR}/setup-db.sh"
install -m 0755 "${SCRIPT_DIR}/healthcheck.sh" "${RELEASE_DIR}/healthcheck.sh"
install -m 0755 "${SCRIPT_DIR}/uninstall.sh"   "${RELEASE_DIR}/uninstall.sh"
install -m 0755 "${SCRIPT_DIR}/upgrade.sh"     "${RELEASE_DIR}/upgrade.sh"

# systemd units
for unit in tpt-postgresql tpt-redis tpt-minio tpt-mediamtx tpt-api tpt-worker tpt-live; do
    install -m 0644 "${SCRIPT_DIR}/${unit}.service" "${RELEASE_DIR}/systemd/${unit}.service"
done

# Pack tarball
mkdir -p "${DIST_DIR}/installer"
TARBALL="${DIST_DIR}/installer/${RELEASE_NAME}.tar.gz"
tar -czf "${TARBALL}" -C "${DIST_DIR}/release" "${RELEASE_NAME}"

echo "==> Done: ${TARBALL}"
echo "    Install: tar xzf ${RELEASE_NAME}.tar.gz && cd ${RELEASE_NAME} && sudo ./install.sh"
