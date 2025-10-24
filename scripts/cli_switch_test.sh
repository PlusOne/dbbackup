#!/usr/bin/env bash
set -u
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
BINARY_NAME="dbbackup_linux_amd64"
BINARY="./${BINARY_NAME}"
LOG_DIR="${REPO_ROOT}/test_logs"
TIMESTAMP="$(date +%Y%m%d_%H%M%S)"
LOG_FILE="${LOG_DIR}/cli_switch_test_${TIMESTAMP}.log"

PG_BACKUP_DIR="/tmp/db_backups"
PG_DATABASE="postgres"
PG_FLAGS=(
  --db-type postgres
  --host localhost
  --port 5432
  --user postgres
  --database "${PG_DATABASE}"
  --backup-dir "${PG_BACKUP_DIR}"
  --jobs 4
  --dump-jobs 4
  --max-cores 8
  --cpu-workload balanced
  --debug
)

MYSQL_BACKUP_DIR="/tmp/mysql_backups"
MYSQL_DATABASE="backup_demo"
MYSQL_FLAGS=(
  --db-type mysql
  --host 127.0.0.1
  --port 3306
  --user backup_user
  --password backup_pass
  --database "${MYSQL_DATABASE}"
  --backup-dir "${MYSQL_BACKUP_DIR}"
  --insecure
  --jobs 2
  --dump-jobs 2
  --max-cores 4
  --cpu-workload io-intensive
  --debug
)

mkdir -p "${LOG_DIR}"

log() {
  printf '%s\n' "$1" | tee -a "${LOG_FILE}" >/dev/null
}

RESULTS=()

run_cmd() {
  local label="$1"
  shift
  log ""
  log "### ${label}"
  log "Command: $*"
  "$@" 2>&1 | tee -a "${LOG_FILE}"
  local status=${PIPESTATUS[0]}
  log "Exit: ${status}"
  RESULTS+=("${label}|${status}")
}

