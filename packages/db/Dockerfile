# Прогон миграций для локального стенда.
#
# В compose это одноразовый сервис: поднимается после готовности postgres,
# накатывает схему и завершается. Go-сервисы ждут именно его успешного
# завершения, а не просто живой базы, — иначе они стартуют на пустой схеме.
#
# Отдельный образ с goose нужен потому, что монтировать SQL в
# /docker-entrypoint-initdb.d нельзя: psql выполнил бы файл целиком, включая
# секцию `-- +goose Down`, и снёс бы всё, что только что создал.
FROM golang:1.23-alpine

# Версия зафиксирована: миграции должны накатываться одинаково у всех
# проверяющих, а latest со временем разъедется.
RUN go install github.com/pressly/goose/v3/cmd/goose@v3.22.1

WORKDIR /migrations
COPY packages/db/migrations/ ./
# Демо-траектория для локальной проверки в браузере. Накатывается отдельным
# сервисом seed-demo со своей таблицей версий; в прод не попадает.
COPY packages/db/migrations-demo/ /migrations-demo/

# goose сам создаёт таблицу goose_db_version и накатывает только новое,
# поэтому повторный `docker compose up` безопасен.
ENTRYPOINT ["sh", "-c", "goose -dir /migrations postgres \"$DATABASE_URL\" up"]
