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
tofu init
tofu plan -out=tfplan
tofu apply tfplan
```

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
| `functions.tf` | функция `traektoria-bot`, публичный вызов без IAM |
| `storage.tf` | сервисный аккаунт для S3, оба бакета, хостинг сайта |
| `dns.tf` | зона `traektoriaedu.ru`, запись апекса, запись проверки владения |
| `certificate.tf` | сертификат Let's Encrypt |
| `outputs.tf` | адреса вебхука, мини-аппа и лендинга |

Значения по умолчанию (облако, каталог, домен) лежат в `variables.tf` — это не секреты,
репозиторий приватный. Токен в конфигурацию не попадает и берётся только из `YC_TOKEN`.

### Код функции выкладывается тем же `apply`

`functions.tf` упаковывает `apps/bot` в zip через `archive_file` и подставляет хеш в `user_hash`.
Поэтому отдельного шага деплоя нет: поменяли Go-код — `tofu apply` создаст новую версию функции.
Если хеш не изменился, новая версия не создаётся.

### Содержимое сайта — не через Terraform

Статика в бакетах намеренно не описана: это артефакт сборки, а не инфраструктура.
Выкладка:

```bash
yc storage s3 cp apps/web/ s3://traektoria/ --recursive
```

Адрес при этом не меняется, поэтому перезаливать можно сколько угодно, в том числе
после того, как организаторы пропишут URL боту.

---

## Состояние

Состояние лежит локально в `terraform.tfstate` и **в git не попадает**: в нём в открытом виде
лежит секретный ключ сервисного аккаунта.

Отсюда следствие, которое надо понимать: применять умеет только тот, у кого есть файл состояния.
Для хакатона это приемлемо — инфраструктура создаётся один раз и с 30.09 замораживается.
Если состояние потеряется, оно восстанавливается импортом, ничего не пересоздавая:

```bash
tofu import 'yandex_function.bot' d4e4a4gqsiq7avbr2f0s
tofu import 'yandex_dns_zone.domain' dns0cbh4jf7o48b27v3v
tofu import 'yandex_cm_certificate.domain' fpqr801q2j527240mpkt
tofu import 'yandex_dns_recordset.apex' 'dns0cbh4jf7o48b27v3v/traektoriaedu.ru./ANAME'
tofu import 'yandex_dns_recordset.acme' 'dns0cbh4jf7o48b27v3v/_acme-challenge.traektoriaedu.ru./CNAME'
```

Бакеты импортируются последними и требуют ключей S3 в окружении, потому что при импорте
конфигурация не вычисляется и ссылку на ключ взять неоткуда:

```bash
export YC_STORAGE_ACCESS_KEY=... YC_STORAGE_SECRET_KEY=...
tofu import 'yandex_storage_bucket.fallback' traektoria
tofu import 'yandex_storage_bucket.domain' traektoriaedu.ru
```

Ключ можно посмотреть в консоли в сервисном аккаунте `traektoria-storage` или выпустить новый.
При обычных `plan` и `apply` эти переменные не нужны: там ключ подставляется из ссылки на ресурс.

`yandex_function_iam_binding` импорту не поддаётся — он просто переутверждается при `apply`,
это идемпотентно.

---

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

## Чего здесь пока нет

Managed PostgreSQL, Lockbox, Timer-триггеры и функции `api`, `reminders`, `content-notifier`
не описаны, потому что ещё не созданы. Порядок и команды — в [../README.md](../README.md);
при создании их надо заводить сразу здесь, а не через `yc`, иначе состояние разойдётся
с реальностью и следующий `plan` покажет расхождения.
