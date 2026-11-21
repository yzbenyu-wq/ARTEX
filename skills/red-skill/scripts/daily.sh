#!/usr/bin/env bash
# 每日索引刷新：update + check，结果追加到 logs/daily.log。失败退出 1。
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
LOG="$ROOT/logs/daily.log"
mkdir -p "$ROOT/logs"

ts() { date '+%Y-%m-%d %H:%M:%S'; }

{
  echo "===== $(ts) update ====="
  python3 "$ROOT/scripts/index.py" update
  up=$?
  echo "----- $(ts) check -----"
  python3 "$ROOT/scripts/index.py" check
  ck=$?
  echo "===== $(ts) done update=$up check=$ck ====="
} >>"$LOG" 2>&1

[ "$up" -eq 0 ] && [ "$ck" -eq 0 ]
