# Бот MAX: приём вебхуков платформы.
#
# ВАЖНО: контекст сборки — КОРЕНЬ репозитория, а не apps/bot
# (в compose это `context: ..` + `dockerfile: apps/bot/Dockerfile`).
# Причина: модуль в проекте один — github.com/ArthurBabkin/max-hackathon, —
# и бинарь импортирует packages/shared. Если сузить контекст до apps/bot,
# сборка не увидит ни go.mod в корне, ни общие пакеты.
#
# Собираем apps/bot/cmd/local — обычный http.Server для локального стенда.
# Прод собирается из apps/bot/cmd/function как плагин Cloud Functions и
# в Docker не участвует.

# ---------- сборка ----------
FROM golang:1.23-alpine AS build

WORKDIR /src

# Манифесты копируем отдельно от кода: слой с зависимостями переиспользуется,
# пока не менялся go.mod, и правка любого .go не тянет повторное скачивание.
# go.sum может ещё не существовать (внешних зависимостей нет) — шаблон go.sum*
# это переживает, потому что go.mod в том же COPY совпал.
COPY go.mod go.sum* ./
RUN go mod download

# Копируем apps/ и packages/ целиком, а не только apps/bot: эти два слоя
# побайтово одинаковы у бота, api и reminders, поэтому BuildKit переиспользует
# их между тремя образами и распаковывает исходники один раз на всю сборку.
COPY packages/ ./packages/
COPY apps/ ./apps/

# CGO выключен: статический бинарь запускается в голом alpine без завязки на
# системный libc. -trimpath убирает пути сборочной машины (сборка становится
# воспроизводимой у любого проверяющего), -s -w выкидывают таблицы отладки.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/server ./apps/bot/cmd/local

# ---------- рантайм ----------
# Отдельная лёгкая стадия: тулчейн Go (~300 МБ) в образ не едет.
FROM alpine:3.20

# ca-certificates нужны для исходящего HTTPS к Bot API MAX и Managed PostgreSQL —
# в базовом alpine корневых сертификатов нет, без них TLS падает с x509.
# tzdata нужна, чтобы дедлайны олимпиад считались в Europe/Moscow, а не в UTC.
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -H -u 10001 app

WORKDIR /app
COPY --from=build /app/server /app/server

# Порт читается из PORT — тот же контракт, что в compose и в Cloud Functions.
ENV PORT=8080 \
    TZ=Europe/Moscow

# Непривилегированный пользователь: процессу не нужны права root,
# а у проверяющего смонтированные тома не обрастут root-файлами.
USER app

EXPOSE 8080
ENTRYPOINT ["/app/server"]
