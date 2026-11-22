#!/bin/sh
# 每个 Go 包一个独立空库跑测试：包内测试会删改自己库里的数据，
# 共享一个库时并行包会互相踩（历史基线就是这样跑的）。
# 需要 psql 可用（CI 用 postgres service，本地用 docker compose 里的 postgres）。
set -u

: "${PGHOST:=127.0.0.1}"
: "${PGPORT:=5432}"
: "${PGUSER:=artex}"
: "${PGDATABASE:=postgres}"
export PGHOST PGPORT PGUSER PGDATABASE

fail=0
failed=""
i=0

psql_admin() {
	psql -v ON_ERROR_STOP=1 -d "$PGDATABASE" -Atc "$1" >/dev/null
}

for pkg in $(go list ./...); do
	i=$((i + 1))
	name=$(printf '%s' "$pkg" | sed 's#^github.com/[^/]*/[^/]*/##; s#[^A-Za-z0-9]#_#g')
	dbname="ci_${i}_${name}"
	dbname=$(printf '%s' "$dbname" | cut -c1-63)

	psql_admin "DROP DATABASE IF EXISTS \"$dbname\"" || { fail=1; failed="$failed $pkg(drop)"; continue; }
	psql_admin "CREATE DATABASE \"$dbname\"" || { fail=1; failed="$failed $pkg(create)"; continue; }

	echo "=== $pkg -> $dbname"
	ARTEX_PG_DSN="postgres://${PGUSER}:${PGPASSWORD:-}@${PGHOST}:${PGPORT}/${dbname}?sslmode=disable" \
		go test -count=1 -timeout 20m "$pkg"
	rc=$?
	if [ "$rc" -ne 0 ]; then
		fail=1
		failed="$failed $pkg"
	fi
	psql_admin "DROP DATABASE IF EXISTS \"$dbname\" WITH (FORCE)" || true
done

if [ "$fail" -ne 0 ]; then
	echo "FAILED:$failed"
	exit 1
fi
echo "ALL GO PACKAGES PASSED"
