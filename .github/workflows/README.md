# CI/CD

Проверки отделены от выкладки намеренно: PR должен проверяться, но ничего
не выкладывать.

| Workflow | Когда | Что делает |
| --- | --- | --- |
| `ci.yml` | каждый PR и push в master | Go: формат, vet, сборка, тесты на Postgres. Фронт: типы, тесты, контраст, сборка. Лендинг: тесты. Живой api против контракта. Terraform: формат и валидация |
| `deploy-functions.yml` | push в master, затронувший Go-код (`apps/{bot,api,reminders,notifier}`, `packages/`, `go.mod`) | `goose up` на проде, затем новая версия каждой функции |
| `deploy-web.yml` | push в master, затронувший `apps/web/`, контракт или словарь текстов | сборка фронта под адрес API Gateway и заливка в бакет |
| `deploy-landing.yml` | push в master, затронувший `apps/landing/` | тесты лендинга и заливка в бакет `traektoriaedu.ru` (имя можно переопределить переменной `YC_LANDING_BUCKET`) |
| `infra-plan.yml` | PR, затронувший `infra/terraform/` | `tofu plan` комментарием в PR, ничего не применяет |
| `infra-apply.yml` | только вручную | `tofu apply` с обязательным подтверждением второго человека |

Все деплои можно запустить руками через **Actions → Run workflow**, не делая пустой коммит.

## Первый запуск бэкенда

Пока база и функции не подняты, **Deploy functions** честно падает на проверке
настроек и прод не трогает. Порядок один раз:

0. **Settings → Environments → `production-infra`**, в нём Required reviewers.
   Окружения с таким именем может не быть: GitHub тогда создаст его сам при
   первом запуске — но **без единого правила защиты**, и apply пройдёт без
   подтверждения. Ради чего и заводился гейт, в этом случае теряется.
1. Завести секреты и переменные из раздела «Настройка» (кроме `DATABASE_URL`
   и `YC_API_URL` — их ещё неоткуда взять).
2. Переменные `ENABLE_DATABASE=true`, `ENABLE_WORKERS=true`, `SSH_PUBLIC_KEY`
   и `SSH_ALLOWED_CIDRS` со своим адресом — Ansible пойдёт на ВМ оттуда.
3. **Actions → Infra apply**. Поднимает ВМ с базой, диск, адрес, Lockbox,
   функции с настройками, шлюз и таймеры. Код в новые функции кладёт Terraform.
4. Настроить саму СУБД — [infra/ansible](../../infra/ansible/README.md).
   До этого шага база не существует: ВМ есть, PostgreSQL на ней нет.
5. `tofu output -raw database_url` → секрет `DATABASE_URL`,
   `tofu output -raw api_url` → переменная `YC_API_URL`.
6. **Deploy functions → Run workflow**: миграции, код, проверки.
   **Deploy web → Run workflow**: мини-приложение с адресом api.
7. Вебхук MAX — один раз, командой из корневого README. Пока подписка есть,
   локальный long polling (`BOT_MODE=poll`) ничего не получает.
8. Закрыть SSH обратно: `SSH_ALLOWED_CIDRS` в `[]` и ещё раз **Infra apply**.

## Процесс изменения инфраструктуры

| Шаг | Кто |
| --- | --- |
| Правка `.tf`, открыть PR | автор |
| `infra-plan` постит план комментарием | автоматически |
| Ревью: смотреть строку `Plan:`, любое `destroy` — стоп | ревьюер |
| Мерж в master | автор |
| Actions → Infra apply → Run workflow | автор |
| Подтверждение окружения `production-infra` | второй человек |

**Автоматического `apply` при слиянии нет намеренно.** Выкладка кода обратима —
откатился на предыдущую версию функции. Выкладка инфраструктуры нет: удалённый
бакет уронит мини-приложение у организаторов, пересозданная функция сменит URL
вебхука и потребует перерегистрации подписки в MAX.