latest_file() {
  local dir="$1"
  local pattern="$2"
  shopt -s nullglob
  local files=("${dir}"/${pattern})
  shopt -u nullglob
  if (( ${#files[@]} == 0 )); then
    return 1
  fi
  local latest="${files[0]}"
  for file in "${files[@]}"; do
    if [[ "${file}" -nt "${latest}" ]]; then
      latest="${file}"
    fi
  done
  printf '%s\n' "${latest}"
}

log "dbbackup CLI regression started"
log "Log file: ${LOG_FILE}"

cd "${REPO_ROOT}"

run_cmd "Go build" go build -o "${BINARY}" .
run_cmd "Ensure Postgres backup dir" sudo -u postgres mkdir -p "${PG_BACKUP_DIR}"
run_cmd "Ensure MySQL backup dir" mkdir -p "${MYSQL_BACKUP_DIR}"

run_cmd "Postgres status" sudo -u postgres "${BINARY}" status "${PG_FLAGS[@]}"
run_cmd "Postgres preflight" sudo -u postgres "${BINARY}" preflight "${PG_FLAGS[@]}"
run_cmd "Postgres CPU info" sudo -u postgres "${BINARY}" cpu "${PG_FLAGS[@]}"
run_cmd "Postgres backup single" sudo -u postgres "${BINARY}" backup single "${PG_DATABASE}" "${PG_FLAGS[@]}"
run_cmd "Postgres backup sample" sudo -u postgres "${BINARY}" backup sample "${PG_DATABASE}" --sample-ratio 5 "${PG_FLAGS[@]}"
run_cmd "Postgres backup cluster" sudo -u postgres "${BINARY}" backup cluster "${PG_FLAGS[@]}"
run_cmd "Postgres list" sudo -u postgres "${BINARY}" list "${PG_FLAGS[@]}"

PG_SINGLE_FILE="$(latest_file "${PG_BACKUP_DIR}" "db_${PG_DATABASE}_*.dump" || true)"
PG_SAMPLE_FILE="$(latest_file "${PG_BACKUP_DIR}" "sample_${PG_DATABASE}_*.sql" || true)"
PG_CLUSTER_FILE="$(latest_file "${PG_BACKUP_DIR}" "cluster_*.tar.gz" || true)"

if [[ -n "${PG_SINGLE_FILE}" ]]; then
  run_cmd "Postgres verify single" sudo -u postgres "${BINARY}" verify "$(basename "${PG_SINGLE_FILE}")" "${PG_FLAGS[@]}"
  run_cmd "Postgres restore single" sudo -u postgres "${BINARY}" restore "$(basename "${PG_SINGLE_FILE}")" "${PG_FLAGS[@]}"
else
  log "No PostgreSQL single backup found for verification"
  RESULTS+=("Postgres single artifact missing|1")
fi

if [[ -n "${PG_SAMPLE_FILE}" ]]; then
  run_cmd "Postgres verify sample" sudo -u postgres "${BINARY}" verify "$(basename "${PG_SAMPLE_FILE}")" "${PG_FLAGS[@]}"
  run_cmd "Postgres restore sample" sudo -u postgres "${BINARY}" restore "$(basename "${PG_SAMPLE_FILE}")" "${PG_FLAGS[@]}"
else
  log "No PostgreSQL sample backup found for verification"
  RESULTS+=("Postgres sample artifact missing|1")
fi

if [[ -n "${PG_CLUSTER_FILE}" ]]; then
  run_cmd "Postgres verify cluster" sudo -u postgres "${BINARY}" verify "$(basename "${PG_CLUSTER_FILE}")" "${PG_FLAGS[@]}"
  run_cmd "Postgres restore cluster" sudo -u postgres "${BINARY}" restore "$(basename "${PG_CLUSTER_FILE}")" "${PG_FLAGS[@]}"
else
  log "No PostgreSQL cluster backup found for verification"
  RESULTS+=("Postgres cluster artifact missing|1")
fi

run_cmd "MySQL status" "${BINARY}" status "${MYSQL_FLAGS[@]}"
run_cmd "MySQL preflight" "${BINARY}" preflight "${MYSQL_FLAGS[@]}"
run_cmd "MySQL CPU info" "${BINARY}" cpu "${MYSQL_FLAGS[@]}"
run_cmd "MySQL backup single" "${BINARY}" backup single "${MYSQL_DATABASE}" "${MYSQL_FLAGS[@]}"
run_cmd "MySQL backup sample" "${BINARY}" backup sample "${MYSQL_DATABASE}" --sample-percent 25 "${MYSQL_FLAGS[@]}"
run_cmd "MySQL list" "${BINARY}" list "${MYSQL_FLAGS[@]}"

MYSQL_SINGLE_FILE="$(latest_file "${MYSQL_BACKUP_DIR}" "db_${MYSQL_DATABASE}_*.sql.gz" || true)"
MYSQL_SAMPLE_FILE="$(latest_file "${MYSQL_BACKUP_DIR}" "sample_${MYSQL_DATABASE}_*.sql" || true)"

if [[ -n "${MYSQL_SINGLE_FILE}" ]]; then
  run_cmd "MySQL verify single" "${BINARY}" verify "$(basename "${MYSQL_SINGLE_FILE}")" "${MYSQL_FLAGS[@]}"
  run_cmd "MySQL restore single" "${BINARY}" restore "$(basename "${MYSQL_SINGLE_FILE}")" "${MYSQL_FLAGS[@]}"
else
  log "No MySQL single backup found for verification"
  RESULTS+=("MySQL single artifact missing|1")
fi

if [[ -n "${MYSQL_SAMPLE_FILE}" ]]; then
  run_cmd "MySQL verify sample" "${BINARY}" verify "$(basename "${MYSQL_SAMPLE_FILE}")" "${MYSQL_FLAGS[@]}"
  run_cmd "MySQL restore sample" "${BINARY}" restore "$(basename "${MYSQL_SAMPLE_FILE}")" "${MYSQL_FLAGS[@]}"
else
  log "No MySQL sample backup found for verification"
  RESULTS+=("MySQL sample artifact missing|1")
fi

run_cmd "Interactive help" "${BINARY}" interactive --help
run_cmd "Root help" "${BINARY}" --help
run_cmd "Root version" "${BINARY}" --version

log ""
log "=== Summary ==="
failed=0
for entry in "${RESULTS[@]}"; do
  IFS='|' read -r label status <<<"${entry}"
  if [[ "${status}" -eq 0 ]]; then
    log "[PASS] ${label}"
  else
    log "[FAIL] ${label} (exit ${status})"
    failed=1
  fi
done

exit "${failed}"
