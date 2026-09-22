# Инфраструктура «Траектории» в Yandex Cloud

Чек-лист развёртывания под архитектуру из раздела 8 ТЗ: статика мини-аппа в Object Storage,
бэкенд на Cloud Functions, напоминания на Timer-триггерах, БД — Managed PostgreSQL.

> **Инфраструктура теперь применяется через Terraform** — см. [terraform/README.md](terraform/README.md).
> Всё, что уже создано, импортировано в состояние. Команды `yc` ниже оставлены как справочник:
> они объясняют, что и зачем делается, и пригодятся при отладке, но **создавать ими новые ресурсы
> не нужно** — состояние разойдётся с реальностью. Новое описываем в `terraform/`.

Порядок важен: пункты 1-4 разблокируют фронтенд и регистрацию вебхука, всё остальное можно делать параллельно.

Команды, помеченные `[сверить]`, собраны по докам, но точный набор флагов стоит проверить
при первом запуске (`yc <команда> --help`).

---

## 0. Предварительно (руками, один раз)

- Создать облако, платёжный аккаунт (активировать грант хакатона, если дают) и каталог — в консоли.
- Поставить и авторизовать CLI:

```bash
curl -sSL https://storage.yandexcloud.net/yandexcloud-yc/install.sh | bash -s -- -a
exec -l $SHELL
yc init          # OAuth в браузере, выбор облака и каталога
yc config list   # проверить, что folder-id проставлен
```

Флаг `-a` дописывает PATH и автодополнение в `~/.zshrc` — без него инсталлятор спросит об этом
интерактивно, и `yc` не найдётся в новой сессии. Бинарник ложится в `~/yandex-cloud/bin/yc`.

Роли на аккаунте: `editor` на каталог (или точечно `functions.admin`, `storage.editor`, `iam.serviceAccounts.user`).

### Сделано (22.09), фактические значения

| Что | Значение |
| --- | --- |
| CLI | 1.36.0 darwin/arm64, `~/yandex-cloud/bin/yc` |
| Облако | `cloud-chinook-330` / `b1g3r44i678j8024470s` |
| Организация | `bpfnru3imurj435kijsc` |
| Каталог | `default` / `b1gsauq7gtvp76o9jil0` |
| Аккаунт | `ajet26kcc11bqhc14dli`, роли `editor` + `admin` на облако |

### Развёрнуто (22.09)

| Ресурс | Значение |
| --- | --- |
| `traektoria-bot` | `d4e4a4gqsiq7avbr2f0s` |
| Бакет `traektoria` | `e3et35iq0oc613nj66nm`, публичное чтение, хостинг сайта |
| `WEBAPP_URL` | `https://traektoria.website.yandexcloud.net/` — HTTPS проверен |
| URL вебхука | `https://functions.yandexcloud.net/d4e4a4gqsiq7avbr2f0s` |
| Версия | `golang123`, entrypoint `cmd/function/main.Handler`, 128 МБ, таймаут 10 с |
| Код | `apps/bot/cmd/function/main.go` — заглушка: любой апдейт → 200 с пустым телом |

Проверено вызовом с IAM-токеном: `200`, тело пустое, ~0,34 с.

Публичный вызов включён: проба без токена снаружи даёт `200`, тело пустое. Вебхук готов к регистрации в MAX.

`allow-unauthenticated-invoke` требует роли `admin` — с одним `editor` возвращает `PermissionDenied`.
Роль выдана 22.09, дальше пункты 1-6 выполняются без участия владельца облака.

Назначать роли можно: 22.09 владелец облака (`ajef4tsh6n3cgsdavaj4`) выдал на аккаунт `admin`
в дополнение к `editor`. Без `admin` не проходят ни `add-access-binding` из пункта 1,
ни `allow-unauthenticated-invoke` из пункта 3.

Токен после `yc init` лежит открытым в `~/.config/yandex-cloud/config.yaml` — файл не коммитить.

---

## 1. Сервисные аккаунты

