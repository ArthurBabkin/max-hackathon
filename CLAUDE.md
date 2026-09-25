# Траектория — правила для Claude Code

Бот и мини-приложение в MAX: подбор олимпиад и трекер сроков. Подробно — `README.md`, ТЗ — `docs/TZ.md`.

## Текущая работа

**Онбординг v2** — `docs/onboarding-v2/`. Начинать с `docs/onboarding-v2/README.md`:
задачи в `TASKS.md`, поведение и тексты в `SPEC.md`, причины решений в `DECISIONS.md`.
SPEC — источник правды; если он противоречит коду или неясен — спросить, не придумывать.

## Задачи — доска «Траектория — задачи»

Над репо работают несколько человек, у каждого свой агент. Чтобы не делать одно и то же дважды, все задачи
лежат на доске https://github.com/users/kvllad/projects/4 (Todo → In Progress → Done); задача — issue в
`ArthurBabkin/max-hackathon`. **Сверяться с доской обязательно перед любой задачей, которая меняет код**,
даже маленькой и даже если пользователь уверен, что её никто не делает. Нужен `gh` со скоупом `project`
(`gh auth refresh -s project`); без него — остановиться и попросить пользователя его выдать.

Константы доски для команд ниже:

```
PROJECT=PVT_kwHOAwW7q84BkslU   STATUS=PVTSSF_lAHOAwW7q84BkslUzhjcQ7o
TODO=f75ad846   IN_PROGRESS=47fc9ee4   DONE=98236657
```

1. **Сверка.** Незакрытые задачи доски и поиск по словам задачи среди issues и PR (и закрытых — вдруг уже сделано):
   ```
   gh project item-list 4 --owner kvllad --format json --limit 200 \
     --jq '.items[] | select(.status != "Done") | "\(.status) | #\(.content.number) \(.title) | \(.assignees // [] | join(",")) | \(.content.url)"'
   gh issue list --state all --search "<слова>";  gh pr list --state all --search "<слова>"
   ```
2. **Задача уже In Progress** или по ней открыт PR — не начинать. Показать пользователю, кто её делает
   (исполнитель, ссылка), и ждать решения. Пересекается частично (те же файлы, экран, миграции) — тоже сказать.
3. **Взять.** Нет issue — завести: `gh issue create --title "…" --body "…" --assignee @me`.
   Есть и свободна — `gh issue edit <N> --add-assignee @me`. Затем на доску и в In Progress
   (`item-add` для уже добавленной задачи просто вернёт её id):
   ```
   ID=$(gh project item-add 4 --owner kvllad --url <ссылка на issue> --format json --jq .id)
   gh project item-edit --project-id $PROJECT --id $ID --field-id $STATUS --single-select-option-id $IN_PROGRESS
   ```
   И комментарий в issue: «Беру, ветка `<ветка>`».
4. **PR** — в теле `Closes #<N>`. После слияния issue закроется, доска сама переведёт её в Done; проверить.
5. **Бросил или отдал** — статус Todo, снять себя с исполнителей, в комментарии — что успел и где ветка.
6. **Нашёл по ходу отдельную задачу** — не делать молча: завести issue в Todo и сказать пользователю.

Брать задачу и двигать её по доске можно без отдельной просьбы: это не пуш и не PR.

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
