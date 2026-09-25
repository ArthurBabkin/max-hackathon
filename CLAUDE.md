# Траектория — правила для Claude Code

Бот и мини-приложение в MAX: подбор олимпиад и трекер сроков. Подробно — `README.md`, ТЗ — `docs/TZ.md`.

## Текущая работа

**Онбординг v2** — `docs/onboarding-v2/`. Начинать с `docs/onboarding-v2/README.md`:
задачи в `TASKS.md`, поведение и тексты в `SPEC.md`, причины решений в `DECISIONS.md`.
SPEC — источник правды; если он противоречит коду или неясен — спросить, не придумывать.

## Сроки

Код сливается и выкатывается до 29.09.2026 вечером. С 30.09 по 14.10 идёт проверка,
код менять нельзя. Не начинать рефакторинги, которые не нужны задаче.

## Устройство

- Go 1.23 (рантайм Yandex Cloud Functions — golang123; не использовать stdlib 1.24+). Один модуль.
- `apps/{bot,api,reminders,notifier}` — четыре функции; `apps/web` — мини-приложение (React + Vite); `apps/landing`.
- `packages/core/*` — чистая логика (`match` — скоринг, `pick` — подбор поверх store, `refdata` — регионы, `voice` — голоса),
  `packages/db` — миграции goose и store, `packages/shared` — `texts`, `maxapi`, `jev`, `llm`, `config`.
- `datasets/` — датасеты A/B/C и парсер; сид контента — `make seed`.

## Правила

- Все тексты пользователю — только в `packages/shared/texts/texts.json`, два голоса: `kid` (на «ты») и `parent`
  (на «вы», о ребёнке по имени: `{student}`, `{student_gen}`, `{student_dat}`). Формулировки нейтральны по роду.
- Миграции только добавлять (`packages/db/migrations/00NN_*.sql`), старые не править; у каждой рабочий `-- +goose Down`.
  Устаревшие колонки оставлять на время раскатки с `COMMENT … 'Устарела'` (как в `0014`).
- Шаги онбординга — условия перехода; кнопка из старого сообщения ничего не меняет (`store.ErrStale`).
- Скоринг детерминированный, без LLM.
- Комментарии и сообщения коммитов — по-русски, в стиле существующих (`feat(bot): …`, `fix(api): …`).
- Не коммитить неотслеживаемые файлы корня: `HANDOFF.md`, `DATASET_C_NOTES.md`, `olimpiady_spravochnik.json`,
  `build/`, `dataset_c_src/`, `apps/bot/.maxemu/`.

## Команды

- `make check` — формат, vet, сборка, юнит-тесты (как CI).
- `make test-db` — все тесты с Postgres (поднимает контейнер).
- `make bot-poll` — бот в живом MAX через long polling; `apps/bot/scripts/dev-bot.sh` — событие боту от имени пользователя.
- `make web-dev` — мини-приложение на живом API.

## Git

Ветка на задачу от свежего `origin/master`: `feat/onb2-<N>-<slug>`. PR в `master`, CI должен быть зелёным.
Пушить и открывать PR — только когда попросят.
