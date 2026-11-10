#!/bin/sh
# soak_par.sh TEST JOIN JOBS RUN... : one soak's seeds as separate test
# processes, JOBS at once, then the join (make soak-autonz, make soak-hive;
# docs/developer/factions.md, "The soaks"). The soaks share package globals
# (the settlement map, the path budget, the content tables), so their seeds
# cannot be parallel subtests in one process. Each process runs one seed
# (LP_SOAK_ONLY=<run>) and writes its row as JSON into a fresh temporary
# directory (LP_SOAK_ROWS); JOIN reads the rows back, prints the table and
# applies the thresholds as a one-process run does. The caller's
# environment (LP_SOAK_AUTONZ_TICKS, LP_SOAK_HIVE_TICKS) passes through. The
# temporary directory is left for the system to clear (each run's log is
# there when a seed fails).
set -e
test_name=$1
join=$2
jobs=$3
shift 3
rows=$(mktemp -d "${TMPDIR:-/tmp}/lp-soak-rows.XXXXXX")
bin="$rows/game.test"
go test -c -o "$bin" ./internal/game
export LP_SOAK=1
export LP_SOAK_ROWS="$rows"
cd internal/game
# A failed seed is reported and the join still runs (it prints the rows the
# other seeds wrote); the script then fails.
failed=0
printf '%s\n' "$@" | xargs -P "$jobs" -I{} sh -c \
	'LP_SOAK_ONLY={} "$0" -test.run "^$1\$" -test.count=1 -test.timeout 60m > "$2/{}.log" 2>&1 || { echo "soak run {} failed:"; tail -30 "$2/{}.log"; exit 1; }' \
	"$bin" "$test_name" "$rows" || failed=1
"$bin" -test.run "^$join\$" -test.v -test.count=1 || failed=1
exit $failed
