#!/usr/bin/env python3
"""Перечень олимпиад №669 с rsr-olymp.ru -> olympiad_index_669.json + subject_slugs.json.

Оба файла — общие с исполнителем датасета C: olympiad_id должен совпасть
у обоих без переписки, поэтому слаг строится формулой, а не на глаз.
"""
import json, re, sys, unicodedata
from datetime import date
from pathlib import Path
from urllib.request import urlopen, Request

SRC = "https://rsr-olymp.ru/"
OUT = Path(__file__).resolve().parent.parent / "data"

# Таблица 1.3 спеки — фиксированная, не трогать.
SPEC_SLUGS = {
    "информатика": "informatika",
    "математика": "matematika",
    "программирование": "programmirovanie",
    "физика": "fizika",
    "биология": "biologiya",
    "химия": "himiya",
    "обществознание": "obschestvoznanie",
    "экономика": "ekonomika",
}

# Транслитерация 1:1 по звучанию, как требует п. 1.3 спеки.
TRANSLIT = {
    "а": "a", "б": "b", "в": "v", "г": "g", "д": "d", "е": "e", "ё": "e",
    "ж": "zh", "з": "z", "и": "i", "й": "y", "к": "k", "л": "l", "м": "m",
    "н": "n", "о": "o", "п": "p", "р": "r", "с": "s", "т": "t", "у": "u",
    "ф": "f", "х": "h", "ц": "c", "ч": "ch", "ш": "sh", "щ": "sch",
    "ъ": "", "ы": "y", "ь": "", "э": "e", "ю": "yu", "я": "ya",
}


def slugify(profile: str) -> str:
    """Профиль -> слаг. Значения из таблицы 1.3 имеют приоритет над формулой."""
    key = profile.strip().lower()
    if key in SPEC_SLUGS:
        return SPEC_SLUGS[key]
    out = []
    for ch in key:
        if ch in TRANSLIT:
            out.append(TRANSLIT[ch])
        elif ch.isalnum() and ch.isascii():
            out.append(ch)
        elif ch in " -_/,.:;()«»\"'":
            out.append("-")
    slug = re.sub(r"-+", "-", "".join(out)).strip("-")
    return slug


def fetch(url: str) -> str:
    req = Request(url, headers={"User-Agent": "Mozilla/5.0 (olymp-navigator dataset build)"})
    with urlopen(req, timeout=60) as r:
        return r.read().decode("utf-8", errors="replace")


def parse(html: str) -> list[dict]:
    """Таблица перечня свёрнута через rowspan: № и название заданы один раз
    на блок строк-профилей. Без переноса этих ячеек вниз теряется ~2/3 строк."""
    rows = re.findall(r"<tr[^>]*>(.*?)</tr>", html, re.S)
    carry: dict[int, list] = {}
    out = []
    for row in rows:
        cells = re.findall(r"<td([^>]*)>(.*?)</td>", row, re.S)
        if not cells:
            continue
        rec: list = [None] * 5
        for ci, held in carry.items():
            if held[1] > 0:
                rec[ci] = held[0]
                held[1] -= 1
        i = 0
        for attrs, raw in cells:
            while i < 5 and rec[i] is not None:
                i += 1
            if i >= 5:
                break
            txt = re.sub(r"\s+", " ", re.sub(r"<[^>]+>", "", raw))
            txt = unicodedata.normalize("NFKC", txt).replace("­", "").strip()
            rec[i] = txt
            span = re.search(r'rowspan="(\d+)"', attrs)
            if span:
                carry[i] = [txt, int(span.group(1)) - 1]
            i += 1
        if not (rec[0] and rec[0].isdigit()):
            continue
        num, name, profile, subjects, level = rec
        out.append({
            "perechen_number_669": int(num),
            "name": name,
            "profile": profile,
            "subjects": [s.strip() for s in subjects.split(",") if s.strip()],
            "level": {"1": "I", "2": "II", "3": "III"}.get(level, level),
            "profile_slug": slugify(profile),
            "olympiad_id": f"p669-{int(num)}-{slugify(profile)}",
        })
    return out


def main() -> int:
    rows = parse(fetch(SRC))

    numbers = {r["perechen_number_669"] for r in rows}
    assert len(numbers) == 83, f"ожидалось 83 олимпиады в перечне, получено {len(numbers)}"
    assert len(rows) > 250, f"строк-профилей {len(rows)} — похоже, rowspan не развёрнут"
    assert all(r["olympiad_id"] and r["profile_slug"] for r in rows), "пустой slug"
    assert all(r["level"] in ("I", "II", "III") for r in rows), "нераспознанный уровень"
    ids = {r["olympiad_id"] for r in rows}
    assert "p669-8-informatika" in ids, "Высшая проба/информатика не найдена — сменилась вёрстка"

    OUT.mkdir(exist_ok=True)
    today = date.today().isoformat()
    (OUT / "olympiad_index_669.json").write_text(json.dumps({
        "_dataset": "olympiad_index_669",
        "_note": "Перечень олимпиад школьников, приказ Минобрнауки №669 от 30.08.2025. "
                 "Общий справочник для датасетов B и C: olympiad_id строится формулой "
                 "раздела 4 спеки, поэтому у обоих исполнителей совпадает.",
        "_source_url": SRC,
        "_source_date": today,
        "olympiads": rows,
    }, ensure_ascii=False, indent=2), encoding="utf-8")

    slugs = {}
    for r in rows:
        slugs.setdefault(r["profile"].lower(), r["profile_slug"])
    (OUT / "subject_slugs.json").write_text(json.dumps({
        "_dataset": "subject_slugs",
        "_note": "Таблица 1.3 спеки, достроенная до всех профилей перечня №669. "
                 "in_spec: true — значение зафиксировано спекой; false — выведено формулой slugify().",
        "_source_date": today,
        "slugs": [
            {"profile": p, "slug": s, "in_spec": p in SPEC_SLUGS}
            for p, s in sorted(slugs.items())
        ],
    }, ensure_ascii=False, indent=2), encoding="utf-8")

    in_spec = sum(1 for p in slugs if p in SPEC_SLUGS)
    print(f"олимпиад: {len(numbers)}, строк-профилей: {len(rows)}, профилей: {len(slugs)} "
          f"(из таблицы 1.3: {in_spec}, достроено: {len(slugs) - in_spec})")
    print(f"записано: {OUT/'olympiad_index_669.json'}, {OUT/'subject_slugs.json'}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
