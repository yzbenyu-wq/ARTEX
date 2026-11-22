#!/bin/sh
# 把 ./backups 里最新的 pg_dump 用 restic 加密同步到外部磁盘（compose profile: offsite）。
# 只在显式启用 profile 时启动；目标目录由 BACKUP_OFFSITE_HOST_DIR 指定。
#
# 先复制到容器内 /tmp 再备份：macOS 上 restic 直接读 VirtioFS 挂载的大文件会报
# input/output error，复制后读容器本地层稳定，也避免明文落盘到外部磁盘。
set -u

: "${BACKUP_DIR:=/backups}"
: "${OFFSITE_INTERVAL_SECONDS:=86400}"
: "${RESTIC_KEEP_DAILY:=7}"
: "${RESTIC_KEEP_WEEKLY:=8}"
: "${OFFSITE_STAGE_DIR:=/tmp/offsite-stage}"

log() { echo "[offsite] $(date '+%Y-%m-%d %H:%M:%S') $*"; }

if [ -z "${RESTIC_PASSWORD:-}" ]; then
	log "缺少 RESTIC_PASSWORD，未开始备份"
	exit 1
fi

# 首次使用时建仓库；已有仓库则复用（restic 会做去重，不会重复占空间）。
if ! restic snapshots >/dev/null 2>&1; then
	if ! restic init >/dev/null 2>&1; then
		log "无法初始化 restic 仓库，请检查外部磁盘是否已挂载并设置 BACKUP_OFFSITE_HOST_DIR"
		exit 1
	fi
	log "已初始化 restic 仓库"
fi

sync_once() {
	latest=$(ls -t "$BACKUP_DIR"/artex-*.dump 2>/dev/null | head -n 1)
	if [ -z "$latest" ]; then
		log "还没有可同步的 dump，跳过"
		return 0
	fi
	name=$(basename "$latest")
	rm -rf "$OFFSITE_STAGE_DIR"
	mkdir -p "$OFFSITE_STAGE_DIR" || return 1
	if ! cp "$latest" "$OFFSITE_STAGE_DIR/$name"; then
		log "复制到临时目录失败：$name"
		rm -rf "$OFFSITE_STAGE_DIR"
		return 1
	fi
	if ! restic backup "$OFFSITE_STAGE_DIR/$name" --tag artex-db >/dev/null 2>&1; then
		log "同步失败：$name"
		rm -rf "$OFFSITE_STAGE_DIR"
		return 1
	fi
	rm -rf "$OFFSITE_STAGE_DIR"
	restic forget \
		--tag artex-db \
		--keep-daily "$RESTIC_KEEP_DAILY" \
		--keep-weekly "$RESTIC_KEEP_WEEKLY" \
		--prune >/dev/null 2>&1 || log "清理旧快照失败（不影响本次备份）"
	log "已同步 $name"
	return 0
}

while :; do
	tries=0
	until sync_once; do
		tries=$((tries + 1))
		if [ "$tries" -ge 3 ]; then
			log "连续 $tries 次失败，等下一轮重试"
			break
		fi
		log "失败（第 $tries 次），60 秒后重试"
		sleep 60
	done
	sleep "$OFFSITE_INTERVAL_SECONDS"
done
