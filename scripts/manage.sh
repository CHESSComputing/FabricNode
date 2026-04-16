#!/usr/bin/env bash
# scripts/manage.sh
# ─────────────────────────────────────────────────────────────────────────────
# Process manager for the CHESS Federated Knowledge Fabric Node.
# Starts, stops, and reports status of all four services without Docker.
#
# Usage:
#   ./scripts/manage.sh start   [service...]   # start all or named services
#   ./scripts/manage.sh stop    [service...]   # stop all or named services
#   ./scripts/manage.sh restart [service...]   # stop then start
#   ./scripts/manage.sh status  [service...]   # show PID, port, health
#   ./scripts/manage.sh logs    <service>      # tail log for one service
#   ./scripts/manage.sh logs    --all          # tail all logs (merged)
#   ./scripts/manage.sh build   [service...]   # (re)build binaries
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

# ── Colours ───────────────────────────────────────────────────────────────────
if [ -t 1 ] && [ "${NO_COLOR:-}" = "" ]; then
  BOLD='\033[1m'; RESET='\033[0m'
  RED='\033[31m'; GREEN='\033[32m'; YELLOW='\033[33m'; CYAN='\033[36m'
else
  BOLD=''; RESET=''; RED=''; GREEN=''; YELLOW=''; CYAN=''
fi

# ── Paths ─────────────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
SVC_DIR="$ROOT_DIR/services"
RUN_DIR="$ROOT_DIR/.run"       # PID files
LOG_DIR="$ROOT_DIR/.logs"      # per-service log files

mkdir -p "$RUN_DIR" "$LOG_DIR"

# ── Service registry ──────────────────────────────────────────────────────────
# Format: name:port:binary_path:env_overrides
declare -A SVC_PORT=(
  [catalog-service]=8781
  [data-service]=8782
  [identity-service]=8783
  [notification-service]=8784
)

declare -A SVC_ENV=(
  [catalog-service]="PORT=8781 NODE_BASE_URL=http://localhost:8781"
  [data-service]="PORT=8782"
  [identity-service]="PORT=8783 NODE_BASE_URL=http://localhost:8783 CATALOG_URL=http://localhost:8781 DATA_URL=http://localhost:8782 NOTIFICATION_URL=http://localhost:8784"
  [notification-service]="PORT=8784"
)

ALL_SERVICES=(catalog-service data-service identity-service notification-service)

# ── Helpers ───────────────────────────────────────────────────────────────────

info()    { printf "${CYAN}→${RESET} %s\n" "$*"; }
ok()      { printf "${GREEN}✓${RESET} %s\n" "$*"; }
warn()    { printf "${YELLOW}⚠${RESET} %s\n" "$*"; }
err()     { printf "${RED}✗${RESET} %s\n" "$*" >&2; }
bold()    { printf "${BOLD}%s${RESET}\n" "$*"; }

pid_file()  { echo "$RUN_DIR/$1.pid"; }
log_file()  { echo "$LOG_DIR/$1.log"; }
bin_path()  { echo "$SVC_DIR/$1/bin/$1"; }

get_pid() {
  local pf; pf=$(pid_file "$1")
  [ -f "$pf" ] && cat "$pf" || echo ""
}

