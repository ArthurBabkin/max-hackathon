# Инфраструктура в Terraform (OpenTofu)

Вся инфраструктура «Траектории» в Yandex Cloud описана здесь и применяется одной командой.
Ресурсы, созданные раньше руками через `yc`, импортированы в состояние — ничего не пересоздавалось.

Исторические заметки о том, что выяснилось при ручной настройке (формат entrypoint,
какие роли нужны, почему HTTPS работает без домена), остались в [../README.md](../README.md).

---

## Что нужно один раз

```bash
brew install opentofu
yc init                       # если CLI ещё не настроен
```

Роли на аккаунте: **`admin`** на каталог или облако. Одного `editor` не хватает —
на нём падает и `yandex_function_iam_binding`, и выдача ролей сервисным аккаунтам.

---

## Как применять

```bash
cd infra/terraform
export YC_TOKEN=$(yc iam create-token)   # токен живёт 12 часов
export AWS_ACCESS_KEY_ID=...             # статические ключи traektoria-storage
export AWS_SECRET_ACCESS_KEY=...
tofu init
tofu plan -out=tfplan
tofu apply tfplan
```

**Ключи `AWS_*` обязательны для любой команды `tofu`**, включая `output` и `state list`:
состояние лежит в бакете, и бэкенд S3 ходит туда именно ими. Без них команда падает
с `No valid credential sources found` — и это легко не заметить, если пайпить вывод
в другую команду: `tofu` пишет ошибку в stderr, а в пайп уходит пусто.

**Всегда через `plan -out` и `apply <файл>`, а не `apply -auto-approve`.** Смысл в том,
что применяется ровно то, что вы прочитали: между отдельными `plan` и `apply` состояние
может измениться, и `apply` сделает не то, что показывал план.

Первым делом в выводе `plan` смотреть на строку `Plan: N to add, M to change, K to destroy`.
**Любой `destroy` — повод остановиться.** Удаление бакета `traektoria` уронит мини-приложение
у организаторов, удаление функции сменит URL вебхука, и подписку в MAX придётся регистрировать заново.

---

## Что описано

| Файл | Ресурсы |
| --- | --- |
| `functions.tf` | функции `bot`, `api`, `reminders`, `content-notifier` из одного zip; публичный вызов только у `bot` |
| `apigateway.tf` | API Gateway перед `api`: прямой вызов функции не передаёт путь запроса (флаг `enable_workers`) |
| `secrets.tf` | Lockbox с токенами и строкой подключения, сервисный аккаунт функций |
| `postgres.tf` | ВМ с PostgreSQL 16, диск под данные, адрес, группа безопасности, бакет бэкапов (флаг `enable_database`). Настройку СУБД катит [Ansible](../ansible/README.md) |
| `cicd.tf` | сервисные аккаунты GitHub Actions: `traektoria-cicd` выкладывает код, `traektoria-infra` применяет Terraform |
| `triggers.tf` | таймеры: напоминания раз в 15 минут, изменения контента раз в сутки (флаг `enable_workers`) |
| `storage.tf` | сервисный аккаунт для S3, оба бакета, хостинг сайта |
| `dns.tf` | зона `traektoriaedu.ru`, запись апекса, запись проверки владения |
| `certificate.tf` | сертификат Let's Encrypt |
| `outputs.tf` | адреса вебхука, api, мини-аппа и лендинга |

Значения по умолчанию (облако, каталог, домен) лежат в `variables.tf` — это не секреты,
репозиторий приватный. Токен в конфигурацию не попадает и берётся только из `YC_TOKEN`.

### Код функций выкладывает пайплайн

Terraform создаёт функции и держит их настройки: окружение, секреты Lockbox,
сервисный аккаунт, память, таймаут. Код при создании функции берётся из zip
рабочей копии (`archive_file`), дальше его выкладывает **Deploy functions** при
слиянии в `master`: `ignore_changes` на `user_hash` и `content` не даёт `apply`
откатить выложенное. Подробно — [../../.github/workflows/README.md](../../.github/workflows/README.md).

### Содержимое сайта — не через Terraform

Статика в бакетах намеренно не описана: это артефакт сборки, а не инфраструктура.
Выкладывает её **Deploy web**: собранный `apps/web/dist/` с адресом API Gateway,
не исходники.

Адрес при этом не меняется, поэтому перезаливать можно сколько угодно, в том числе
после того, как организаторы пропишут URL боту.

---

## Состояние

Состояние лежит в бакете `traektoria-tfstate` (приватный, версионирование включено),
а не на чьём-то ноутбуке: применять должен уметь любой в команде.

Бакет Terraform'ом не управляется намеренно — иначе он попытался бы удалить
хранилище собственного состояния. Создаётся один раз:

```bash
yc storage bucket create --name traektoria-tfstate
yc storage bucket update --name traektoria-tfstate --versioning versioning-enabled
```

Версионирование — страховка: если состояние побьётся, предыдущая версия объекта
остаётся в бакете и откатывается средствами Object Storage.

Доступ к бакету идёт по статическим ключам сервисного аккаунта:

```bash
export AWS_ACCESS_KEY_ID=... AWS_SECRET_ACCESS_KEY=...
```

