#!/usr/bin/env bash
# Downloads all third-party binary dependencies into deps/.
# Run once before build-installer.sh.
#
# Requires: curl, tar, xz (apt: xz-utils / dnf: xz)
#
# Versions — update these to pin newer releases:
PGSQL_VERSION="17"
REDIS_VERSION="7.4.2"
MINIO_DATE="latest"
FFMPEG_VERSION="7.1"
MEDIAMTX_VERSION="1.9.1"
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPS_DIR="${SCRIPT_DIR}/deps"
TMP_DIR=$(mktemp -d)
trap 'rm -rf "${TMP_DIR}"' EXIT

mkdir -p \
    "${DEPS_DIR}/pgsql/bin" \
    "${DEPS_DIR}/pgsql/lib" \
    "${DEPS_DIR}/redis" \
    "${DEPS_DIR}/minio" \
    "${DEPS_DIR}/ffmpeg" \
    "${DEPS_DIR}/mediamtx"

dl() {
    local url="$1" dest="$2"
    echo "  downloading: ${url}"
    curl -fsSL --progress-bar -o "${dest}" "${url}"
}

# ── PostgreSQL (via PGDG apt package extraction) ──────────────────────────────
echo "==> PostgreSQL ${PGSQL_VERSION}"
# Detect distro for PGDG URL
if command -v apt-get &>/dev/null; then
    PGDG_CODENAME=$(. /etc/os-release && echo "${VERSION_CODENAME:-bookworm}")
    dl "https://apt.postgresql.org/pub/repos/apt/pool/main/p/postgresql-${PGSQL_VERSION}/postgresql-${PGSQL_VERSION}_${PGSQL_VERSION}.0-1.pgdg${PGDG_CODENAME}+1_amd64.deb" \
        "${TMP_DIR}/postgresql.deb"
    (cd "${TMP_DIR}" && ar x postgresql.deb && tar xf data.tar.* -C .)
    cp "${TMP_DIR}/usr/lib/postgresql/${PGSQL_VERSION}/bin/"{initdb,pg_ctl,psql,pg_isready,postgres,pg_dump,pg_restore} \
        "${DEPS_DIR}/pgsql/bin/" 2>/dev/null || true
    # Copy shared libs
    [[ -d "${TMP_DIR}/usr/lib/postgresql/${PGSQL_VERSION}/lib" ]] && \
        cp -r "${TMP_DIR}/usr/lib/postgresql/${PGSQL_VERSION}/lib/." "${DEPS_DIR}/pgsql/lib/"
    # Also fetch libpq and common libs
    for pkg in libpq5 postgresql-client-common postgresql-common; do
        dl "https://apt.postgresql.org/pub/repos/apt/pool/main/p/${pkg}/${pkg}_${PGSQL_VERSION}*_amd64.deb" \
            "${TMP_DIR}/${pkg}.deb" 2>/dev/null || true
    done
elif command -v dnf &>/dev/null || command -v yum &>/dev/null; then
    echo "  NOTE: RHEL/Fedora — using embedded-postgres-binaries project"
    # embedded-postgres-binaries provides portable Linux builds
    # https://github.com/zonkyio/embedded-postgres-binaries/releases
    EP_VERSION="17.0.0"
    dl "https://github.com/zonkyio/embedded-postgres-binaries/releases/download/v${EP_VERSION}/postgres-linux-amd64.txz" \
        "${TMP_DIR}/pgsql.txz"
    tar -xJf "${TMP_DIR}/pgsql.txz" -C "${TMP_DIR}/pgsql_extract"
    cp "${TMP_DIR}/pgsql_extract/bin/"{initdb,pg_ctl,psql,pg_isready,postgres,pg_dump,pg_restore} \
        "${DEPS_DIR}/pgsql/bin/" 2>/dev/null || true
    cp -r "${TMP_DIR}/pgsql_extract/lib/." "${DEPS_DIR}/pgsql/lib/" 2>/dev/null || true
else
    echo "  WARNING: unknown distro — download PostgreSQL binaries manually to ${DEPS_DIR}/pgsql/"
fi
chmod 0755 "${DEPS_DIR}/pgsql/bin/"* 2>/dev/null || true

# ── Redis (static build) ──────────────────────────────────────────────────────
echo "==> Redis ${REDIS_VERSION}"
dl "https://github.com/redis/redis/archive/refs/tags/${REDIS_VERSION}.tar.gz" \
    "${TMP_DIR}/redis.tar.gz"
tar -xzf "${TMP_DIR}/redis.tar.gz" -C "${TMP_DIR}"
REDIS_SRC="${TMP_DIR}/redis-${REDIS_VERSION}"
(cd "${REDIS_SRC}" && make -j"$(nproc)" CFLAGS='-O2' 2>/dev/null)
cp "${REDIS_SRC}/src/redis-server" "${DEPS_DIR}/redis/redis-server"
cp "${REDIS_SRC}/src/redis-cli"    "${DEPS_DIR}/redis/redis-cli"
chmod 0755 "${DEPS_DIR}/redis/redis-server" "${DEPS_DIR}/redis/redis-cli"

# ── MinIO (static Go binary) ──────────────────────────────────────────────────
echo "==> MinIO"
dl "https://dl.min.io/server/minio/release/linux-amd64/minio" \
    "${DEPS_DIR}/minio/minio"
chmod 0755 "${DEPS_DIR}/minio/minio"

# ── FFmpeg (static build) ─────────────────────────────────────────────────────
echo "==> FFmpeg ${FFMPEG_VERSION}"
dl "https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-amd64-static.tar.xz" \
    "${TMP_DIR}/ffmpeg.tar.xz"
tar -xJf "${TMP_DIR}/ffmpeg.tar.xz" -C "${TMP_DIR}"
FFMPEG_DIR=$(find "${TMP_DIR}" -maxdepth 1 -name "ffmpeg-*-amd64-static" -type d | head -1)
cp "${FFMPEG_DIR}/ffmpeg"  "${DEPS_DIR}/ffmpeg/ffmpeg"
cp "${FFMPEG_DIR}/ffprobe" "${DEPS_DIR}/ffmpeg/ffprobe"
chmod 0755 "${DEPS_DIR}/ffmpeg/ffmpeg" "${DEPS_DIR}/ffmpeg/ffprobe"

# ── MediaMTX ─────────────────────────────────────────────────────────────────
echo "==> MediaMTX v${MEDIAMTX_VERSION}"
dl "https://github.com/bluenviron/mediamtx/releases/download/v${MEDIAMTX_VERSION}/mediamtx_v${MEDIAMTX_VERSION}_linux_amd64.tar.gz" \
    "${TMP_DIR}/mediamtx.tar.gz"
tar -xzf "${TMP_DIR}/mediamtx.tar.gz" -C "${TMP_DIR}/mediamtx_extract" --strip-components=0
cp "${TMP_DIR}/mediamtx_extract/mediamtx"     "${DEPS_DIR}/mediamtx/mediamtx"
cp "${TMP_DIR}/mediamtx_extract/mediamtx.yml" "${DEPS_DIR}/mediamtx/mediamtx.yml"
chmod 0755 "${DEPS_DIR}/mediamtx/mediamtx"

echo ""
echo "==> All dependencies downloaded to ${DEPS_DIR}"
echo "    Run ./build-installer.sh to assemble the release tarball."
