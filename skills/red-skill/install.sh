#!/usr/bin/env bash
# 安装 red-skill：默认软链到 ~/.qoder-cn/skills/jameslee-red。
# 用法：./install.sh [--to DIR] [--copy]
set -eu

SRC="$(cd "$(dirname "$0")" && pwd)"
DEST_DEFAULT="$HOME/.qoder-cn/skills/jameslee-red"
DEST="$DEST_DEFAULT"
MODE="link"

while [ $# -gt 0 ]; do
  case "$1" in
    --to)   [ $# -ge 2 ] || { echo "缺少 --to 参数" >&2; exit 2; }; DEST="$2"; shift 2 ;;
    --copy) MODE="copy"; shift ;;
    -h|--help) sed -n '2,4p' "$0"; exit 0 ;;
    *) echo "未知参数: $1" >&2; exit 2 ;;
  esac
done

python3 -c "import yaml" 2>/dev/null || { echo "缺少 PyYAML：pip3 install pyyaml" >&2; exit 1; }

if [ -L "$DEST" ]; then
  if [ "$(cd "$DEST" && pwd -P)" = "$SRC" ]; then
    echo "已安装（软链指向 $SRC），无需重复操作：$DEST"
    exit 0
  fi
  if [ "$MODE" = "link" ]; then
    rm -f "$DEST"
    ln -s "$SRC" "$DEST"
    echo "已改指向：$DEST -> $SRC"
    exit 0
  fi
  echo "目标是软链，--copy 不覆盖：$DEST" >&2
  exit 1
fi

if [ -e "$DEST" ]; then
  echo "目标已存在，拒绝覆盖：$DEST" >&2
  echo "如确认无用，请手动移走后重试。" >&2
  exit 1
fi

mkdir -p "$(dirname "$DEST")"

if [ "$MODE" = "copy" ]; then
  mkdir -p "$DEST"
  cp -R "$SRC/SKILL.md" "$SRC/README.md" "$SRC/INDEX.md" "$SRC/index.json" \
        "$SRC/CHANGELOG.md" "$SRC/cards" "$SRC/meta" "$SRC/tests" "$SRC/scripts" "$SRC/library" "$DEST/"
  echo "已复制到：$DEST"
else
  ln -s "$SRC" "$DEST"
  echo "已软链：$DEST -> $SRC"
fi

echo "自检：python3 \"$DEST/scripts/index.py\" check"
echo "调用：RED_SKILL=\"$DEST\"（或保持默认源路径）"
