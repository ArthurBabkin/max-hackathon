# Мини-приложение MAX: сборка Vite и раздача статики nginx.
#
# Контекст сборки — КОРЕНЬ репозитория, как у Go-сервисов: фронт импортирует
# packages/api-contract и packages/shared/texts, и без них сборка не пройдёт.
# Что не уезжает в демон — см. .dockerignore в корне.

# node:22 — минимум, который требуют Vite 8 (>=22.12) и Vitest 5.
FROM node:22-alpine AS build
WORKDIR /repo

# package.json и лок копируются отдельно и до исходников: слой с npm ci
# переиспользуется, пока зависимости не менялись. Это разница между
# двадцатью секундами и тремя минутами на каждой пересборке, а у хакатона
# требование — уложиться в пять минут.
COPY apps/web/package.json apps/web/package-lock.json apps/web/
RUN cd apps/web && npm ci --no-audit --no-fund

# Общие пакеты. Отдельным слоем, потому что меняются реже кода мини-приложения.
COPY packages/api-contract/ packages/api-contract/
COPY packages/shared/texts/ packages/shared/texts/
COPY packages/core/refdata/regions.json packages/core/refdata/

COPY apps/web/ apps/web/

# Адрес API зашивается в бандл на сборке. По умолчанию — относительный путь:
# запрос идёт на тот же origin и попадает в прокси ниже, поэтому CORS
# локально не возникает вовсе.
ARG VITE_API_BASE=/api/v1
ENV VITE_API_BASE=$VITE_API_BASE
# Заглушка MAX Bridge в сборке: внутри MAX настоящий мост всё равно главнее,
# а в обычном браузере без неё приложение не откроется. Для прод-статики не
# включается: её собирают отдельно, см. infra/terraform/README.md.
ARG VITE_DEV_FAKE_WEBAPP=false
ENV VITE_DEV_FAKE_WEBAPP=$VITE_DEV_FAKE_WEBAPP

RUN cd apps/web && npm run build

# ---------------------------------------------------------------------------

FROM nginx:1.27-alpine

COPY --from=build /repo/apps/web/dist /usr/share/nginx/html

# Слушаем 8083 вместо дефолтного 80, чтобы внутренний и внешний порт совпадали:
# ссылка http://localhost:8083 одинаково работает и с хоста, и изнутри
# compose-сети, и её не надо переписывать между окружениями.
#
# Конфиг генерим здесь же, чтобы не заводить отдельный файл ради двух десятков
# строк. Одинарные кавычки не дают шеллу съесть $uri.
#
# resolver + переменная в proxy_pass — обязательны. С голым именем хоста nginx
# падает НА СТАРТЕ с «host not found in upstream», если контейнера api ещё нет,
# и ломается обещание compose, что статика открывается даже когда бэкенд лежит.
# С переменной имя резолвится лениво, и в худшем случае приходит 502.
#
# Проксируется именно /api/v1/, и путь уходит в Go-сервис без изменений: базовый
# путь /api/v1 объявлен в servers контракта (packages/api-contract/openapi.yaml),
# значит ручки там и живут. Healthcheck сервиса лежит вне контракта, в корне,
# и сюда не попадает — его видно напрямую на http://localhost:8081/health.
#
# try_files ... /index.html — мини-приложение использует hash-роутинг, но
# перезагрузка по прямому адресу всё равно не должна отдавать 404.
#
# Имена файлов в /assets/ содержат хеш содержимого, поэтому их можно кешировать
# навсегда: изменится содержимое — изменится и имя. Directive expires здесь
# не используется, иначе Cache-Control уходит двумя заголовками сразу.
RUN printf '%s\n' \
    'server {' \
    '    listen 8083;' \
    '    server_name localhost;' \
    '    root /usr/share/nginx/html;' \
    '    index index.html;' \
    '' \
    '    resolver 127.0.0.11 valid=10s;' \
    '    set $api http://api:8081;' \
    '' \
    '    location /api/v1/ {' \
    '        proxy_pass $api;' \
    '        proxy_set_header Host $host;' \
    '        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;' \
    '    }' \
    '' \
    '    location /assets/ {' \
    '        add_header Cache-Control "public, max-age=31536000, immutable";' \
    '    }' \
    '' \
    '    location / {' \
    '        try_files $uri $uri/ /index.html;' \
    '    }' \
    '}' \
    > /etc/nginx/conf.d/default.conf

EXPOSE 8083