is_running() {
  local pid; pid=$(get_pid "$1")
  [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null
}

# Resolve service list: if args given use them, else all services
resolve_services() {
  if [ $# -eq 0 ]; then
    echo "${ALL_SERVICES[@]}"
  else
    for s in "$@"; do
      if [ -z "${SVC_PORT[$s]+x}" ]; then
        err "Unknown service: $s  (known: ${ALL_SERVICES[*]})"
        exit 1
      fi
      echo "$s"
    done
  fi
}

# ── build ─────────────────────────────────────────────────────────────────────
cmd_build() {
  local services; services=$(resolve_services "$@")
  for svc in $services; do
    info "Building $svc"
    (cd "$SVC_DIR/$svc" && make build --no-print-directory) \
      && ok "$svc binary ready" \
      || { err "Build failed for $svc"; exit 1; }
  done
}

# ── start ─────────────────────────────────────────────────────────────────────
cmd_start() {
  local services; services=$(resolve_services "$@")
  for svc in $services; do
    if is_running "$svc"; then
      warn "$svc already running (PID $(get_pid "$svc"))"
      continue
    fi

    local bin; bin=$(bin_path "$svc")
    if [ ! -x "$bin" ]; then
      info "Binary not found — building $svc first"
      (cd "$SVC_DIR/$svc" && make build --no-print-directory)
    fi

    local log; log=$(log_file "$svc")
    local pf;  pf=$(pid_file "$svc")
    local env_str="${SVC_ENV[$svc]}"

    info "Starting $svc (port ${SVC_PORT[$svc]}, log: .logs/$svc.log)…"

    # Launch in background with env vars, redirect stdout+stderr to log file
    # Using env(1) so the env string is split cleanly without eval
    env $env_str "$bin" >> "$log" 2>&1 &
    local pid=$!
    echo "$pid" > "$pf"

    # Brief pause then verify it stayed up
    sleep 0.4
    if kill -0 "$pid" 2>/dev/null; then
      ok "$svc started  PID=$pid"
    else
      err "$svc failed to start — check $(log_file "$svc")"
      rm -f "$pf"
      exit 1
    fi
  done
}

# ── stop ──────────────────────────────────────────────────────────────────────
cmd_stop() {
  local services; services=$(resolve_services "$@")
  # Stop in reverse order (dependents first)
  local reversed=()
  for svc in $services; do reversed=("$svc" "${reversed[@]+${reversed[@]}}"); done

  for svc in "${reversed[@]}"; do
    local pid; pid=$(get_pid "$svc")
    local pf;  pf=$(pid_file "$svc")
    if [ -z "$pid" ]; then
      warn "$svc — no PID file found (already stopped?)"
      continue
    fi
    if kill -0 "$pid" 2>/dev/null; then
      info "Stopping $svc (PID $pid)"
      kill "$pid"
      # Wait up to 5 seconds for clean shutdown
      local i=0
      while kill -0 "$pid" 2>/dev/null && [ "$i" -lt 10 ]; do
        sleep 0.5; i=$((i + 1))
      done
      if kill -0 "$pid" 2>/dev/null; then
        warn "  $svc did not stop cleanly — sending SIGKILL"
        kill -9 "$pid" 2>/dev/null || true
      fi
      ok "$svc stopped"
    else
      warn "$svc — PID $pid not found (process already gone)"
    fi
    rm -f "$pf"
  done
}

# ── restart ───────────────────────────────────────────────────────────────────
cmd_restart() {
  cmd_stop "$@"
  sleep 0.3
  cmd_start "$@"
}

# ── status ────────────────────────────────────────────────────────────────────
cmd_status() {
  local services; services=$(resolve_services "$@")

  printf "\n"
  printf "${BOLD}%-24s %-8s %-6s %-10s %s${RESET}\n" \
    "SERVICE" "STATUS" "PID" "PORT" "HEALTH"
  printf '%0.s─' {1..70}; printf '\n'

  for svc in $services; do
    local port="${SVC_PORT[$svc]}"
    local pid;    pid=$(get_pid "$svc")
    local status_str health_str pid_str

    if is_running "$svc"; then
      status_str="${GREEN}running${RESET}"
      pid_str="$pid"
      # Health check via HTTP
      local http_code
      http_code=$(curl -so /dev/null -w "%{http_code}" \
        --connect-timeout 1 --max-time 2 \
        "http://localhost:$port/health" 2>/dev/null || echo "000")
      if [ "$http_code" = "200" ]; then
        health_str="${GREEN}healthy${RESET}"
      else
        health_str="${YELLOW}unreachable (HTTP $http_code)${RESET}"
      fi
    else
      status_str="${RED}stopped${RESET}"
      pid_str="-"
      health_str="-"
    fi

    printf "%-24s ${status_str}%-$((8 - ${#status_str}))s %-6s %-10s ${health_str}\n" \
      "$svc" "" "$pid_str" "$port"
  done
  printf "\n"

  # Show log tail hints for any stopped services
  for svc in $services; do
    local log; log=$(log_file "$svc")
    if ! is_running "$svc" && [ -f "$log" ]; then
      printf "  ${YELLOW}Last log lines for $svc:${RESET}\n"
      tail -3 "$log" | sed 's/^/    /'
      printf "\n"
    fi
  done
}

# ── logs ──────────────────────────────────────────────────────────────────────
cmd_logs() {
  if [ "${1:-}" = "--all" ] || [ $# -eq 0 ]; then
    # Merge all log files with service name prefix using tail -f
    info "Tailing all service logs (Ctrl+C to stop)…"
    # Build list of existing log files
    local log_args=()
    for svc in "${ALL_SERVICES[@]}"; do
      local log; log=$(log_file "$svc")
      [ -f "$log" ] && log_args+=("$log")
    done
    if [ ${#log_args[@]} -eq 0 ]; then
      warn "No log files found yet. Start services first."
      exit 0
    fi
    # Use multitail if available, otherwise tail -f with named files
    if command -v multitail &>/dev/null; then
      local mt_args=()
      for svc in "${ALL_SERVICES[@]}"; do
        local log; log=$(log_file "$svc")
        [ -f "$log" ] && mt_args+=(-l "tail -f $log" -t "$svc")
      done
      multitail "${mt_args[@]}"
    else
      # Plain tail -f — shows filename as header when multiple files given
      tail -f "${log_args[@]}"
    fi
  else
    local svc="$1"
    if [ -z "${SVC_PORT[$svc]+x}" ]; then
      err "Unknown service: $svc"
      exit 1
    fi
    local log; log=$(log_file "$svc")
    if [ ! -f "$log" ]; then
      warn "No log file for $svc yet (.logs/$svc.log)"
      exit 0
    fi
    info "Tailing $svc log (Ctrl+C to stop)…"
    tail -f "$log"
  fi
}

# ── usage ─────────────────────────────────────────────────────────────────────
usage() {
  bold "CHESS Fabric Node — Service Manager"
  printf "\n"
  printf "Usage: %s <command> [service...]\n\n" "$(basename "$0")"
  printf "Commands:\n"
  printf "  ${CYAN}build${RESET}   [svc...]   (Re)build service binaries\n"
  printf "  ${CYAN}start${RESET}   [svc...]   Start services in background\n"
  printf "  ${CYAN}stop${RESET}    [svc...]   Stop services gracefully\n"
  printf "  ${CYAN}restart${RESET} [svc...]   Stop then start services\n"
  printf "  ${CYAN}status${RESET}  [svc...]   Show PID, port, health check\n"
  printf "  ${CYAN}logs${RESET}    <svc>      Tail log for one service\n"
  printf "  ${CYAN}logs${RESET}    --all      Tail all logs merged\n"
  printf "\n"
  printf "Services:\n"
  for svc in "${ALL_SERVICES[@]}"; do
    printf "  ${CYAN}%-26s${RESET} port %s\n" "$svc" "${SVC_PORT[$svc]}"
  done
  printf "\n"
  printf "Examples:\n"
  printf "  %s start                    # start all four services\n" "$(basename "$0")"
  printf "  %s start catalog-service    # start one service\n" "$(basename "$0")"
  printf "  %s status                   # show all statuses\n" "$(basename "$0")"
  printf "  %s logs catalog-service     # tail catalog log\n" "$(basename "$0")"
  printf "  %s logs --all               # tail all logs\n" "$(basename "$0")"
  printf "  %s stop                     # stop all\n" "$(basename "$0")"
  printf "\n"
  printf "Log files:  .logs/<service>.log\n"
  printf "PID files:  .run/<service>.pid\n"
}

# ── Entry point ───────────────────────────────────────────────────────────────
CMD="${1:-help}"
shift || true

case "$CMD" in
  build)   cmd_build   "$@" ;;
  start)   cmd_start   "$@" ;;
  stop)    cmd_stop    "$@" ;;
  restart) cmd_restart "$@" ;;
  status)  cmd_status  "$@" ;;
  logs)    cmd_logs    "$@" ;;
  help|--help|-h) usage ;;
  *)
    err "Unknown command: $CMD"
    usage
    exit 1
    ;;
esac
