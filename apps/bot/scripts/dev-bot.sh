#!/usr/bin/env bash
# Разработка: отправить боту событие от имени пользователя MAX, как будто
# он нажал «Начать» или написал текст. Нужен, когда клиент MAX не отправляет
# сообщения боту, а inline-кнопки работают.
#
# Скрипт шлёт обновление в тот же вебхук, что и MAX, с секретом вебхука.
# Секреты берутся из Lockbox traektoria-app через yc (нужен доступ к облаку)
# и не печатаются. Можно задать их в окружении: WEBHOOK_SECRET, MAX_BOT_TOKEN.
#
#   apps/bot/scripts/dev-bot.sh users
#       последние пользователи бота из базы: user_id и имя — чтобы узнать свой
#   apps/bot/scripts/dev-bot.sh dialogs
#       то же из MAX API (GET /chats может не отдавать личные диалоги)
#   apps/bot/scripts/dev-bot.sh start <user_id> <имя> [payload]
#       как /start или «Начать»: бот пришлёт приветствие, дальше — кнопками
#   apps/bot/scripts/dev-bot.sh text <user_id> <имя> <текст>
#       как сообщение боту: /delete, /menu, имя ребёнка, поиск вуза
#
# Имя передаётся потому, что бот обновляет его из каждого события: без него
# «Тебя зовут …?» спросит не то имя.

set -euo pipefail

SECRET_NAME=traektoria-app
FUNCTION_NAME=traektoria-bot
MAX_API=https://platform-api2.max.ru
ROOT=$(cd "$(dirname "$0")/../../.." && pwd)
# Сертификат MAX выпущен корнем Минцифры — его нет в системных корнях.
CA="$ROOT/packages/shared/maxapi/russian_trusted_root_ca.pem"

usage() {
  sed -n '9,20p' "$0" | sed 's/^# \{0,1\}//'
  exit 2
}

secret() {
  local key=$1
  if [ -n "${!key:-}" ]; then
    printf '%s' "${!key}"
    return
  fi
  yc lockbox payload get "$SECRET_NAME" --key "$key"
}

webhook_url() {
  if [ -n "${BOT_WEBHOOK_URL:-}" ]; then
    printf '%s' "$BOT_WEBHOOK_URL"
    return
  fi
  local id
  id=$(yc serverless function get "$FUNCTION_NAME" --format json | jq -r .id)
  printf 'https://functions.yandexcloud.net/%s' "$id"
}

# post отправляет обновление в вебхук и печатает ответ бота.
post() {
  local body=$1 code
  code=$(curl -sS -o /dev/stderr -w '%{http_code}' -X POST "$(webhook_url)" \
    -H 'Content-Type: application/json' \
    -H "X-Max-Bot-Api-Secret: $(secret WEBHOOK_SECRET)" \
    --data "$body")
  echo
  if [ "$code" = 200 ]; then
    echo "✓ бот принял событие — ответ придёт в чат"
  else
    echo "✗ вебхук ответил $code" >&2
    exit 1
  fi
}

now_ms() { echo $(($(date +%s) * 1000)); }

cmd=${1:-}
case "$cmd" in
  users)
    # Каждый, кто хоть раз нажал кнопку или начал диалог, есть в users.
    psql "$(secret DATABASE_URL)" -X -A -F $'\t' -c \
      "SELECT max_user_id, first_name, created_at::date FROM users ORDER BY created_at DESC LIMIT 20"
    ;;
  dialogs)
    resp=$(curl -sS --cacert "$CA" -H "Authorization: $(secret MAX_BOT_TOKEN)" "$MAX_API/chats?count=100")
    if ! jq -e '.chats' >/dev/null 2>&1 <<<"$resp"; then
      echo "MAX ответил не списком чатов: $resp" >&2
      exit 1
    fi
    found=$(jq -r '.chats[] | select(.type == "dialog") | .dialog_with_user
                   | "\(.user_id)\t\(.first_name) \(.last_name // "")"' <<<"$resp")
    if [ -z "$found" ]; then
      echo "В ответе $(jq '.chats | length' <<<"$resp") чатов, личных диалогов нет — попробуйте: $0 users" >&2
      exit 1
    fi
    echo "$found"
    ;;
  start)
    [ $# -ge 3 ] || usage
    body=$(jq -nc --argjson uid "$2" --arg name "$3" --arg payload "${4:-}" --argjson ts "$(now_ms)" '
      {update_type: "bot_started", timestamp: $ts, chat_id: $uid,
       user: {user_id: $uid, first_name: $name}}
      + (if $payload == "" then {} else {payload: $payload} end)')
    post "$body"
    ;;
  text)
    [ $# -ge 4 ] || usage
    ts=$(now_ms)
    body=$(jq -nc --argjson uid "$2" --arg name "$3" --arg text "$4" --argjson ts "$ts" --arg mid "dev.$ts.$RANDOM" '
      {update_type: "message_created", timestamp: $ts,
       message: {sender: {user_id: $uid, first_name: $name},
                 recipient: {chat_type: "dialog", user_id: $uid},
                 timestamp: $ts, body: {mid: $mid, text: $text}}}')
    post "$body"
    ;;
  *)
    usage
    ;;
esac