```bash
# от имени которого работают функции (читает секреты из Lockbox, пишет логи)
yc iam service-account create --name traektoria-fn

# от имени которого таймеры вызывают функции
yc iam service-account create --name traektoria-invoker

FOLDER=$(yc config get folder-id)
FN_SA=$(yc iam service-account get traektoria-fn --format json | jq -r .id)
INV_SA=$(yc iam service-account get traektoria-invoker --format json | jq -r .id)

yc resource-manager folder add-access-binding $FOLDER \
  --role lockbox.payloadViewer --service-account-id $FN_SA
yc resource-manager folder add-access-binding $FOLDER \
  --role serverless.functions.invoker --service-account-id $INV_SA
```

---

## 2. Бакет со статикой мини-приложения

Выполнено 22.09, команды рабочие:

```bash
yc storage bucket create --name traektoria
yc storage bucket update --name traektoria --public-read \
  --website-settings '{"index":"index.html","error":"index.html"}'

# выкладка содержимого
yc storage s3 cp apps/web/index.html s3://traektoria/index.html \
  --content-type "text/html; charset=utf-8" --cache-control "no-cache"
```

Публичное чтение задаётся флагом `--public-read` у `bucket update`, а не `--acl public-read` при
создании. Настройки сайта принимаются инлайном через `--website-settings`, временный файл не нужен.
Загрузка — через `yc storage s3 cp` (группы `object upload` в CLI нет), `--content-type` указываем
явно, иначе есть риск получить неверный MIME.

Адрес сайта: **`https://traektoria.website.yandexcloud.net/`** — это `WEBAPP_URL`.
Его же передаём организаторам через форму привязки мини-приложения.

**HTTPS проверен 22.09:** `200`, `ssl_verify=0`, сертификат GlobalSign, действителен до 16.11.2026 —
окно проверки 30.09-14.10 покрывает. Браузер открывает без предупреждений. Домен и Let's Encrypt
не нужны: работает wildcard-сертификат Yandex Cloud.

Условие для HTTPS — **в имени бакета не должно быть точек**: wildcard покрывает только один уровень
поддомена. Поэтому собственный домен здесь не «привязывается через DNS»: имя бакета должно полностью
совпадать с доменом (`traektoria.ru`), а такой бакет уже требует своего сертификата из Certificate
Manager. Переезд на домен = новый бакет + повторная отправка формы организаторам.

`error` указан как `index.html` намеренно: у мини-аппа клиентский роутинг, все пути отдаём в SPA.
Ключ индексного документа не может содержать `/`.

Нюанс: на несуществующий путь отдаётся тело `index.html`, но со статусом **404**, а не 200.
Для клиентского роутинга это безразлично — браузер отрисует страницу и роутер разберётся сам.

---

## 3. Функции

Создаём все четыре сразу, пусть с заглушками — URL присваивается в момент создания
и больше не меняется, так что вебхук и фронт можно настраивать, пока логики ещё нет.

```bash
for f in traektoria-bot traektoria-api traektoria-reminders traektoria-content-notifier; do
  yc serverless function create --name $f
done

# бот и api должны быть вызываемы без IAM-авторизации: MAX и браузер ходят к ним снаружи
yc serverless function allow-unauthenticated-invoke traektoria-bot
yc serverless function allow-unauthenticated-invoke traektoria-api

# воркеры наружу не смотрят — их дёргает только таймер, публичный доступ им не нужен

for f in traektoria-bot traektoria-api; do
  echo "$f: https://functions.yandexcloud.net/$(yc serverless function get $f --format json | jq -r .id)"
done
```

Первый URL идёт в `POST /subscriptions` MAX как `WEBHOOK_URL`, второй — во фронт как `API_BASE_URL`.

Выкладка версии (проверено 22.09):

```bash
cd apps/bot
yc serverless function version create \
  --function-name traektoria-bot \
  --runtime golang123 \
  --entrypoint cmd/function/main.Handler \
  --memory 256m \
  --execution-timeout 30s \
  --service-account-id $FN_SA \
  --source-path . \
  --environment MAX_API_BASE=https://platform-api2.max.ru
```

