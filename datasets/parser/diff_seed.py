#!/usr/bin/env python3
"""Миграция-разница сидов: льготы и направления из 0003 и 0021 в том виде,
в каком они в `--base` (обычно origin/master, то есть на проде), -> в рабочем
дереве после `make seed`. Накатанной базе нужна именно разница: 0003 и 0021
уже применены, повторно goose их не выполнит.

    python3 datasets/parser/diff_seed.py --base origin/master --note note.txt \\
        --touched packages/db/migrations/0017_innopolis_lost_benefits.sql \\
                  packages/db/migrations/0018_msu_merged_cells.sql \\
        -o packages/db/migrations/0022_benefit_linking.sql

`--tables stages` — сроки этапов своей миграцией (0023).

Вузы (список направлений) только обновляются. Направления 0021: группы
направлений онбординга обновляются, новые вставляются, а направление, которого
в сиде больше нет, удаляется, только если его не выбрал ни один ученик.

`--touched` — миграции между сидом и этой, что сами правят льготы: на чистой
базе они идут поверх новой 0003, поэтому их строки приводятся к сиду всегда.
Down возвращает состояние `--base`. Эквивалентность проверяет make test-db.
"""
import argparse
import re
import subprocess
import sys
from dataclasses import dataclass, field
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SEEDS = ["packages/db/migrations/0003_seed_content.sql",
         "packages/db/migrations/0021_university_directions_content.sql"]
# Таблица -> сколько первых колонок составляют ключ.
KEYS = {"sources": 1, "universities": 1, "benefits": 1, "university_directions": 2, "direction_benefits": 4,
        "stages": 1}
DIR_GROUPS = "UPDATE directions d SET groups = v.groups"
DIR_INSERT = "INSERT INTO directions (id, name, subject_codes, groups, onboarding) VALUES"


def up_section(sql: str) -> str:
    return sql.split("-- +goose Down")[0]


def fields(row: str) -> list[str]:
    """Поля строки VALUES верхнего уровня: запятые внутри кавычек и скобок не делят."""
    body, out, cur, depth, quoted, i = row.strip()[1:-1], [], "", 0, False, 0
    while i < len(body):
        ch = body[i]
        if quoted:
            cur += ch
            if ch == "'":
                if body[i + 1:i + 2] == "'":
                    cur += "'"
                    i += 1
                else:
                    quoted = False
        elif ch == "'":
            quoted, cur = True, cur + ch
        elif ch in "([":
            depth, cur = depth + 1, cur + ch
        elif ch in ")]":
            depth, cur = depth - 1, cur + ch
        elif ch == "," and depth == 0:
            out.append(cur.strip())
            cur = ""
        else:
            cur += ch
        i += 1
    out.append(cur.strip())
    return out


def _unquote(v: str) -> str:
    return v[1:-1].replace("''", "'") if v.startswith("'") else v


def _key(row: str, n: int):
    ks = [_unquote(f) for f in fields(row)[:n]]
    return ks[0] if n == 1 else tuple(ks)


@dataclass
class Block:
    prefix: str = ""
    suffix: str = ""
    cols: list[str] = field(default_factory=list)
    rows: dict = field(default_factory=dict)


def blocks(sql: str, table: str) -> Block:
    """Все INSERT INTO table секции Up: строки по ключу, а заголовок и хвост
    первого такого INSERT — чтобы вывести строки тем же оператором."""
    lines = up_section(sql).split("\n")
    out = Block()
    i = 0
    while i < len(lines):
        m = re.match(rf"INSERT INTO {table} \(([^)]*)\)", lines[i])
        if not m:
            i += 1
            continue
        start = i
        while i < len(lines) and not lines[i].startswith("  ("):
            i += 1
        head = lines[start:i]
        while i < len(lines) and lines[i].startswith("  ("):
            row = re.sub(r"[,;]$", "", lines[i].strip())
            out.rows[_key(row, KEYS[table])] = row
            i += 1
        tail = []
        while i < len(lines):
            tail.append(lines[i])
            i += 1
            if tail[-1].rstrip().endswith(";"):
                break
        if not out.prefix:
            out.prefix, out.suffix = "\n".join(head), "\n".join(tail)
            out.cols = [c.strip() for c in m.group(1).split(",")]
    return out