**В окне проверки 30.09-14.10 `infra-apply` не запускается вообще**, кроме аварии.
Перед 30.09 стоит прогнать план и убедиться, что он чистый.

## Сервисные аккаунты

Их два, с разными правами — чтобы компрометация ключа выкладки не давала
возможности тронуть инфраструктуру.

| Аккаунт | Роли | Для чего |
| --- | --- | --- |
| `traektoria-cicd` | `functions.editor`, `storage.editor` | выкладка кода и статики |
| `traektoria-infra` | `admin` | `tofu plan` и `apply` |

У `traektoria-infra` права широкие вынужденно: без `admin` не проходит ни
назначение ролей, ни публичный вызов функций. Защита здесь не в правах, а в
окружении `production-infra` с обязательным подтверждением.

## Настройка

Секреты и переменные заводятся один раз, в **Settings → Secrets and variables → Actions**.

### Переменные (вкладка Variables)

| Имя | Значение |
| --- | --- |
| `YC_FOLDER_ID` | `b1gsauq7gtvp76o9jil0` |
| `YC_WEB_BUCKET` | `traektoria` |
| `YC_LANDING_BUCKET` | необязательно, по умолчанию `traektoriaedu.ru` — бакет лендинга, имя совпадает с доменом |
| `YC_API_URL` | `tofu output -raw api_url` — адрес API Gateway, вшивается в сборку фронта и проверяется после выкладки api |
| `ENABLE_DATABASE` | `true`, когда нужна ВМ с PostgreSQL |
| `SSH_PUBLIC_KEY` | открытый ключ для пользователя `ubuntu` на ВМ с базой: им ходит Ansible. Обязателен при `ENABLE_DATABASE=true` — без него в ВМ будет не войти. Не секрет, поэтому переменная: видно, чей он |
| `SSH_ALLOWED_CIDRS` | откуда разрешён SSH к базе, JSON-списком: `["1.2.3.4/32"]`. Пусто — порт закрыт всем; открывать на время прогона Ansible и закрывать обратно |
| `ENABLE_WORKERS` | `true`, когда нужны api, воркеры, шлюз и таймеры |
| `ENABLE_DOMAIN_HTTPS` | `true`, когда сертификат домена `ISSUED` |
| `ENABLE_BROWSER_DEMO` | **Временно.** `true` — мини-приложение открывается в браузере без MAX от лица демо-семьи (`?dev_user=parent` — родитель). Включение и выключение: Infra apply → Deploy functions → Deploy web. Выключить до 30.09 |

Это не секреты: идентификаторы каталога и адреса ничего сами по себе не открывают.
`YC_BOT_FUNCTION_ID` больше не нужен: скрипт выкладки находит функцию по имени.

Флаги `ENABLE_*` описывают, каким должен быть прод, и одинаково читаются в
`infra-plan` и `infra-apply`. Галочками при запуске они не задаются намеренно:
забытая галочка `enable_database` удалила бы базу. Перед первым запуском
выставьте их по текущему состоянию — иначе план покажет `destroy`.

### Секреты (вкладка Secrets)

Сначала создать сервисный аккаунт CI:

```bash
cd infra/terraform
export YC_TOKEN=$(yc iam create-token)
tofu plan -out=tfplan && tofu apply tfplan
```

Дальше — значения секретов. Перед чтением обязательно выставить ключи доступа
к бакету с состоянием, иначе `tofu output` упадёт, а в пайп уйдёт пустая строка
и секрет молча запишется пустым:

```bash
export AWS_ACCESS_KEY_ID=... AWS_SECRET_ACCESS_KEY=...
```

Затем прочитать значения **по одному** и вставить в GitHub:

```bash
tofu output -raw github_secret_yc_sa_key
tofu output -raw github_secret_yc_storage_access_key
tofu output -raw github_secret_yc_storage_secret_key
```

