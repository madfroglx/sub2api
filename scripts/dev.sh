#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEV_DIR="${SUB2API_DEV_DIR:-$ROOT_DIR/.dev}"
PID_DIR="$DEV_DIR/pids"
LOG_DIR="$DEV_DIR/logs"
BACKEND_DATA_DIR="${SUB2API_DATA_DIR:-$DEV_DIR/backend-data}"

BACKEND_PORT="${SUB2API_BACKEND_PORT:-8080}"
FRONTEND_PORT="${SUB2API_FRONTEND_PORT:-3000}"
BACKEND_URL="${SUB2API_BACKEND_URL:-http://localhost:$BACKEND_PORT}"

BACKEND_PID="$PID_DIR/backend.pid"
FRONTEND_PID="$PID_DIR/frontend.pid"
BACKEND_LOG="$LOG_DIR/backend.log"
FRONTEND_LOG="$LOG_DIR/frontend.log"

usage() {
  cat <<EOF
Usage:
  scripts/dev.sh start [backend|frontend|all]
  scripts/dev.sh stop [backend|frontend|all]
  scripts/dev.sh restart [backend|frontend|all]
  scripts/dev.sh status
  scripts/dev.sh logs [backend|frontend|all]

Defaults:
  start target: all
  stop target: all
  restart target: backend
  logs target: all

Environment:
  SUB2API_BACKEND_PORT   Backend port, default: 8080
  SUB2API_FRONTEND_PORT  Frontend port, default: 3000
  SUB2API_BACKEND_URL    Frontend proxy target, default: http://localhost:\$SUB2API_BACKEND_PORT
  SUB2API_DATA_DIR       Backend DATA_DIR, default: .dev/backend-data
  SUB2API_DEV_DIR        Runtime pid/log directory, default: .dev
EOF
}

ensure_dirs() {
  mkdir -p "$PID_DIR" "$LOG_DIR" "$BACKEND_DATA_DIR"
}

pid_is_running() {
  local pid_file="$1"
  [[ -f "$pid_file" ]] || return 1
  local pid
  pid="$(cat "$pid_file" 2>/dev/null || true)"
  [[ -n "$pid" ]] || return 1
  kill -0 "$pid" 2>/dev/null
}

kill_tree() {
  local pid="$1"
  local child
  if command -v pgrep >/dev/null 2>&1; then
    while IFS= read -r child; do
      [[ -n "$child" ]] && kill_tree "$child"
    done < <(pgrep -P "$pid" 2>/dev/null || true)
  fi
  kill "$pid" 2>/dev/null || true
}

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Missing command: $cmd" >&2
    exit 1
  fi
}

start_backend() {
  ensure_dirs
  require_cmd go
  if pid_is_running "$BACKEND_PID"; then
    echo "backend already running: $(cat "$BACKEND_PID")"
    return
  fi

  if [[ ! -f "$BACKEND_DATA_DIR/config.yaml" ]]; then
    echo "backend config not found: $BACKEND_DATA_DIR/config.yaml"
    echo "backend will start setup mode unless AUTO_SETUP/config env is provided."
  fi

  (
    cd "$ROOT_DIR/backend"
    nohup env \
      DATA_DIR="$BACKEND_DATA_DIR" \
      SERVER_HOST="${SUB2API_BACKEND_HOST:-127.0.0.1}" \
      SERVER_PORT="$BACKEND_PORT" \
      SERVER_MODE="${SUB2API_SERVER_MODE:-debug}" \
      go run ./cmd/server >"$BACKEND_LOG" 2>&1 &
    echo $! >"$BACKEND_PID"
  )
  echo "backend started: http://localhost:$BACKEND_PORT"
  echo "backend log: $BACKEND_LOG"
}

start_frontend() {
  ensure_dirs
  require_cmd pnpm
  if pid_is_running "$FRONTEND_PID"; then
    echo "frontend already running: $(cat "$FRONTEND_PID")"
    return
  fi
  if [[ ! -d "$ROOT_DIR/frontend/node_modules" ]]; then
    echo "frontend dependencies not found. Run: pnpm --dir frontend install" >&2
    exit 1
  fi

  (
    cd "$ROOT_DIR"
    nohup env \
      VITE_DEV_PROXY_TARGET="$BACKEND_URL" \
      VITE_DEV_PORT="$FRONTEND_PORT" \
      pnpm --dir frontend run dev -- --host 0.0.0.0 --port "$FRONTEND_PORT" >"$FRONTEND_LOG" 2>&1 &
    echo $! >"$FRONTEND_PID"
  )
  echo "frontend started: http://localhost:$FRONTEND_PORT"
  echo "frontend log: $FRONTEND_LOG"
  echo "frontend changes are handled by Vite HMR; no restart needed for normal UI edits."
}

stop_one() {
  local name="$1"
  local pid_file="$2"
  if ! pid_is_running "$pid_file"; then
    rm -f "$pid_file"
    echo "$name not running"
    return
  fi
  local pid
  pid="$(cat "$pid_file")"
  kill_tree "$pid"
  sleep 1
  if kill -0 "$pid" 2>/dev/null; then
    kill -9 "$pid" 2>/dev/null || true
  fi
  rm -f "$pid_file"
  echo "$name stopped"
}

stop_backend() {
  stop_one backend "$BACKEND_PID"
}

stop_frontend() {
  stop_one frontend "$FRONTEND_PID"
}

status_one() {
  local name="$1"
  local pid_file="$2"
  if pid_is_running "$pid_file"; then
    echo "$name: running pid=$(cat "$pid_file")"
  else
    echo "$name: stopped"
  fi
}

show_status() {
  status_one backend "$BACKEND_PID"
  status_one frontend "$FRONTEND_PID"
  echo "backend url:  http://localhost:$BACKEND_PORT"
  echo "frontend url: http://localhost:$FRONTEND_PORT"
  echo "data dir:     $BACKEND_DATA_DIR"
}

show_logs() {
  local target="${1:-all}"
  ensure_dirs
  case "$target" in
    backend)
      touch "$BACKEND_LOG"
      tail -n 120 -f "$BACKEND_LOG"
      ;;
    frontend)
      touch "$FRONTEND_LOG"
      tail -n 120 -f "$FRONTEND_LOG"
      ;;
    all)
      touch "$BACKEND_LOG" "$FRONTEND_LOG"
      tail -n 120 -f "$BACKEND_LOG" "$FRONTEND_LOG"
      ;;
    *)
      usage
      exit 1
      ;;
  esac
}

target="${2:-}"
case "${1:-}" in
  start)
    case "${target:-all}" in
      backend) start_backend ;;
      frontend) start_frontend ;;
      all) start_backend; start_frontend ;;
      *) usage; exit 1 ;;
    esac
    ;;
  stop)
    case "${target:-all}" in
      backend) stop_backend ;;
      frontend) stop_frontend ;;
      all) stop_frontend; stop_backend ;;
      *) usage; exit 1 ;;
    esac
    ;;
  restart)
    case "${target:-backend}" in
      backend) stop_backend; start_backend ;;
      frontend) stop_frontend; start_frontend ;;
      all) stop_frontend; stop_backend; start_backend; start_frontend ;;
      *) usage; exit 1 ;;
    esac
    ;;
  status)
    show_status
    ;;
  logs)
    show_logs "${target:-all}"
    ;;
  help|-h|--help|"")
    usage
    ;;
  *)
    usage
    exit 1
    ;;
esac
