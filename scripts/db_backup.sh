#!/bin/sh
# 定时备份 PostgreSQL 到 $BACKUP_DIR（compose 里挂成 ./backups）。
# 单容器串行执行：先落 .part 再原子改名，失败不会留下半截文件被当成有效备份。
set -u

: "${PGHOST:=postgres}"
: "${PGUSER:=artex}"
: "${PGDATABASE:=artex}"
: "${BACKUP_DIR:=/backups}"
: "${BACKUP_INTERVAL_SECONDS:=86400}"
: "${BACKUP_KEEP_DAYS:=7}"

mkdir -p "$BACKUP_DIR"

log() { echo "[backup] $(date '+%Y-%m-%d %H:%M:%S') $*"; }

dump_once() {
	ts=$(date '+%Y%m%d-%H%M%S')
	final="$BACKUP_DIR/artex-$ts.dump"
	part="$BACKUP_DIR/.artex-$ts.dump.part"
	if pg_dump --format=custom --file="$part" && mv "$part" "$final"; then
		log "ok $final ($(du -h "$final" | cut -f1))"
		return 0
	fi
	rm -f "$part"
	return 1
}

while :; do
	tries=0
	until dump_once; do
		tries=$((tries + 1))
		if [ "$tries" -ge 3 ]; then
			log "连续 $tries 次失败，等下一轮重试"
			break
		fi
		log "失败（第 $tries 次），30 秒后重试"
		sleep 30
	done
	# 保留最近 $BACKUP_KEEP_DAYS 天：mtime 超过 N-1 天的删掉。
	find "$BACKUP_DIR" -maxdepth 1 -name 'artex-*.dump' -mtime +"$((BACKUP_KEEP_DAYS - 1))" -delete
	sleep "$BACKUP_INTERVAL_SECONDS"
done