| Секрет | Откуда |
| --- | --- |
| `YC_SA_KEY` | `github_secret_yc_sa_key` — JSON целиком, вместе со скобками |
| `YC_INFRA_KEY` | `github_secret_yc_infra_key` — JSON целиком |
| `YC_STORAGE_ACCESS_KEY` | `github_secret_yc_storage_access_key` |
| `YC_STORAGE_SECRET_KEY` | `github_secret_yc_storage_secret_key` |
| `MAX_BOT_TOKEN` | токен бота |
| `WEBHOOK_SECRET` | секрет вебхука, `[a-zA-Z0-9_-]{5,256}` |
| `JWT_SECRET` | `openssl rand -hex 32` — один раз; смена разлогинит всех |
| `POLZA_AI_API_KEY` | ключ polza.ai; без него помощник отвечает шаблоном «данных нет» |
| `PG_PASSWORD` | пароль пользователя БД. Тот же пароль получает Ansible — разойдутся, и функции не подключатся |
| `DATABASE_URL` | `tofu output -raw database_url` — только для миграций в Deploy functions |

Секреты приложения `infra-apply` передаёт в Terraform, а тот кладёт их в Lockbox.
Без них `apply` не запускается: пустое значение Terraform из Lockbox убрал бы,
и функции остались бы без токена.

Ключи хранилища нужны не только для заливки статики: ими бэкенд S3 ходит
в бакет с состоянием Terraform.

### Окружение

**Settings → Environments → New environment**, имя `production-infra`,
включить **Required reviewers** и добавить туда как минимум одного человека
кроме себя. Без этого `infra-apply` применит изменения без подтверждения —
то есть вся защита процесса исчезнет.

`tofu output` без `-raw` покажет все значения сразу и оставит их в истории
терминала — поэтому по одному.

## Кто владеет кодом функции

Terraform создаёт функцию и держит её настройки, но **код в неё выкладывает CI**.
В `functions.tf` на этот случай стоит `ignore_changes` на `user_hash` и `content`:
иначе каждый `tofu apply` откатывал бы функцию на ту версию, что собрана из
рабочей копии запускающего, затирая выложенное пайплайном.

`yc serverless function version create` ничего не наследует от прошлой версии:
версия без явных флагов вышла бы без секретов, и бот упал бы на старте. Поэтому
`.github/scripts/deploy_function.py` берёт окружение, ссылки на секреты Lockbox,
сервисный аккаунт, память и таймаут у текущей версии `$latest` и выкладывает с
ними новый код. Если у текущей версии нет нужных функции переменных — Terraform
ещё не применён, — скрипт падает до выкладки. Указывать сервисный аккаунт
`traektoria-functions` аккаунту CI разрешает роль `iam.serviceAccounts.user`
(`cicd.tf`).

Выкладывается новая **версия**, функция не пересоздаётся: её URL зарегистрирован
в MAX как вебхук и меняться не должен. Предыдущие версии остаются — это и есть
откат, переключением тега `$latest` без правки кода.

## Проверки после выкладки

Все деплои заканчиваются проверкой живости. Фронт должен отдать `200`. Лендинг —
отдать `index.html` с заголовком, `styles.css` как `text/css` и `main.js` как JavaScript:
скрипт подключён модулем, и с чужим типом браузер его не выполнит. Вебхук
получает запрос с заведомо чужим секретом и должен ответить `403`: это ответ
нашего обработчика, значит, версия поднялась с токеном, секретом и конфигом —
без них инициализация роняет инстанс и вызов отвечает `502`. В базу и в MAX
проба не ходит. api проверяется через шлюз: `GET <YC_API_URL>/health` → `200`.
Воркеры HTTP-пробы не имеют: их вызов и есть рассылка.

Если проверка не прошла, скрипт возвращает `$latest` на прежнюю версию и
workflow падает. Для вебхука это критично: MAX считает ошибкой доставки любой
код кроме 200 и через 8 часов неудач автоматически отписывает бота.

## Выкладка вручную, пока Actions не работают