def rows(sql: str, table: str) -> dict:
    return blocks(sql, table).rows


@dataclass
class Diff:
    removed: list
    upsert: dict


def diff(old: dict, new: dict, touched=frozenset()) -> Diff:
    removed = sorted(k for k in set(old) | set(touched) if k not in new)
    return Diff(removed, {k: v for k, v in new.items() if old.get(k) != v or k in touched})


def touched_keys(sql: str, table: str) -> set:
    """Ключи строк, которые Up миграции вставляет или удаляет."""
    keys = set(rows(sql, table))
    for m in re.finditer(rf"DELETE FROM {table} WHERE id IN \(([^;]*?)\);", up_section(sql), re.S):
        keys |= {_unquote(x.strip()) for x in m.group(1).split(",") if x.strip()}
    return keys


def _merged(files: list[str], table: str) -> Block:
    out = Block()
    for sql in files:
        b = blocks(sql, table)
        if not out.prefix and b.prefix:
            out.prefix, out.suffix, out.cols = b.prefix, b.suffix, b.cols
        for k, v in b.rows.items():
            out.rows.setdefault(k, v)
    return out


def _values_rows(sql: str, head: str) -> dict:
    """Строки VALUES оператора секции Up, который начинается с head, по первому полю."""
    out, started = {}, False
    for line in up_section(sql).split("\n"):
        if line.startswith(head):
            started = True
        elif started and line.startswith("  ("):
            row = re.sub(r"[,;]$", "", line.strip())
            out[_key(row, 1)] = row
        elif started and out:
            break
    return out


def _directions(old: str, new: str) -> tuple[list[str], list[str]]:
    """Направления 0021 от old к new: (вставки и группы — до пар, удаления — после)."""
    groups = diff(_values_rows(old, DIR_GROUPS), _values_rows(new, DIR_GROUPS))
    dirs = diff(_values_rows(old, DIR_INSERT), _values_rows(new, DIR_INSERT))
    rows_ = lambda d: ",\n".join("  " + r for r in d.values())  # noqa: E731
    before, after = [], []
    if groups.upsert:
        before.append(f"{DIR_GROUPS}\nFROM (VALUES\n{rows_(groups.upsert)}\n) AS v(id, groups)\n"
                      "WHERE d.id = v.id;\n")
    if dirs.upsert:
        before.append(f"{DIR_INSERT}\n{rows_(dirs.upsert)}\nON CONFLICT (id) DO UPDATE SET\n"
                      "  name = EXCLUDED.name,\n  subject_codes = EXCLUDED.subject_codes,\n"
                      "  groups = EXCLUDED.groups;\n")
    if dirs.removed:
        # Направление в цели ученика остаётся: каскад снял бы его из цели.
        after.append("DELETE FROM directions d WHERE d.id IN (\n" + ",\n".join(f"  {_lit(k)}" for k in dirs.removed)
                     + ")\n  AND NOT d.onboarding\n"
                     "  AND NOT EXISTS (SELECT 1 FROM trajectory_directions t WHERE t.direction_id = d.id)\n"
                     "  AND NOT EXISTS (SELECT 1 FROM trajectories t WHERE t.direction_id = d.id);\n")
    return before, after


def _lit(v: str) -> str:
    return v if v.isdigit() else "'" + v.replace("'", "''") + "'"


def _delete(table: str, cols: list[str], keys: list) -> str:
    if not keys:
        return ""
    if KEYS[table] == 1:
        return f"DELETE FROM {table} WHERE {cols[0]} IN (\n" + ",\n".join(f"  {_lit(k)}" for k in keys) + "\n);\n"
    kc = cols[:KEYS[table]]
    vals = ",\n".join("  (" + ", ".join(_lit(x) for x in k) + ")" for k in keys)
    cond = " AND ".join(f"d.{c} = v.{c}" + ("::smallint" if c == "admission_year" else "") for c in kc)
    return (f"DELETE FROM {table} d USING (VALUES\n{vals}\n) AS v({', '.join(kc)})\n"
            f"WHERE {cond};\n")


