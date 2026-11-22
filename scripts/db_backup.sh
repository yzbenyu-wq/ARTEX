#!/bin/sh
# 定时备份 PostgreSQL 到 $BACKUP_DIR（compose 里挂成 ./backups）。
# 单容器串行执行：先落 .part 再原子改名，失败不会留下半截文件被当成有效备份。
# 每份 dump 生成后先做 pg_restore --list 完整性校验；距上次演练超过 BACKUP_DRILL_EVERY 天
# 时，把最新 dump 灌进临时库比对关键表行数，无误后删掉临时库。
set -u

: "${PGHOST:=postgres}"
: "${PGUSER:=artex}"
: "${PGDATABASE:=artex}"
: "${BACKUP_DIR:=/backups}"
: "${BACKUP_INTERVAL_SECONDS:=86400}"
: "${BACKUP_KEEP_DAYS:=7}"
: "${BACKUP_DRILL_EVERY:=7}"
: "${BACKUP_DRILL_DB:=artex_drill}"
: "${BACKUP_DRILL_TABLES:=tasks assets findings llm_usage settings exploration_nodes}"

mkdir -p "$BACKUP_DIR"

log() { echo "[backup] $(date '+%Y-%m-%d %H:%M:%S') $*"; }

# 只用 psql 的目录表：拿 dump 的文件清单，不连库也不改任何东西。
verify_dump() {
	if pg_restore --list "$1" >/dev/null 2>&1; then
		return 0
	fi
	log "校验失败，丢弃损坏备份 $1"
	mv "$1" "$1.corrupt" 2>/dev/null || rm -f "$1"
	return 1
}

drop_drill_db() {
	psql -d postgres -qc "DROP DATABASE IF EXISTS \"$BACKUP_DRILL_DB\" WITH (FORCE)" >/dev/null 2>&1
}

drill_once() {
	latest=$1
	# 全新建库时业务表还没建，跳过本次演练且不写标记，等下一次 dump 再试。
	if [ -z "$(psql -d "$PGDATABASE" -Atc "SELECT to_regclass('public.tasks')" 2>/dev/null)" ]; then
		log "演练跳过：业务表尚未创建"
		return 2
	fi
	log "开始恢复演练：$latest -> $BACKUP_DRILL_DB"
	drop_drill_db
	if ! psql -d postgres -qc "CREATE DATABASE \"$BACKUP_DRILL_DB\"" >/dev/null; then
		log "演练失败：无法创建临时库"
		return 1
	fi
	if ! pg_restore --exit-on-error --no-owner --no-privileges --dbname="$BACKUP_DRILL_DB" "$latest" >/dev/null 2>&1; then
		log "演练失败：pg_restore 未能完整还原"
		drop_drill_db
		return 1
	fi
	mismatch=0
	for table in $BACKUP_DRILL_TABLES; do
		src=$(psql -d "$PGDATABASE" -Atc "SELECT count(*) FROM $table" 2>/dev/null)
		dst=$(psql -d "$BACKUP_DRILL_DB" -Atc "SELECT count(*) FROM $table" 2>/dev/null)
		if [ -z "$src" ] || [ "$src" != "$dst" ]; then
			log "演练不一致：$table 线上=$src 还原=$dst"
			mismatch=1
		fi
	done
	drop_drill_db
	if [ "$mismatch" -ne 0 ]; then
		log "演练失败：行数比对不一致"
		return 1
	fi
	log "演练通过：$BACKUP_DRILL_TABLES 行数一致"
	return 0
}

drill_due() {
	marker="$BACKUP_DIR/.last-drill"
	[ -f "$marker" ] || return 0
	[ "$(find "$marker" -mtime +"$((BACKUP_DRILL_EVERY - 1))" | wc -l | tr -d ' ')" = "1" ]
}

dump_once() {
	ts=$(date '+%Y%m%d-%H%M%S')
	final="$BACKUP_DIR/artex-$ts.dump"
	part="$BACKUP_DIR/.artex-$ts.dump.part"
	if ! pg_dump --format=custom --file="$part"; then
		rm -f "$part"
		return 1
	fi
	mv "$part" "$final" || { rm -f "$part"; return 1; }
	log "ok $final ($(du -h "$final" | cut -f1))"
	verify_dump "$final" || return 1
	if drill_due; then
		drill_once "$final"
		case $? in
		0) touch "$BACKUP_DIR/.last-drill" ;;
		2) log "演练待下一次备份重试" ;;
		esac
	fi
	return 0
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
	find "$BACKUP_DIR" -maxdepth 1 -name '*.corrupt' -mtime +7 -delete
	sleep "$BACKUP_INTERVAL_SECONDS"
done