GitHub может перестать запускать задачи: у приватного репозитория 2000
бесплатных минут в месяц, и когда они кончились, каждая задача «падает» за
пару секунд без единого шага с сообщением «The job was not started because
recent account payments have failed or your spending limit needs to be
increased». Минуты обнуляются в начале месяца. До этого `master` выкладывается
с машины разработчика теми же шагами, что делают workflow, — через `yc`, без
статических ключей.

Выкладка в прод — только по явной просьбе человека (AGENTS.md, раздел 6).
Команды — из корня репозитория, на свежем `master`, после локальных проверок
вместо CI: `make check && make test-db` и в `apps/web`
`npm run typecheck && npm test && npm run check:contrast && npm run build`.

### Один раз: yc

```bash
curl -sSL https://storage.yandexcloud.net/yandexcloud-yc/install.sh | bash
```

`yc` встаёт в `~/yandex-cloud/bin` и попадает в `PATH` только в новом шелле —
до этого вызывать по полному пути. Войти (`~/yandex-cloud/bin/yc init`) должен
человек: аккаунтом с доступом к каталогу `YC_FOLDER_ID`, выбрать этот каталог.
Агент проверяет, что вошли тем аккаунтом:

```bash
~/yandex-cloud/bin/yc storage bucket get "$(gh variable get YC_WEB_BUCKET)"
```

`Access Denied` — аккаунт не тот, `yc init` заново.

### Что выкладывать

Выложенные коммиты: у функций — `version` в
`curl -s "$(gh variable get YC_API_URL)/health"`, у мини-приложения — первые
7 знаков коммита, которые сборщик вшил константой в чанк профиля (там их
показывает строка «Версия: приложение …»):

```bash
W="https://$(gh variable get YC_WEB_BUCKET).website.yandexcloud.net"; P=$(curl -s "$W/$(curl -s "$W/" | grep -o 'assets/index-[^"]*\.js' | head -1)" | grep -oE 'Profile-[A-Za-z0-9_-]+\.js' | head -1); curl -s "$W/assets/$P" | grep -oE '=`[0-9a-f]{7}`' | head -1
```

Дальше `git diff --name-only <коммит> master` и пути из `on.push.paths`
каждого workflow: что затронуто, то и выкладывается. Функции — раньше
мини-приложения: фронт может ждать новых полей API.

### Функции: миграции — человек, код — агент

Миграции идут раньше кода. Им нужна строка подключения из Lockbox, а агенту
читать секреты незачем (автопроверка прав Claude Code такое и не пропускает).
Человек запускает у себя — строка подставляется внутри команды и на экран не
выводится:

```bash
go run github.com/pressly/goose/v3/cmd/goose@v3.22.1 -dir packages/db/migrations postgres "$(~/yandex-cloud/bin/yc lockbox payload get --name traektoria-app --key DATABASE_URL)" up
```

Пока `ENABLE_BROWSER_DEMO=true`, так же катится демо-семья, и её приглашение
отзывается — как в Deploy functions:

```bash
go run github.com/pressly/goose/v3/cmd/goose@v3.22.1 -table goose_demo_version -dir packages/db/migrations-demo postgres "$(~/yandex-cloud/bin/yc lockbox payload get --name traektoria-app --key DATABASE_URL)" up
```

```bash
psql "$(~/yandex-cloud/bin/yc lockbox payload get --name traektoria-app --key DATABASE_URL)" -c "UPDATE invites SET revoked_at = now() WHERE token = 'demoInviteParent01' AND revoked_at IS NULL"
```

Код — агент. Архив исходников тем же составом, что в workflow:

```bash
rm -rf build && mkdir build && rsync -a --exclude='.git' --exclude='.github' --exclude='.claude' --exclude='docs' --exclude='datasets' --exclude='dataset_c_src' --exclude='infra' --exclude='apps/web' --exclude='build' --exclude='olimpiady_spravochnik.json' --exclude='DATASET_C_NOTES.md' ./ build/ && git rev-parse --short=7 HEAD > build/packages/shared/version/commit.txt
```

