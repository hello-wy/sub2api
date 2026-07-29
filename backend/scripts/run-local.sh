#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
backend_dir="$(cd -- "${script_dir}/.." && pwd)"
project_dir="$(cd -- "${backend_dir}/.." && pwd)"
env_file="${project_dir}/deploy/.env"

if [[ ! -f "${env_file}" ]]; then
  echo "Missing local deployment configuration: ${env_file}" >&2
  exit 1
fi

env_value() {
  awk -F= -v key="$1" '$1 == key { sub(/^[^=]*=/, ""); print; exit }' "${env_file}"
}

require_env_value() {
  local value
  value="$(env_value "$1")"
  if [[ -z "${value}" ]]; then
    echo "Missing $1 in ${env_file}" >&2
    exit 1
  fi
  printf '%s' "${value}"
}

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker is required to run the local PostgreSQL and Redis dependencies." >&2
  exit 1
fi

if ! docker compose -f "${project_dir}/deploy/docker-compose.yml" ps --status running --services | grep -qx postgres; then
  echo "PostgreSQL is not running. Start dependencies with:" >&2
  echo "  cd ${project_dir}/deploy && docker compose up -d postgres redis" >&2
  exit 1
fi

if ! docker compose -f "${project_dir}/deploy/docker-compose.yml" ps --status running --services | grep -qx redis; then
  echo "Redis is not running. Start dependencies with:" >&2
  echo "  cd ${project_dir}/deploy && docker compose up -d postgres redis" >&2
  exit 1
fi

export DATABASE_HOST=127.0.0.1
export DATABASE_PORT=5433
export DATABASE_USER="$(require_env_value POSTGRES_USER)"
export DATABASE_PASSWORD="$(require_env_value POSTGRES_PASSWORD)"
export DATABASE_DBNAME="$(require_env_value POSTGRES_DB)"
export REDIS_HOST=127.0.0.1
export REDIS_PORT=6380
export REDIS_USERNAME="$(env_value REDIS_USERNAME)"
export REDIS_PASSWORD="$(env_value REDIS_PASSWORD)"
export REDIS_DB="$(env_value REDIS_DB)"
export JWT_SECRET="$(require_env_value JWT_SECRET)"
export TOTP_ENCRYPTION_KEY="$(require_env_value TOTP_ENCRYPTION_KEY)"

cd "${backend_dir}"
exec go run -tags embed ./cmd/server