def _insert(b: Block, rows_: dict) -> str:
    if not rows_:
        return ""
    return b.prefix + "\n" + ",\n".join("  " + r for r in rows_.values()) + "\n" + b.suffix + "\n"


def render(old: list[str], new: list[str], note: str, touched: dict) -> str:
    o = {t: _merged(old, t) for t in KEYS}
    n = {t: _merged(new, t) for t in KEYS}
    d = {t: diff(o[t].rows, n[t].rows, touched.get(t, set())) for t in KEYS}
    back = {t: diff(n[t].rows, o[t].rows, touched.get(t, set())) for t in KEYS}
    # Источники, на которые ссылаются только этапы, — дело миграции сроков
    # (render_stages).
    stages_only = _source_refs(n["stages"].rows) - _source_refs(
        {**n["benefits"].rows, **{str(k): v for k, v in n["direction_benefits"].rows.items()}})
    d["sources"].upsert = {k: v for k, v in d["sources"].upsert.items()
                           if k not in stages_only or k in o["sources"].rows}
    new_sources = [k for k in d["sources"].upsert if k not in o["sources"].rows]
    gone_sources = sorted(k for k in o["sources"].rows if k not in n["sources"].rows)

    def count(t):
        return f"удаляется {len(d[t].removed)}, добавляется или меняется {len(d[t].upsert)}"

    head = (f"{note.rstrip()}\n--\n"
            f"-- Льготы по вузам: {count('benefits')}.\n"
            f"-- Льготы по направлениям: {count('direction_benefits')}.\n"
            f"-- Пары вуз–направление: {count('university_directions')}. Источников новых {len(new_sources)}.\n"
            f"--\n-- Сгенерировано datasets/parser/diff_seed.py; эквивалентность сиду — make test-db.\n")

    dirs_up, dirs_gone = _directions(old[-1], new[-1])
    dirs_back, dirs_back_gone = _directions(new[-1], old[-1])
    up = [_insert(n["sources"], d["sources"].upsert), _insert(n["universities"], d["universities"].upsert),
          *dirs_up]
    for t in ("benefits", "university_directions", "direction_benefits"):
        up += [_delete(t, n[t].cols or o[t].cols, d[t].removed), _insert(n[t], d[t].upsert)]
    up += dirs_gone
    if gone_sources:
        up.append("-- Источники, которых в новом сиде нет, — если на них больше ничто не ссылается.\n"
                  + _drop_unused_sources(gone_sources))
    down = ["-- Источники прежних льгот: в базе, накатанной уже с новыми 0003 и 0021, их нет.\n"
            + _insert(o["sources"], back["sources"].upsert),
            _insert(o["universities"], back["universities"].upsert), *dirs_back]
    # Сначала удаления, потом вставки от родителей к детям: льгота по
    # направлению вставляется, только если её пара уже есть.
    down += [_delete(t, o[t].cols or n[t].cols, back[t].removed)
             for t in ("direction_benefits", "university_directions", "benefits")]
    down += [_insert(o[t], back[t].upsert) for t in ("benefits", "university_directions", "direction_benefits")]
    down += dirs_back_gone
    if new_sources:
        down.append(_drop_unused_sources(new_sources))
    body = lambda parts: "\n".join(p for p in parts if p)  # noqa: E731
    return f"{head}\n-- +goose Up\n\n{body(up)}\n-- +goose Down\n\n{body(down)}"


def _source_refs(rows_: dict) -> set:
    """Источники, на которые ссылаются строки: source_id — последняя колонка."""
    return {_unquote(fields(r)[-1]) for r in rows_.values()} - {"NULL"}


