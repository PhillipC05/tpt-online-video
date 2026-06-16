#!/usr/bin/env bash
# Post-install health check — verifies all services are running and the API responds.
#
# Usage: healthcheck.sh [tpt-home] [retries]
#   tpt-home  default /opt/tpt
#   retries   number of 5-second attempts for API probe (default 12 ≈ 60s)
set -uo pipefail

TPT_HOME="${1:-/opt/tpt}"
RETRIES="${2:-12}"
LOGS_DIR="${TPT_HOME}/logs"
REPORT="${LOGS_DIR}/install-healthcheck.txt"
FAILED=()

mkdir -p "${LOGS_DIR}"

# -- Service status -----------------------------------------------------------
SERVICES=(
    "tpt-postgresql:PostgreSQL"
    "tpt-redis:Redis"
    "tpt-minio:MinIO"
    "tpt-mediamtx:MediaMTX"
    "tpt-api:TPT API"
    "tpt-worker:TPT Worker"
    "tpt-live:TPT Live"
    "nginx:Nginx"
)

check_service() {
    local unit="${1%%:*}"
    local label="${1#*:}"
    if systemctl is-active --quiet "${unit}"; then
        printf "  [OK]   %s\n" "${label}"
    else
        printf "  [FAIL] %s: %s\n" "${label}" "$(systemctl is-active "${unit}" 2>&1)"
        FAILED+=("${label} service is not active")
    fi
}

for svc in "${SERVICES[@]}"; do
    check_service "${svc}"
done

# -- API liveness probe -------------------------------------------------------
API_UP=false
for i in $(seq 1 "${RETRIES}"); do
    if curl -fsS --max-time 5 "http://localhost:8080/healthz" >/dev/null 2>&1; then
        API_UP=true
        break
    fi
    sleep 5
done

if $API_UP; then
    echo "  [OK]   API /healthz: 200 OK"
else
    echo "  [FAIL] API /healthz: no response after $((RETRIES * 5))s"
    FAILED+=("API /healthz did not respond within $((RETRIES * 5)) seconds")
fi

# -- API readiness probe ------------------------------------------------------
if $API_UP; then
    if curl -fsS --max-time 10 "http://localhost:8080/readyz" >/dev/null 2>&1; then
        echo "  [OK]   API /readyz: 200 OK"
    else
        echo "  [FAIL] API /readyz: unexpected response"
        FAILED+=("API /readyz did not return 200")
    fi
fi

# -- Write report -------------------------------------------------------------
{
    echo "TPT Online Video — install health check $(date '+%Y-%m-%d %H:%M:%S')"
    echo ""
    for svc in "${SERVICES[@]}"; do
        unit="${svc%%:*}"
        label="${svc#*:}"
        status=$(systemctl is-active "${unit}" 2>/dev/null || echo "not-found")
        ok=$([ "${status}" = "active" ] && echo "OK" || echo "FAIL")
        printf "  [%s] %s: %s\n" "${ok}" "${label}" "${status}"
    done
    echo ""
    printf "  [%s] API /healthz: %s\n" "$(${API_UP} && echo OK || echo FAIL)" \
           "$($API_UP && echo '200 OK' || echo 'unreachable')"
    echo ""
    if [[ ${#FAILED[@]} -gt 0 ]]; then
        echo "FAILED:"
        for msg in "${FAILED[@]}"; do
            echo "  - ${msg}"
        done
    else
        echo "All checks passed."
    fi
} | tee "${REPORT}"

if [[ ${#FAILED[@]} -gt 0 ]]; then
    echo ""
    echo "Health check failed. See ${REPORT}"
    exit 1
fi