Go-рантаймы в каталоге: `golang121`, `golang123` (полный список — `yc serverless function runtime list`).

**Формат entrypoint** — `<путь к каталогу>/<имя пакета>.<Функция>`, а не `<путь к пакету>.<Функция>`.
Для `apps/bot/cmd/function/main.go` с `package main` это `cmd/function/main.Handler`.
Вариант `cmd/function.Handler` падает на сборке: `./cmd is not a package in module rooted at ...`.

`--source-path` принимает каталог, архив собирать не нужно. `go.mod` должен лежать в корне
того, что передаём (`apps/bot/go.mod`), и объявлять версию не выше рантайма.
В файле с обработчиком **не должно быть `func main()`** — рантайм генерирует свой.
Архив до 3,5 МБ грузится напрямую, до 128 МБ — только через Object Storage
(`--package-bucket-name` / `--package-object-name`).

---

## 4. Таймер-триггеры

Cron у Yandex Cloud — **шесть полей**: `Минуты Часы День-месяца Месяц День-недели Год`,
время в **UTC**. `?` = «любое значение», ставится в поле дня месяца или дня недели.

```bash
R_ID=$(yc serverless function get traektoria-reminders --format json | jq -r .id)
N_ID=$(yc serverless function get traektoria-content-notifier --format json | jq -r .id)

# напоминания: раз в 15 минут, сам воркер решает, кому уже пора
yc serverless trigger create timer \
  --name traektoria-reminders \
  --cron-expression '0/15 * ? * * *' \
  --invoke-function-id $R_ID \
  --invoke-function-service-account-id $INV_SA \
  --retry-attempts 3 --retry-interval 30s

# уведомления об изменениях контента: раз в сутки в 10:00 МСК = 07:00 UTC
yc serverless trigger create timer \
  --name traektoria-content-notifier \
  --cron-expression '0 7 ? * * *' \
  --invoke-function-id $N_ID \
  --invoke-function-service-account-id $INV_SA
```

Не перепутать часовой пояс: `REMINDER_HOUR=10` в коде — местное время ученика,
а cron здесь — UTC.

---

## 5. Managed PostgreSQL

MCP-серверами Yandex Cloud не покрывается, поднимать через консоль или CLI `[сверить]`.
Минимальная конфигурация, PostgreSQL 16, публичный доступ к хосту + `sslmode=require`
(так функции достают БД без привязки к VPC и без NAT-шлюза).

Один хост дешевле; два снимают риск техработ на окне проверки 30.09-14.10,
когда решение должно быть доступно непрерывно. Если грант позволяет — брать два.

Строка подключения кладётся в Lockbox как `DATABASE_URL`.

---

## 6. Секреты

```bash
yc lockbox secret create --name traektoria \
  --payload '[{"key":"MAX_BOT_TOKEN","text_value":"..."},
              {"key":"JWT_SECRET","text_value":"..."},
              {"key":"WEBHOOK_SECRET","text_value":"..."},
              {"key":"DATABASE_URL","text_value":"..."},
              {"key":"GIGACHAT_AUTH_KEY","text_value":"..."}]'
```

Функции читают секреты под `traektoria-fn` (роль выдана в пункте 1).
В код и в git ничего из этого не попадает — это прямо 10% технической оценки.

---

## 7. Остаётся вне Yandex Cloud

- **Регистрация вебхука в MAX** — `POST /subscriptions` с `WEBHOOK_URL` из пункта 3 и `WEBHOOK_SECRET`.
- **URL мини-приложения организаторам** — через форму привязки, адрес из пункта 2.
- **Ключ GigaChat API** — регистрируется отдельно в личном кабинете Сбера, бывает ожидание.
  Без него не поедет ИИ-помощник (F35-F37).
- **Заявка на квоту** — по умолчанию 10 одновременных вызовов функций на зону доступности.
  Для демо хватает; если планируется нагрузочная проверка, подавать заранее.