def render_stages(old: list[str], new: list[str], note: str) -> str:
    """Сроки этапов — своей миграцией: разница stages и источники, на которые
    ссылаются изменённые этапы. Этапы только обновляются: на них ссылаются
    отметки трекера, удалять их молча нельзя."""
    o, n = _merged(old, "stages"), _merged(new, "stages")
    so, sn = _merged(old, "sources"), _merged(new, "sources")
    d, back = diff(o.rows, n.rows), diff(n.rows, o.rows)
    if d.removed:
        print("! в новом сиде нет этапов: " + ", ".join(d.removed), file=sys.stderr)
    refs = _source_refs(d.upsert)
    src = {k: v for k, v in sn.rows.items() if k in refs and so.rows.get(k) != v}
    new_sources = [k for k in src if k not in so.rows]
    head = (f"{note.rstrip()}\n--\n"
            f"-- Этапы: удаляется {len(d.removed)}, добавляется или меняется {len(d.upsert)}."
            f" Источников новых {len(new_sources)}.\n"
            f"--\n-- Сгенерировано datasets/parser/diff_seed.py --tables stages.\n")
    up = [_insert(sn, src), _delete("stages", n.cols or o.cols, d.removed), _insert(n, d.upsert)]
    down = [_delete("stages", o.cols or n.cols, back.removed), _insert(o, back.upsert)]
    if new_sources:
        down.append(_drop_unused_sources(new_sources))
    body = lambda parts: "\n".join(p for p in parts if p)  # noqa: E731
    return f"{head}\n-- +goose Up\n\n{body(up)}\n-- +goose Down\n\n{body(down)}"


def _drop_unused_sources(ids: list[str]) -> str:
    return ("DELETE FROM sources s WHERE s.id IN (\n" + ",\n".join(f"  {_lit(k)}" for k in ids)
            + ")\n  AND NOT EXISTS (SELECT 1 FROM benefits b WHERE b.source_id = s.id)\n"
            "  AND NOT EXISTS (SELECT 1 FROM direction_benefits d WHERE d.source_id = s.id)\n"
            "  AND NOT EXISTS (SELECT 1 FROM stages st WHERE st.source_id = s.id)\n"
            "  AND NOT EXISTS (SELECT 1 FROM olympiad_profiles p WHERE p.source_id = s.id);\n")


def _other_tables_changed(old: list[str], new: list[str]) -> list[str]:
    names = set()
    for sql in old + new:
        names |= set(re.findall(r"^INSERT INTO (\w+) \(", up_section(sql), re.M))
    out = []
    for t in sorted(names - set(KEYS)):
        KEYS[t] = 1
        # Направления 0021 переносит _directions, здесь — только направления 0003.
        files = (old[:1], new[:1]) if t == "directions" else (old, new)
        if _merged(files[0], t).rows != _merged(files[1], t).rows:
            out.append(t)
        del KEYS[t]
    return out


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--base", default="origin/master")
    ap.add_argument("--note", required=True, help="файл с комментарием-шапкой миграции (строки «-- …»)")
    ap.add_argument("--touched", nargs="*", default=[])
    ap.add_argument("--tables", choices=["benefits", "stages"], default="benefits",
                    help="benefits — льготы и направления (0022), stages — сроки этапов")
    ap.add_argument("-o", "--out", required=True)
    a = ap.parse_args()
    old = [subprocess.run(["git", "show", f"{a.base}:{p}"], cwd=ROOT, check=True,
                          capture_output=True, text=True).stdout for p in SEEDS]
    new = [(ROOT / p).read_text(encoding="utf-8") for p in SEEDS]
    for t in _other_tables_changed(old, new):
        print(f"! в сидах изменилась таблица {t} — в эту миграцию она не входит", file=sys.stderr)
    touched = {"benefits": set()}
    for p in a.touched:
        touched["benefits"] |= touched_keys((ROOT / p).read_text(encoding="utf-8"), "benefits")
    note = Path(a.note).read_text(encoding="utf-8")
    sql = render_stages(old, new, note) if a.tables == "stages" else render(old, new, note, touched)
    (ROOT / a.out).write_text(sql, encoding="utf-8")
    print(sql.split("\n\n-- +goose Up")[0].split("--\n", 1)[-1], file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
