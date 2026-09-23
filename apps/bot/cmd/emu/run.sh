#!/usr/bin/env bash
# Эмулятор MAX с ботом внутри одной командой: make emu.
#
# База: DATABASE_URL из окружения, иначе Postgres на localhost:5432 (make db-up),
# иначе свой одноразовый кластер из brew (postgresql@15/16) в .maxemu/pg на
# порту 5433. Миграции, демо-траектория и вымышленные олимпиады накатываются
# goose'ом, как в docker-compose. Чат — http://localhost:9000.
set -euo pipefail
cd "$(dirname "$0")/../../../.."

GO=${GO:-go}
EMU_ADDR=${EMU_ADDR:-localhost:9000}
export LC_ALL=${LC_ALL:-en_US.UTF-8} # без локали Postgres на macOS не стартует

cleanup() {
  if [[ -n "${PGDIR:-}" ]]; then "$PGBIN/pg_ctl" -D "$PGDIR" -m fast stop >/dev/null 2>&1 || true; fi
}
trap cleanup EXIT INT TERM

if [[ -z "${DATABASE_URL:-}" ]]; then
  if (exec 3<>/dev/tcp/127.0.0.1/5432) 2>/dev/null; then
    DATABASE_URL='postgres://traektoria:traektoria@localhost:5432/traektoria?sslmode=disable'
  else
    PGBIN=$(ls -d /opt/homebrew/opt/postgresql@{16,15,14}/bin /usr/local/opt/postgresql@{16,15,14}/bin 2>/dev/null | head -1 || true)
    [[ -z "$PGBIN" ]] && PGBIN=$(dirname "$(command -v pg_ctl || true)")
    [[ -x "$PGBIN/pg_ctl" ]] || { echo "Нет Postgres: задайте DATABASE_URL, запустите make db-up или brew install postgresql@16" >&2; exit 1; }
    PGDIR=$PWD/.maxemu/pg
    if [[ ! -d "$PGDIR" ]]; then
      mkdir -p .maxemu
      "$PGBIN/initdb" -D "$PGDIR" -U traektoria --auth=trust -E UTF8 --locale=C >/dev/null
    fi
    "$PGBIN/pg_ctl" -D "$PGDIR" -o "-p 5433 -k /tmp" -l "$PGDIR.log" -w start >/dev/null
    "$PGBIN/createdb" -h localhost -p 5433 -U traektoria traektoria 2>/dev/null || true
    DATABASE_URL='postgres://traektoria@localhost:5433/traektoria?sslmode=disable'
  fi
fi
export DATABASE_URL

GOOSE=$(command -v goose || echo "$HOME/go/bin/goose")
[[ -x "$GOOSE" ]] || { echo "Нет goose: go install github.com/pressly/goose/v3/cmd/goose@v3.22.1" >&2; exit 1; }
"$GOOSE" -dir packages/db/migrations postgres "$DATABASE_URL" up >/dev/null
"$GOOSE" -table goose_demo_version -dir packages/db/migrations-demo postgres "$DATABASE_URL" up >/dev/null
"$GOOSE" -table goose_local_version -dir packages/db/migrations-local postgres "$DATABASE_URL" up >/dev/null
echo "база: $DATABASE_URL (миграции накатаны)"

export MAX_BOT_NAME=${MAX_BOT_NAME:-t356_hakaton_max_bot} MAX_BOT_ID=${MAX_BOT_ID:-426643746}
export EMU_ADDR APP_ENV=development
"$GO" run ./apps/bot/cmd/emu
