#!/usr/bin/env bash
set -euo pipefail

APP_URL="${APP_URL:-http://127.0.0.1:${HOST_PORT:-8080}}"
COMPOSE_PROJECT="${COMPOSE_PROJECT:-}"
HEALTH_RETRIES="${HEALTH_RETRIES:-20}"
HEALTH_INTERVAL="${HEALTH_INTERVAL:-2}"

cd "$(dirname "$0")"

log() {
  printf '[deploy] %s\n' "$*"
}

fail() {
  printf '[deploy] ERROR: %s\n' "$*" >&2
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing command: $1"
}

compose() {
  if [ -n "$COMPOSE_PROJECT" ]; then
    docker compose -p "$COMPOSE_PROJECT" "$@"
  else
    docker compose "$@"
  fi
}

need_cmd git
need_cmd docker
need_cmd curl

[ -f ".env" ] || fail ".env not found. Create it from .env.example before deploying."
[ -f "docker-compose.yml" ] || fail "docker-compose.yml not found."

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  fail "deploy.sh must run inside a git working tree"
fi

if [ "${ALLOW_DIRTY:-0}" != "1" ]; then
  git diff --quiet || fail "tracked files have unstaged changes. Commit/stash them or set ALLOW_DIRTY=1."
  git diff --cached --quiet || fail "tracked files have staged changes. Commit/stash them or set ALLOW_DIRTY=1."
fi

current_branch="$(git rev-parse --abbrev-ref HEAD)"
log "pulling latest code on branch ${current_branch}"
git pull --ff-only

log "validating compose config"
compose config >/dev/null

log "building and starting containers"
compose up -d --build

log "waiting for health check: ${APP_URL}/health"
for i in $(seq 1 "$HEALTH_RETRIES"); do
  if curl -fsS "${APP_URL}/health" >/dev/null; then
    log "health check passed"
    compose ps
    exit 0
  fi
  log "health check not ready (${i}/${HEALTH_RETRIES})"
  sleep "$HEALTH_INTERVAL"
done

compose ps
compose logs --tail=80 app || true
fail "health check failed after ${HEALTH_RETRIES} attempts"
