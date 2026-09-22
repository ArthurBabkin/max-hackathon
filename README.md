# Траектория

Навигатор по олимпиадам и поступлению: чат-бот и мини-приложение в MAX.
Трек «Образовательные решения», студенческий хакатон MAX.

Бот: [@t356_hakaton_max_bot](https://max.ru/t356_hakaton_max_bot)
Мини-приложение: https://traektoria.website.yandexcloud.net/

## Структура

| Папка | Что внутри |
| --- | --- |
| `apps/bot` | Go: вебхук MAX. `cmd/function` — точка входа Cloud Functions |
| `apps/web` | мини-приложение: React + Vite + MAX UI |
| `docs` | ТЗ, прототип, экраны, продуктовые материалы, исходники хакатона |
| `datasets` | датасеты олимпиад и вузов, парсеры, спецификация формата |
| `infra` | развёртывание: Terraform и чек-лист по Yandex Cloud |

| `packages/api-contract` | OpenAPI-контракт мини-приложения и сгенерированные TS-типы |
| `packages/shared` | общие Go-пакеты и словарь текстов kid/parent |
| `packages/db` | SQL-миграции |

## С чего начать

Техническое задание — [docs/TZ.md](docs/TZ.md), архитектура в разделе 8, структура репозитория в 13.1.
Что именно требуется от решения по условиям хакатона — [docs/hackathon/summary.md](docs/hackathon/summary.md).

## Развёртывание

Инфраструктура описана в Terraform: [infra/terraform/README.md](infra/terraform/README.md).

```bash
cd infra/terraform
export YC_TOKEN=$(yc iam create-token)
tofu plan -out=tfplan && tofu apply tfplan
```

Тот же `apply` выкладывает и код бота. Статика сайта заливается отдельно:

```bash
npm --prefix apps/web ci && npm --prefix apps/web run build
yc storage s3 cp apps/web/dist/ s3://traektoria/ --recursive
```

Заливается именно `dist/`: в `apps/web/` лежат исходники, и выкладывать
их в публичный бакет незачем.

## Окружение

Переменные — в [.env.example](.env.example). Секреты в git не попадают:
токен бота, ключ GigaChat и строка подключения к БД живут в Lockbox и в локальном `.env`.
