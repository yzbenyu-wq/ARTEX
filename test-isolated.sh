#!/usr/bin/env bash
# 每包独立空库的 Go 测试回归。
#
# ARTEX 的测试假设自己独占数据库：多个包/用例共享同一库会互相污染
# （跨包清表、建表顺序依赖），跑出与代码无关的假失败。本脚本启动一个
# 临时 PostgreSQL 容器，为每个包创建独立空库，串行执行 go test -count=1，
# 结果只反映代码本身。测试结束自动删除容器。
#
# 用法:
#   ./test-isolated.sh                # 跑全部包
#   ./test-isolated.sh ./agent ./db   # 只跑指定包
#
# 环境变量:
#   ARTEX_TEST_PG_IMAGE   PostgreSQL 镜像（默认 postgres:16-alpine）
#   ARTEX_TEST_PG_PORT    宿主机端口（默认 55432，需空闲）
#   GO_BIN                go 可执行文件（默认 PATH 中的 go，其次 ~/.local/go/bin/go）
#
# 依赖: docker daemon（Docker Desktop 等）+ go 工具链。
set -euo pipefail
cd "$(dirname "$0")"

IMAGE="${ARTEX_TEST_PG_IMAGE:-postgres:16-alpine}"
PORT="${ARTEX_TEST_PG_PORT:-55432}"
CONTAINER="artex-isolated-test-pg"

find_docker() {
  if command -v docker >/dev/null 2>&1; then command -v docker; return; fi
  local c
  for c in "$HOME/.docker/bin/docker" \
           "/Applications/Docker.app/Contents/Resources/bin/docker" \
           "/usr/local/bin/docker"; do
    if [ -x "$c" ]; then echo "$c"; return; fi
  done
  echo "找不到 docker CLI（可在 PATH 中加入 docker，或检查 Docker Desktop 是否安装）" >&2
  return 1
}
DOCKER="$(find_docker)"

# Docker Desktop 的 socket 不一定在 /var/run/docker.sock。
if [ -z "${DOCKER_HOST:-}" ] && [ ! -S /var/run/docker.sock ] && [ -S "$HOME/.docker/run/docker.sock" ]; then
  export DOCKER_HOST="unix://$HOME/.docker/run/docker.sock"
fi

if [ -n "${GO_BIN:-}" ]; then
  GO="$GO_BIN"
elif command -v go >/dev/null 2>&1; then
  GO="$(command -v go)"
elif [ -x "$HOME/.local/go/bin/go" ]; then
  GO="$HOME/.local/go/bin/go"
else
  echo "找不到 go 工具链（可设置 GO_BIN 指向 go 可执行文件）" >&2
  exit 1
fi

"$DOCKER" info >/dev/null 2>&1 || { echo "docker daemon 不可用，请先启动 Docker Desktop" >&2; exit 1; }

if command -v lsof >/dev/null 2>&1 && lsof -nP -iTCP:"$PORT" -sTCP:LISTEN >/dev/null 2>&1; then
  echo "端口 $PORT 已被占用，可用 ARTEX_TEST_PG_PORT 指定其它端口" >&2
  exit 1
fi

cleanup() { "$DOCKER" stop "$CONTAINER" >/dev/null 2>&1 || true; }
trap cleanup EXIT INT TERM

echo "[test-isolated] 启动临时 PostgreSQL（$IMAGE @ 127.0.0.1:$PORT）"
"$DOCKER" run -d --rm --name "$CONTAINER" \
  -e POSTGRES_USER=baseline -e POSTGRES_PASSWORD=baseline -e POSTGRES_DB=postgres \
  -p "127.0.0.1:$PORT:5432" "$IMAGE" >/dev/null

ready=0
for _ in $(seq 1 60); do
  if "$DOCKER" exec "$CONTAINER" pg_isready -U baseline -d postgres >/dev/null 2>&1; then
    ready=1; break
  fi
  sleep 0.5
done
if [ "$ready" -ne 1 ]; then
  echo "PostgreSQL 未在 30s 内就绪" >&2
  exit 1
fi

if [ "$#" -gt 0 ]; then
  PKGS=("$@")
else
  PKGS=()
  while IFS= read -r p; do PKGS+=("$p"); done < <("$GO" list ./...)
fi

LOG_DIR="$(mktemp -d)"
PASS=0
FAIL=0
FAILED=()
START=$(date +%s)

for pkg in "${PKGS[@]}"; do
  safe="$(printf '%s' "$pkg" | tr -c 'A-Za-z0-9' '_')"
  db="artex_$safe"
  if [ "${#db}" -gt 63 ]; then
    db="artex_$(printf '%s' "$pkg" | shasum | cut -c1-16)"
  fi
  "$DOCKER" exec "$CONTAINER" psql -U baseline -d postgres -q \
    -c "SET client_min_messages = warning" \
    -c "DROP DATABASE IF EXISTS $db" -c "CREATE DATABASE $db" </dev/null >/dev/null
  log="$LOG_DIR/$safe.log"
  if ARTEX_PG_DSN="postgres://baseline:baseline@127.0.0.1:$PORT/$db?sslmode=disable" \
     "$GO" test -count=1 "$pkg" >"$log" 2>&1; then
    PASS=$((PASS + 1))
    printf '  PASS  %s\n' "$pkg"
  else
    FAIL=$((FAIL + 1))
    FAILED+=("$pkg")
    printf '  FAIL  %s\n' "$pkg"
    grep -E '^(--- FAIL|FAIL)' "$log" | head -5 | sed 's/^/        /' || true
  fi
done

ELAPSED=$(( $(date +%s) - START ))
echo
echo "[test-isolated] $PASS 通过 / $FAIL 失败（${ELAPSED}s）"
if [ "$FAIL" -gt 0 ]; then
  echo "失败包日志保留在: $LOG_DIR"
  printf '失败包: %s\n' "${FAILED[*]}"
  exit 1
fi
rm -rf "$LOG_DIR"
