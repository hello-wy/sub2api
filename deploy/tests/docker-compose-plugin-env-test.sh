#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$repo_root"

variable=PLUGINS_TRUSTED_PUBLISHERS_JSON
expected="      - ${variable}=\${${variable}:-}"

for compose_file in \
  deploy/docker-compose.yml \
  deploy/docker-compose.local.yml \
  deploy/docker-compose.standalone.yml \
  deploy/docker-compose.dev.yml
do
  count=$(grep -Fxc "$expected" "$compose_file" || true)
  if [ "$count" -ne 1 ]; then
    printf '%s must pass %s exactly once\n' "$compose_file" "$variable" >&2
    exit 1
  fi
done

example_count=$(grep -Fxc "${variable}={}" deploy/.env.example || true)
if [ "$example_count" -ne 1 ]; then
  printf 'deploy/.env.example must document %s exactly once\n' "$variable" >&2
  exit 1
fi

printf 'docker compose plugin publisher environment test passed\n'