Потом каждая функция тем же скриптом: он переносит настройки текущей версии,
проверяет живость и откатывает `$latest`, если проверка не прошла.

```bash
export PATH="$HOME/yandex-cloud/bin:$PATH" GITHUB_SHA="$(git rev-parse HEAD)" API_URL="$(gh variable get YC_API_URL)"
python3 .github/scripts/deploy_function.py --name traektoria-bot --require MAX_BOT_TOKEN,WEBHOOK_SECRET,DATABASE_URL --probe bot
python3 .github/scripts/deploy_function.py --name traektoria-api --require MAX_BOT_TOKEN,JWT_SECRET,DATABASE_URL,GODEBUG --probe api --api-url "$API_URL" --optional
python3 .github/scripts/deploy_function.py --name traektoria-reminders --require MAX_BOT_TOKEN,DATABASE_URL --optional
python3 .github/scripts/deploy_function.py --name traektoria-content-notifier --require MAX_BOT_TOKEN,DATABASE_URL --optional
```

Упала одна — остальные выкладывать можно: функции независимы, как в
`fail-fast: false` workflow. Упавшая уже откатила себя на прежнюю версию.

Итог — `version` в `/health` равна `git rev-parse --short=7 HEAD`. Каталог
`build/` после выкладки удалить.

### Мини-приложение — агент

Сборка с адресом API и демо-режимом, как в Deploy web
(`VITE_DEV_FAKE_WEBAPP` — по переменной `ENABLE_BROWSER_DEMO`):

```bash
cd apps/web && npm ci && VITE_API_BASE="$(gh variable get YC_API_URL)" VITE_APP_VERSION="$(git rev-parse HEAD)" VITE_DEV_FAKE_WEBAPP="$(gh variable get ENABLE_BROWSER_DEMO)" npm run build && cd ../..
```

Всё, кроме `index.html`, — с вечным кешем: у файлов в имени хеш содержимого.
Сначала тот же вызов с `--dryrun`: в списке `upload:` не должно быть `index.html`.

```bash
~/yandex-cloud/bin/yc storage s3 cp --recursive apps/web/dist/ "s3://$(gh variable get YC_WEB_BUCKET)/" --exclude "index.html" --cache-control "public, max-age=31536000, immutable"
```

`index.html` — последним и без кеша, иначе MAX продолжит открывать старую сборку:

```bash
~/yandex-cloud/bin/yc storage s3 cp apps/web/dist/index.html "s3://$(gh variable get YC_WEB_BUCKET)/index.html" --content-type "text/html; charset=utf-8" --cache-control "no-cache"
```

Итог — коммит из раздела «Что выкладывать» равен `git rev-parse --short=7 HEAD`, а
`curl -sI` страницы отдаёт `200` и `cache-control: no-cache`. В отличие от
`aws s3 sync --delete`, старые файлы в бакете остаются: сборка на них не
ссылается, удалять их не нужно.

### Лендинг — агент

```bash
node --test apps/landing/landing.test.mjs && ~/yandex-cloud/bin/yc storage s3 cp --recursive apps/landing/ "s3://$(gh variable get YC_LANDING_BUCKET 2>/dev/null || echo traektoriaedu.ru)/" --exclude "index.html" --exclude "*.test.mjs" --exclude "README.md" --cache-control "public, max-age=3600"
```

```bash
~/yandex-cloud/bin/yc storage s3 cp apps/landing/index.html "s3://$(gh variable get YC_LANDING_BUCKET 2>/dev/null || echo traektoriaedu.ru)/index.html" --content-type "text/html; charset=utf-8" --cache-control "no-cache"
```

Проверка — та же, что в Deploy landing: `index.html` с «Одна траектория»,
`styles.css` как `text/css`, `main.js` как JavaScript.

Когда Actions заработают, повторять ничего не нужно: следующее слияние в
`master` выложит всё обычным путём.