**Блокировка работает** — проверено 22.09. `use_lockfile` кладёт рядом с состоянием
объект `terraform.tfstate.tflock`; при попытке взять занятую блокировку Object
Storage отвечает `412 Precondition Failed`, и вторая операция останавливается.
DynamoDB, на которую рассчитан классический вариант блокировки, в Yandex Cloud нет.

Если прогон упал и оставил блокировку висеть, снять её можно только осознанно:

```bash
tofu force-unlock <ID из сообщения об ошибке>
```

Делать это, лишь убедившись, что никакая операция реально не идёт, — иначе
два одновременных `apply` побьют состояние.

В состоянии открытым текстом лежат ключи сервисных аккаунтов, поэтому бакет
приватный, а локальные копии состояния в git не попадают.

## Включение HTTPS на своём домене

Сертификат выпускается автоматически, но только после того, как домен делегирован
на `ns1.yandexcloud.net` / `ns2.yandexcloud.net` и делегирование разошлось.

```bash
tofu output certificate_status        # ждём ISSUED
```

Пока статус `VALIDATING` — ждём. Если стал `INVALID`, значит Certificate Manager исчерпал
попытки: сертификат пересоздаётся (`tofu taint yandex_cm_certificate.domain` и `apply`),
запись проверки обновится сама.

Как только `ISSUED`:

```bash
tofu apply -var enable_domain_https=true
```

Флаг вынесен в переменную намеренно: привязка сертификата, которого ещё нет, роняет `apply`.

---

## Выкатка бэкенда

Секреты передаются только переменными `TF_VAR_*` и уходят в Lockbox (`secrets.tf`),
а не в переменные окружения версии функции. Функции читают их от имени сервисного
аккаунта `traektoria-functions` с ролью `lockbox.payloadViewer`.

Из пайплайна — **Actions → Infra apply**: секреты берутся из секретов
репозитория, флаги — из переменных `ENABLE_*`. Руками то же самое:

```bash
set -a; . ../../.env; set +a                        # MAX_BOT_TOKEN, POLZA_AI_API_KEY, WEBHOOK_SECRET
export TF_VAR_max_bot_token="$MAX_BOT_TOKEN"
export TF_VAR_polza_ai_api_key="$POLZA_AI_API_KEY"
export TF_VAR_webhook_secret="$WEBHOOK_SECRET"
export TF_VAR_jwt_secret="$(openssl rand -hex 32)"  # один раз; сохранить — смена разлогинит всех
export TF_VAR_pg_password='…'                      # пароль пользователя БД
tofu plan -out=tfplan -var enable_database=true -var enable_workers=true
tofu apply tfplan
```

Что получает каждая функция:

| Функция | Окружение | Секреты из Lockbox |
| --- | --- | --- |
| `bot` | `APP_ENV=production`, ник и id бота, `REMINDER_HOUR` | `MAX_BOT_TOKEN`, `WEBHOOK_SECRET`, `DATABASE_URL` |
| `api` | то же + `CORS_ALLOWED_ORIGINS`, `LLM_BASE_URL`, `LLM_MODEL` | `MAX_BOT_TOKEN`, `JWT_SECRET`, `POLZA_AI_API_KEY`, `DATABASE_URL` |
| `reminders`, `notifier` | то же, что у `bot` | `MAX_BOT_TOKEN`, `DATABASE_URL` |

`APP_ENV=production` выключает дев-обход подписи initData: api с
`DEV_UNSIGNED_INITDATA=true` в проде не стартует. Функция бота не стартует
без `WEBHOOK_SECRET`: её адрес публичный, и без секрета обновление от имени
любого пользователя мог бы прислать кто угодно.

Схему и контент катит **Deploy functions** перед выкладкой кода (секрет
`DATABASE_URL`). Руками, из корня репозитория:

```bash
DATABASE_URL="$(cd infra/terraform && tofu output -raw database_url)"
go run github.com/pressly/goose/v3/cmd/goose@v3.22.1 -dir packages/db/migrations postgres "$DATABASE_URL" up
```

Демо-миграции (`packages/db/migrations-demo`, демо-семья) катятся в прод только при `ENABLE_BROWSER_DEMO=true` — это делает Deploy functions. Вымышленные олимпиады (`packages/db/migrations-local`) в прод не катятся никогда.

Вебхук и подсказки команд — один раз, когда функция бота с новым кодом выкачена:

```bash
go run ./apps/bot/cmd/setup -webhook "$(cd infra/terraform && tofu output -raw webhook_url)"
```

Пока подписка есть, long polling (`BOT_MODE=poll`) ничего не получает: для локальной
отладки её снимают флагом `-unsubscribe <адрес>`.

Мини-приложению нужен адрес api на сборке — это адрес API Gateway, а не функции:
Cloud Functions при прямом вызове не передаёт путь запроса («Cloud Functions не
поддерживает пути в запросах. Для корректной работы http.ServeMux функцию нужно
вызывать через API-шлюз», документация Go-рантайма).

Пайплайн берёт его из переменной `YC_API_URL`. Руками:

```bash
VITE_API_BASE="$(cd infra/terraform && tofu output -raw api_url)" npm --prefix apps/web run build
yc storage s3 cp apps/web/dist/ s3://traektoria/ --recursive
```
