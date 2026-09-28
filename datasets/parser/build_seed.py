#!/usr/bin/env python3
"""Датасеты A, B, C -> packages/db/migrations/0003_seed_content.sql.

Сид контента генерируется один раз при обновлении датасетов, результат
коммитится и накатывается goose вместе со схемой. Так проверяющему не нужны
ни Python, ни 22-мегабайтный JSON: `docker compose up` видит обычную миграцию.

Все вставки — ON CONFLICT (id) DO UPDATE по текстовым ключам, поэтому
повторный прогон SQL ничего не ломает, а вывод генератора детерминирован:
тот же вход даёт байт в байт тот же файл.

Правила, которые здесь зашиты, описаны в плане бэкенда и проверены на данных:

* C-запись — это профиль олимпиады. `olympiads.id` выводится из номера
  перечня (`p669-5`) или равен `vsosh-<предмет>`.
* Льготы берутся только у программ со status='offered' и сворачиваются до
  ключа (профиль, вуз, год приёма) — это гранулярность схемы. Одна строка на
  ключ, различия победителя и призёра уходят в `benefit` и `note`.
* Запись вуза с требованием уровня строже, чем уровень профиля, отбрасывается.
* `is_demo` — признак, а не фильтр: если хоть одна запись ключа демо, у льготы
  нет источника, и API честно отдаёт «данные уточняются».
* Даты этапов: опубликованные — как есть, а если опубликованы только ранние
  этапы, финал — демо. Если известны только сроки прошлого сезона — демо-даты
  по ним, через 52 недели. ВсОШ — по графику «Сириуса» и предельным срокам
  Порядка проведения (vsosh_stages). Остальные профили — демо, с пометкой
  is_demo, чтобы работали трекер, календарь и напоминания.

Вторым файлом генератор пишет 0021_university_directions_content.sql —
направления вузов и льготы по направлениям (схема — 0020). Правила — у
build_university_directions.

Запуск: `make seed` (или `python3 datasets/parser/build_seed.py`).
"""
from __future__ import annotations

import hashlib
import json
import re
from collections import Counter, defaultdict
from dataclasses import dataclass, field
from datetime import date, timedelta
from pathlib import Path
from urllib.parse import quote

from diff_seed import _unquote, fields, rows as sql_rows, up_section

ROOT = Path(__file__).resolve().parent.parent
REPO = ROOT.parent
DATA, SPEC = ROOT / "data", ROOT / "spec"
OUT = REPO / "packages" / "db" / "migrations" / "0003_seed_content.sql"
UD_OUT = REPO / "packages" / "db" / "migrations" / "0021_university_directions_content.sql"
# Вымышленные олимпиады — только локальный стенд и CI. Не migrations-demo:
# та катится и в прод, пока включён демо-режим в браузере.
DEMO_OUT = REPO / "packages" / "db" / "migrations-local" / "0001_fictional_olympiads.sql"

SCHOOL_YEAR = "2026/27"
# Датасеты собраны 21.09.2026; это дата проверки источников, отмеченных как факт.
CHECKED = "2026-09-21"
TZ = "+03"  # даты этапов публикуются по Москве

# ---------------------------------------------------------------------------
# Справочники, которых нет в датасетах
# ---------------------------------------------------------------------------

# Коды предметов задаёт фронт (apps/web/src/screens/Profile.tsx, Catalog.tsx).
SUBJECTS = [
    ("inf", "Информатика"),
    ("math", "Математика"),
    ("phys", "Физика"),
    ("chem", "Химия"),
    ("bio", "Биология"),
    ("soc", "Обществознание"),
    ("econ", "Экономика"),
    ("astro", "Астрономия"),
    ("ecol", "Экология"),
]
SUBJECT_BY_NAME = {name: code for code, name in SUBJECTS}
# «по информатике» — для названий ВсОШ.
SUBJECT_PREP = {
    "inf": "информатике", "math": "математике", "phys": "физике", "chem": "химии",
    "bio": "биологии", "soc": "обществознанию", "econ": "экономике",
    "astro": "астрономии", "ecol": "экологии",
}

# У профилей из missing_in_C нет школьного предмета, а без него профиль не
# попадёт в подбор. Сопоставление — по смыслу профиля.
MISSING_SUBJECT = {
    "p669-5-yadernye-tehnologii": "phys",
    "p669-49-estestvennye-nauki": "phys",
    "p669-37-astronomiya": "astro",
    "p669-50-vysokie-tehnologii": "phys",
    "p669-55-kompyuternoe-modelirovanie-i-grafika": "inf",
    "vsosh-astronomiya": "astro",
    "p669-59-medicina": "bio",
    "p669-37-robototehnika": "inf",
    "vsosh-ekologiya": "ecol",
    "p669-37-ekologiya": "ecol",
}

# Короткие имена вузов для плиток и чипов профиля (как в макетах).
UNIVERSITY_SHORT = {
    "msu": "МГУ", "spbu": "СПбГУ", "hse": "ВШЭ", "mipt": "МФТИ", "itmo": "ИТМО",
    "nsu": "НГУ", "kfu": "КФУ", "innopolis": "УИ", "sechenov": "ПМГМУ",
    "kazan-gmu": "КГМУ",
}

# Ключевые предметы профильных групп — фактор скоринга «совпадение профиля с
# направлением» (ТЗ §6.1).
GROUP_SUBJECTS = {
    "ИТ": ["inf", "math"],
    "Физика": ["phys", "math"],
    "Биомед": ["bio", "chem"],
    "Экономика": ["econ", "soc", "math"],
}

# Направления, профильный предмет которых — предмет ВсОШ: коды или их начала
# (УГСН). Победители и призёры ВсОШ поступают на них обычно без вступительных
# испытаний. Если по правилам вуза там только 100 баллов (КФУ: ВсОШ по
# информатике на ИТ, хотя на страницах ИТИС — БВИ), в льготе остаётся факт из
# нормативного документа, а в примечании — просьба уточнить. Группы программ
# для этого слишком широкие: в «Экономику» попадают лингвистика и политология,
# где 100 баллов за ВсОШ по экономике вполне законны.
VSOSH_OWN_DIRECTIONS = {
    "vsosh-informatika": ("09.", "10.", "02.03.02", "02.03.03"),
    "vsosh-fizika": ("03.",),
    "vsosh-biologiya": ("06.",),
    "vsosh-ekonomika": ("38.",),
}
DOUBT_NOTE = ("Уточните в приёмной комиссии — по правилам приёма здесь 100 баллов, "
              "хотя ВсОШ по профильному предмету обычно даёт БВИ")

# ВсОШ не подтверждают ЕГЭ, и предмета в записи нет: 100 баллов засчитают по
# предмету самой олимпиады, если он есть в ЕГЭ. Астрономии, экологии и
# экономики в ЕГЭ нет — что им сопоставил вуз, в записях не сказано.
VSOSH_EGE_SUBJECT = {
    "vsosh-informatika": "Информатика", "vsosh-matematika": "Математика",
    "vsosh-fizika": "Физика", "vsosh-himiya": "Химия", "vsosh-biologiya": "Биология",
    "vsosh-obschestvoznanie": "Обществознание",
}

# Направления-цели для онбординга (F8): по четыре самых распространённых в
# вузах базы на каждую профильную группу. Список короткий, потому что это
# клавиатура в чате.
DIRECTIONS = [
    ("09.03.04", "ИТ", None),
    ("01.03.02", "ИТ", None),
    ("09.03.01", "ИТ", None),
    ("10.03.01", "ИТ", None),
    ("03.03.02", "Физика", None),
    ("03.03.01", "Физика", None),
    ("03.05.01", "Физика", ["astro", "phys", "math"]),
    ("16.03.01", "Физика", None),
    ("31.05.01", "Биомед", None),
    ("19.03.01", "Биомед", None),
    ("33.05.01", "Биомед", None),
    ("06.03.01", "Биомед", ["bio", "chem", "ecol"]),
    ("38.03.01", "Экономика", None),
    ("38.03.02", "Экономика", None),
    ("38.03.05", "Экономика", None),
    ("38.03.04", "Экономика", None),
]

# Олимпиады вне перечня (F16). В датасетах их нет, а без них блок «Вне
# перечня» и пояснение «до 10 баллов» не проверить. Это вымышленные примеры —
# так и написано в организаторе, источника нет, даты демо. Первая — та же,
# что в прототипе (docs/prototype/index.html). В основной сид не входят:
# в проде их приняли бы за настоящие, поэтому они идут в DEMO_OUT.
OTHER_OLYMPIADS = [
    {
        "id": "other-tyk", "name": "Турнир юных программистов Казани",
        "subject": "inf", "grades": (7, 11),
        "benefits": [("kfu", 3), ("innopolis", 2)],
    },
    {
        "id": "other-impuls", "name": "Городская олимпиада по физике «Импульс»",
        "subject": "phys", "grades": (7, 11),
        "benefits": [("kfu", 2)],
    },
    {
        "id": "other-biznes-start", "name": "Олимпиада по экономике «Бизнес-старт»",
        "subject": "econ", "grades": (8, 11),
        "benefits": [("hse", 2), ("kfu", 1)],
    },
]
OTHER_ORGANIZER = "Вымышленный пример для демонстрации"
OTHER_CITY = {"other-tyk": ("Казань", "16"), "other-impuls": ("Казань", "16"),
              "other-biznes-start": ("Москва", "77")}

# Города финала в датасетах нет. Олимпиаду вуза проводит сам вуз, и главная
# площадка её финала — в городе вуза, поэтому город выводим из организатора.
# Это приближение: у многих таких олимпиад есть и региональные площадки
# (README, «Данные»). Министерство, оргкомитеты и консорциумы нескольких
# вузов однозначного города не имеют и остаются без него.
ORGANIZER_CITY = {
    "НИУ ВШЭ": ("Москва", "77"),
    "МГУ имени М.В. Ломоносова": ("Москва", "77"),
    "НИЯУ МИФИ": ("Москва", "77"),
    "НИТУ МИСИС": ("Москва", "77"),
    "МГТУ им. Н.Э. Баумана": ("Москва", "77"),
    "Финансовый университет при Правительстве РФ": ("Москва", "77"),
    "РНИМУ им. Н.И. Пирогова": ("Москва", "77"),
    "Сеченовский Университет": ("Москва", "77"),
    "РЭУ им. Г.В. Плеханова": ("Москва", "77"),
    "РАНХиГС": ("Москва", "77"),
    "РГГУ": ("Москва", "77"),
    "НИУ «МЭИ»": ("Москва", "77"),
    "РХТУ им. Д.И. Менделеева": ("Москва", "77"),
    "МФТИ": ("Долгопрудный", "50"),
    "СПбГУ": ("Санкт-Петербург", "78"),
    "Университет ИТМО": ("Санкт-Петербург", "78"),
    "Санкт-Петербургский горный университет": ("Санкт-Петербург", "78"),
    "БГТУ «ВОЕНМЕХ» им. Д.Ф. Устинова": ("Санкт-Петербург", "78"),
    "Казанский (Приволжский) федеральный университет": ("Казань", "16"),
    "Университет Иннополис": ("Иннополис", "16"),
    "УрФУ": ("Екатеринбург", "66"),
    "Челябинский государственный университет": ("Челябинск", "74"),
    "ТГПУ им. Л.Н. Толстого": ("Тула", "71"),
    "НГУ": ("Новосибирск", "54"),
    "НГТУ": ("Новосибирск", "54"),
    "СУНЦ НГУ": ("Новосибирск", "54"),
    "Сибирский федеральный университет": ("Красноярск", "24"),
    "ННГУ им. Н.И. Лобачевского": ("Нижний Новгород", "52"),
    "Томский государственный университет": ("Томск", "70"),
    "Дальневосточный федеральный университет": ("Владивосток", "25"),
    "Пермский государственный национальный исследовательский университет": ("Пермь", "59"),
}

BENEFIT_ORDER = {"bvi": 0, "bvi_winners": 1, "score100": 2, "extra_points": 3}
LEVEL_RANK = {"I": 1, "II": 2, "III": 3}

# ---------------------------------------------------------------------------
# Модель вывода
# ---------------------------------------------------------------------------


@dataclass
class Seed:
    subjects: list[dict] = field(default_factory=list)
    directions: list[dict] = field(default_factory=list)
    universities: list[dict] = field(default_factory=list)
    sources: dict[str, dict] = field(default_factory=dict)
    olympiads: dict[str, dict] = field(default_factory=dict)
    profiles: dict[str, dict] = field(default_factory=dict)
    stages: list[dict] = field(default_factory=list)
    benefits: list[dict] = field(default_factory=list)
    stats: Counter = field(default_factory=Counter)


def load(name: str, key: str, base: Path = DATA):
    return json.loads((base / name).read_text(encoding="utf-8"))[key]


# ---------------------------------------------------------------------------
# Источники
# ---------------------------------------------------------------------------


def safe_url(url: str) -> str:
    """Кириллица в пути -> процентная кодировка: контракт требует format: uri."""
    return quote(url, safe=":/?#[]@!$&'()*+,;=%~")


def source_id(url: str, page) -> str:
    raw = f"{url}#{page if page is not None else ''}"
    return "src-" + hashlib.sha1(raw.encode("utf-8")).hexdigest()[:12]


def add_source(seed: Seed, url: str, page, kind: str, title: str, verified) -> str | None:
    if not url:
        return None
    sid = source_id(url, page)
    shown = safe_url(url)
    if page is not None and ".pdf" in url.lower():
        shown += f"#page={page}"
    prev = seed.sources.get(sid)
    if prev is None:
        seed.sources[sid] = {"id": sid, "kind": kind, "title": title, "url": shown,
                             "verified_at": verified}
    elif verified and not prev["verified_at"]:
        prev["verified_at"] = verified
    return sid


def later_source_refs() -> list[tuple[str, str]]:
    """(миграция, источник): на что ссылаются миграции после сида, не вставляя
    источник сами."""
    out = []
    for f in sorted(OUT.parent.glob("*.sql")):
        if f in (OUT, UD_OUT):
            continue
        # Удаление источников — не ссылка на них.
        up = re.sub(r"DELETE FROM sources .*?;", "", up_section(f.read_text(encoding="utf-8")), flags=re.S)
        own = set(sql_rows(up, "sources"))
        out += [(f.name, sid) for sid in sorted(set(re.findall(r"'(src-[0-9a-f]{12})'", up)) - own)]
    return out


def keep_later_sources(seed: Seed):
    """Источник, на который ссылается миграция после сида (0018 — страница МГУ
    о Севастополе), остаётся в сиде, даже когда льгот с ним в датасете нет:
    на чистой базе такая миграция идёт поверх свежего сида и иначе упала бы на
    внешнем ключе. Строка берётся из закоммиченных сидов — она там уже есть."""
    committed = {}
    for path in (OUT, UD_OUT):
        committed.update(sql_rows(path.read_text(encoding="utf-8"), "sources"))
    for name, sid in later_source_refs():
        if sid in seed.sources:
            continue
        if sid not in committed:
            raise SystemExit(f"{name} ссылается на {sid}, а в сидах его нет — верните строку источника")
        f = [None if x == "NULL" else _unquote(x.removesuffix("::date")) for x in fields(committed[sid])]
        seed.sources[sid] = dict(zip(["id", "kind", "title", "url", "verified_at"], f))


# ---------------------------------------------------------------------------
# Профили и олимпиады
# ---------------------------------------------------------------------------


def olympiad_id_of(profile_id: str, number) -> str:
    if profile_id.startswith("vsosh-"):
        return profile_id
    return f"p669-{number}"


def grades_from_name(name: str):
    m = re.search(r"(\d{1,2})\s*[-–]\s*(\d{1,2})\s*класс", name or "")
    return (int(m.group(1)), int(m.group(2))) if m else None


def build_profiles(seed: Seed, C: list[dict], missing: list[dict], perechen_title: str):
    perechen_src = add_source(seed, "https://rsr-olymp.ru/", None, "order", perechen_title, CHECKED)
    by_number: dict[int, list[dict]] = defaultdict(list)
    for c in C:
        if c["perechen_number_669"] is not None:
            by_number[c["perechen_number_669"]].append(c)

    def olympiad_row(oid, name, organizer, kind, url):
        city, region = ORGANIZER_CITY.get(organizer, (None, None))
        return {"id": oid, "name": name, "organizer": organizer, "kind": kind,
                "official_url": url or None, "format": None, "final_city": city,
                "final_region_code": region}

    for c in C:
        pid = c["olympiad_id"]
        is_vsosh = pid.startswith("vsosh-")
        subject = SUBJECT_BY_NAME[c["subject"]]
        oid = olympiad_id_of(pid, c["perechen_number_669"])
        if is_vsosh:
            vs_src = add_source(seed, c["source_url"], c["source_page"], "site",
                                "Всероссийская олимпиада школьников — официальный сайт", CHECKED)
            seed.olympiads[oid] = olympiad_row(oid, f"ВсОШ по {SUBJECT_PREP[subject]}",
                                               "Минпросвещения России", "vsosh", c["official_site"])
            prof_src, level, grades = vs_src, None, (5, 11)
        else:
            if oid not in seed.olympiads:
                siblings = by_number[c["perechen_number_669"]]
                name = Counter(s["name"] for s in siblings).most_common(1)[0][0]
                seed.olympiads[oid] = olympiad_row(oid, name, c["organizer"], "perechen",
                                                   c["official_site"])
            ge = c["grades_eligible"] or []
            grades = (min(ge), max(ge)) if ge else (8, 11)
            prof_src, level = perechen_src, c["level"]
        seed.profiles[pid] = {
            "id": pid, "olympiad_id": oid, "subject_code": subject,
            "profile_slug": c["profile_slug"], "profile_name": c["profile"],
            "level": level, "school_year": SCHOOL_YEAR,
            "grades_from": grades[0], "grades_to": grades[1], "source_id": prof_src,
            "_c": c,
        }

    for m in missing:
        pid = m["olympiad_id"]
        subject = MISSING_SUBJECT[pid]
        oid = olympiad_id_of(pid, m["perechen_number_669"])
        is_vsosh = pid.startswith("vsosh-")
        if oid not in seed.olympiads:
            if is_vsosh:
                seed.olympiads[oid] = olympiad_row(oid, f"ВсОШ по {SUBJECT_PREP[subject]}",
                                                   "Минпросвещения России", "vsosh",
                                                   "https://vserosolimp.edsoo.ru/")
            else:
                siblings = by_number.get(m["perechen_number_669"], [])
                organizer = siblings[0]["organizer"] if siblings else None
                url = siblings[0]["official_site"] if siblings else None
                seed.olympiads[oid] = olympiad_row(oid, m["name"], organizer, "perechen", url)
        grades = (5, 11) if is_vsosh else (grades_from_name(m["name"]) or (8, 11))
        vs_src = next((p["source_id"] for p in seed.profiles.values()
                       if p["olympiad_id"].startswith("vsosh-")), None)
        seed.profiles[pid] = {
            "id": pid, "olympiad_id": oid, "subject_code": subject,
            "profile_slug": m["profile_slug"], "profile_name": m["profile"],
            "level": None if is_vsosh else m["level"], "school_year": SCHOOL_YEAR,
            "grades_from": grades[0], "grades_to": grades[1],
            "source_id": vs_src if is_vsosh else perechen_src,
            "_c": None,
        }


# ---------------------------------------------------------------------------
# Этапы: три яруса
# ---------------------------------------------------------------------------


def fnv32(s: str) -> int:
    """FNV-1a, 32 бита: детерминированный «случай» без генератора случайных чисел."""
    h = 0x811C9DC5
    for b in s.encode("utf-8"):
        h ^= b
        h = (h * 0x01000193) & 0xFFFFFFFF
    return h


def ts(d: date | str, end: bool = False) -> str:
    d = d if isinstance(d, str) else d.isoformat()
    return f"{d} {'23:59:59' if end else '00:00:00'}{TZ}"


# Для регистрации и отборочного окна важен последний день, для очных этапов —
# первый: к нему надо успеть подготовиться и приехать.
DEADLINE_AT_END = {"registration", "qualifying", "school"}

DEFAULT_TITLE = {
    "registration": "Регистрация", "qualifying": "Отборочный этап",
    "final": "Заключительный этап", "school": "Школьный этап",
    "municipal": "Муниципальный этап", "regional": "Региональный этап",
}


def stage(pid, kind, n, start, end, online, demo, src=None, title=None):
    # Бывает, что организатор называет только крайний день («не позднее
    # 31 марта»): начала нет, и срок — этот день.
    starts, ends = (ts(start) if start else None), ts(end or start, end=True)
    return {
        "id": f"{pid}:{kind}:{n}", "olympiad_profile_id": pid, "kind": kind,
        "title": title or DEFAULT_TITLE[kind], "starts_at": starts, "ends_at": ends,
        "deadline_at": ends if kind in DEADLINE_AT_END or not starts else starts,
        "is_online": online, "is_demo": demo, "source_id": src,
    }


# Календарь сезона опубликован целиком или частично: во втором случае финала
# ещё нет, и он ставится демо — иначе трекер и календарь не видели бы, чем
# кончается сезон.
PUBLISHED = {"published_2026_27", "partial_2026_27"}
# Сроков сезона ещё нет, но в etapy — прошлогодние: демо-даты — те же через
# 52 недели, чтобы туры пришлись на тот же день недели.
PREVIOUS_SEASON_SHIFT = timedelta(weeks=52)


def published_stages(seed: Seed, p: dict) -> list[dict]:
    c = p["_c"]
    demo = c["etapy_status"] not in PUBLISHED
    src = None if demo else add_source(seed, c["etapy_source_url"], c["etapy_source_page"], "site",
                                       f"{seed.olympiads[p['olympiad_id']]['name']} — сроки этапов",
                                       c.get("etapy_checked_at") or CHECKED)
    shift = PREVIOUS_SEASON_SHIFT if c["etapy_status"] == "previous_season" else timedelta(0)

    def moved(d):
        return (date.fromisoformat(d) + shift).isoformat() if d else None

    out, counter = [], Counter()
    for e in c["etapy"]:
        kind = e["type"]
        counter[kind] += 1
        online = e["format"] == "online" or (kind == "registration" and e["format"] is None)
        out.append(stage(p["id"], kind, counter[kind], moved(e["start_date"]), moved(e["end_date"]),
                         online, demo, src, e["stage_name"]))
    if c["etapy_status"] == "partial_2026_27" and not counter["final"]:
        # Как у сгенерированных демо-дат: финал через месяц после отбора, три дня.
        final = date.fromisoformat(max(e["end_date"] for e in c["etapy"])) + timedelta(days=30)
        out.append(stage(p["id"], "final", 1, final, final + timedelta(days=2), False, True))
    return out


# ВсОШ 2026/27. Школьный этап по шести предметам — на «Сириус.Курсах»: день
# зависит от группы регионов, здесь — окно от первой группы до последней
# (7–11 класс). По остальным предметам и дальше — предельные сроки Порядка
# проведения: день назначает регион, а заключительный этап — Минпросвещения.
VSOSH_SIRIUS = {
    "vsosh-astronomiya": ("2026-09-22", "2026-09-25"),
    "vsosh-fizika": ("2026-09-29", "2026-10-02"),
    "vsosh-biologiya": ("2026-10-06", "2026-10-09"),
    "vsosh-himiya": ("2026-10-12", "2026-10-16"),
    "vsosh-matematika": ("2026-10-13", "2026-10-16"),
    "vsosh-informatika": ("2026-10-19", "2026-10-23"),
}
VSOSH_CHECKED = "2026-09-26"


def vsosh_stages(seed: Seed, pid: str) -> list[dict]:
    """Четыре этапа ВсОШ: школьный — по графику «Сириуса» или «не позднее
    1 ноября», муниципальный — до 25 декабря, региональный — до 1 марта,
    заключительный — до конца апреля (Порядок проведения)."""
    order = add_source(seed, "https://vserosolimp.edsoo.ru/", None, "site",
                       "ВсОШ: сроки этапов по Порядку проведения", VSOSH_CHECKED)
    if pid in VSOSH_SIRIUS:
        start, end = VSOSH_SIRIUS[pid]
        sirius = add_source(seed, "https://siriusolymp.ru/school2026/about", None, "site",
                            "ВсОШ: график школьного этапа на «Сириус.Курсах», 2026/27", VSOSH_CHECKED)
        school = stage(pid, "school", 1, start, end, True, False, sirius)
    else:
        school = stage(pid, "school", 1, None, "2026-11-01", False, False, order)
    return [
        school,
        stage(pid, "municipal", 1, None, "2026-12-25", False, False, order),
        stage(pid, "regional", 1, None, "2027-03-01", False, False, order),
        stage(pid, "final", 1, None, "2027-04-30", False, False, order),
    ]


def generated_stages(pid: str) -> list[dict]:
    """Демо-даты для профиля перечня без опубликованного календаря.

    Регистрация стартует между 1 октября и 14 ноября 2026 и длится три недели,
    отборочный онлайн-этап — через неделю на две недели, финал — через месяц
    на три дня. Сроки расходятся по октябрю — январю, поэтому трекер,
    календарь и все цвета плашек сроков работают на живых данных.
    """
    reg = date(2026, 10, 1) + timedelta(days=fnv32(pid) % 45)
    reg_end = reg + timedelta(days=20)
    qual = reg_end + timedelta(days=7)
    qual_end = qual + timedelta(days=13)
    final = qual_end + timedelta(days=30)
    return [
        stage(pid, "registration", 1, reg, reg_end, True, True),
        stage(pid, "qualifying", 1, qual, qual_end, True, True),
        stage(pid, "final", 1, final, final + timedelta(days=2), False, True),
    ]


def build_stages(seed: Seed):
    for pid in sorted(seed.profiles):
        p = seed.profiles[pid]
        c = p["_c"]
        if pid.startswith("vsosh-"):
            rows, tier = vsosh_stages(seed, pid), "vsosh"
        elif c and c["etapy"]:
            rows = published_stages(seed, p)
            tier = "published" if c["etapy_status"] in PUBLISHED else "previous"
        elif c:
            rows, tier = generated_stages(pid), "generated"
        else:
            # Профиль есть только в перечне и в правилах вузов: календаря нет,
            # карточка честно покажет «даты уточняются».
            rows, tier = [], "none"
        seed.stats[f"stages_profiles_{tier}"] += 1
        seed.stages.extend(rows)


def olympiad_format(seed: Seed):
    """Подпись формата для карточки — «Онлайн-отбор, очный финал»."""
    by_olymp = defaultdict(list)
    for s in seed.stages:
        by_olymp[seed.profiles[s["olympiad_profile_id"]]["olympiad_id"]].append(s)
    for oid, o in seed.olympiads.items():
        st = by_olymp.get(oid, [])
        if o["kind"] == "vsosh":
            o["format"] = "Первый этап в школе"
            continue
        parts = []
        if any(s["kind"] == "qualifying" and s["is_online"] for s in st):
            parts.append("онлайн-отбор")
        if any(s["kind"] == "final" and not s["is_online"] for s in st):
            parts.append("очный финал")
        o["format"] = ", ".join(parts).capitalize() if parts else None


# ---------------------------------------------------------------------------
# Льготы
# ---------------------------------------------------------------------------


def level_ok(profile_level, required, is_vsosh: bool) -> bool:
    if is_vsosh:
        return True
    if required == "ВсОШ":
        return False
    if profile_level not in LEVEL_RANK or required not in LEVEL_RANK:
        return True
    return LEVEL_RANK[profile_level] <= LEVEL_RANK[required]


def grades_text(grades: list[int]) -> str:
    """9, 9–10, 9, 11."""
    if len(grades) > 1 and grades[-1] - grades[0] == len(grades) - 1:
        return f"{grades[0]}–{grades[-1]}"
    return ", ".join(map(str, grades))


def doubtful_olympiads(code: str | None, records: list[dict]) -> set[str]:
    """ВсОШ по профильному предмету направления code, за которую победителю
    или призёру программа не даёт БВИ, — см. VSOSH_OWN_DIRECTIONS."""
    namesakes = {oid for oid, prefixes in VSOSH_OWN_DIRECTIONS.items() if (code or "").startswith(prefixes)}
    bvi = {(r["olympiad_id"], r["diploma_status"]) for r in records if r["benefit_type"] == "БВИ"}
    return {r["olympiad_id"] for r in records if r["olympiad_id"] in namesakes
            and (r["olympiad_id"], r["diploma_status"]) not in bvi}


def score100_subject(records: list[dict]) -> str:
    """На какой предмет засчитают 100 баллов: предмет, соответствующий профилю
    олимпиады, — тот же, что подтверждает льготу (КФУ, приложение 3, стр. 2)."""
    subjects = sorted({s for r in records
                       for s in (r["ege_confirm_subject"] or VSOSH_EGE_SUBJECT.get(r["olympiad_id"], "")).split(" или ")
                       if s})
    if not subjects:
        return "Предмет, по которому засчитают 100 баллов, уточните в приёмной комиссии"
    return "100 баллов засчитают по предмету " + " или ".join(f"«{s}»" for s in subjects)


def aggregate_key(records: list[dict], doubtful: bool = False) -> dict:
    """Свернуть записи вуза по одному профилю и году в строку benefits.
    doubtful — льгота сомнительна (doubtful_olympiads) хоть на одной программе."""
    by_status = defaultdict(set)
    for r in records:
        by_status[r["diploma_status"]].add(r["benefit_type"])
    win, pri = by_status.get("pobeditel", set()), by_status.get("prizyor", set())
    notes = []
    if "БВИ" in win and "БВИ" in pri:
        benefit = "bvi"
    elif "БВИ" in win:
        benefit = "bvi_winners"
        notes.append("Победителю — БВИ, призёру — 100 баллов" if "100_ballov" in pri
                     else "БВИ только победителю")
    elif "БВИ" in pri:
        # Призёру БВИ, а победителю нет — так бывает только при неполной
        # выгрузке правил. Победитель не может получить меньше призёра.
        benefit = "bvi"
    else:
        benefit = "score100"
        if not pri:
            notes.append("100 баллов только победителю")
    if doubtful and benefit != "bvi":
        notes.append(DOUBT_NOTE)

    # Классы — у записей лучшей льготы: строка БВИ не обещает её за диплом
    # класса, которому дают только 100 баллов; такие классы — в заметке (#78).
    best = [r for r in records if r["benefit_type"] == "БВИ"
            and (benefit != "bvi_winners" or r["diploma_status"] == "pobeditel")] or records
    grade_lists = [r["eligible_grades"] for r in best]
    diploma_grades = (sorted({g for gl in grade_lists for g in gl})
                      if all(grade_lists) else None)
    taken = {id(r) for r in best}
    rest = [r["eligible_grades"] for r in records if id(r) not in taken]
    if diploma_grades and rest and all(rest):
        extra = sorted({g for gl in rest for g in gl} - set(diploma_grades))
        if extra:
            notes.append(f"За диплом {grades_text(extra)} класса — 100 баллов")

    scores = sorted({r["ege_confirm_min_score"] for r in records
                     if r["ege_confirm_min_score"] is not None})
    ege_min = scores[0] if scores else None
    if len(scores) > 1:
        notes.append(f"Порог ЕГЭ зависит от программы: {scores[0]}–{scores[-1]} баллов")
    # Предмет записи бывает составным: «Математика или Обществознание».
    subjects = sorted({s for r in records if r["ege_confirm_subject"]
                       for s in r["ege_confirm_subject"].split(" или ")})
    if subjects:
        notes.append("Подтвердить ЕГЭ: " + " или ".join(subjects))

    # Предмет — только если 100 баллов видны в строке: у «БВИ» обоим 100
    # баллов бывают лишь на других программах, и предмет там сбивает.
    hundred = [r for r in records if r["benefit_type"] == "100_ballov"]
    subject100 = score100_subject(hundred) if hundred else None
    if subject100 and (benefit == "score100" or any("100 баллов" in n for n in notes)):
        notes.append(subject100)

    demo = any(r["is_demo"] for r in records)
    src_counter = Counter((r["source_url"], r["source_page"]) for r in records)
    (src_url, src_page), _ = sorted(src_counter.items(),
                                    key=lambda kv: (-kv[1], kv[0][0] or "", kv[0][1] or 0))[0]
    return {
        "benefit": benefit, "ege_min": ege_min, "diploma_grades": diploma_grades,
        "note": ". ".join(notes) or None, "demo": demo, "subject100": subject100,
        "src_url": src_url, "src_page": src_page,
        "src_date": max(r["source_date"] or "" for r in records) or None,
    }


def benefit_keys(B: list[dict], profiles: dict, statuses=("offered",)):
    """Сгруппировать записи B по ключу (профиль, вуз, год)."""
    keys: dict[tuple, list[dict]] = defaultdict(list)
    stats = Counter()
    for prog in B:
        if prog["status"] not in statuses:
            continue
        for r in prog["prinimaemye_olimpiady"]:
            p = profiles.get(r["olympiad_id"])
            if p is None:
                stats["benefit_unknown_profile"] += 1
                continue
            if not level_ok(p["level"], r["min_level_required"], p["id"].startswith("vsosh-")):
                stats["benefit_level_filtered"] += 1
                continue
            keys[(r["olympiad_id"], prog["vuz_id"], prog["admission_year"])].append(r)
    return keys, stats


def doubtful_keys(B: list[dict]) -> set[tuple]:
    """Ключи (профиль, вуз, год), где льгота сомнительна хоть на одной программе."""
    return {(oid, prog["vuz_id"], prog["admission_year"]) for prog in B if prog["status"] == "offered"
            for oid in doubtful_olympiads(prog["napravlenie_code"], prog["prinimaemye_olimpiady"])}


def build_benefits(seed: Seed, B: list[dict], short: dict[str, str]):
    keys, stats = benefit_keys(B, seed.profiles)
    seed.stats.update(stats)
    doubts = doubtful_keys(B)
    for (pid, vuz, year) in sorted(keys):
        agg = aggregate_key(keys[(pid, vuz, year)], (pid, vuz, year) in doubts)
        src = None
        if not agg["demo"]:
            src = add_source(seed, agg["src_url"], agg["src_page"], "rules",
                             f"{short[vuz]}: особые права победителей и призёров олимпиад, {year}",
                             agg["src_date"])
        seed.benefits.append({
            "id": f"{pid}__{vuz}__{year}__{agg['benefit']}",
            "olympiad_profile_id": pid, "university_id": vuz, "admission_year": year,
            "benefit": agg["benefit"], "extra_points": None, "ege_min": agg["ege_min"],
            "diploma_grades": agg["diploma_grades"], "note": agg["note"], "source_id": src,
        })
    seed.stats["benefit_keys"] = len(keys)


# ---------------------------------------------------------------------------
# Вузы, направления, предметы, олимпиады вне перечня
# ---------------------------------------------------------------------------


def build_universities(seed: Seed, vuzy: list[dict], A: list[dict], B: list[dict], manifest):
    scores = defaultdict(set)
    for prog in B:
        if prog["status"] != "offered":
            continue
        for r in prog["prinimaemye_olimpiady"]:
            if r["ege_confirm_min_score"] is not None:
                scores[prog["vuz_id"]].add(r["ege_confirm_min_score"])
    places = defaultdict(Counter)
    for a in A:
        if a["status"] == "offered" and a["napravlenie_name"]:
            places[a["vuz_id"]][base_name(a["napravlenie_name"])] += a["budget_places_2026"] or 0
    for v in vuzy:
        vid = v["vuz_id"]
        rules = sorted((s for s in manifest if s["vuz_id"] == vid and s["role"] == "rules"),
                       key=lambda s: ("порог" not in (s.get("note") or ""), s["url"]))
        rule = rules[0] if rules else None
        sc = sorted(scores[vid])
        ege_note = None
        if sc:
            ege_note = (f"от {sc[0]} баллов по профильному предмету" if len(sc) == 1
                        else f"от {sc[0]} до {sc[-1]} баллов по профильному предмету")
        top = sorted(places[vid].items(), key=lambda kv: (-kv[1], kv[0]))[:8]
        seed.universities.append({
            "id": vid, "short_name": UNIVERSITY_SHORT[vid], "name": v["name"],
            "city": v["city"], "directions": [n for n, _ in top], "ege_note": ege_note,
            "rules_url": safe_url(rule["url"]) if rule else (v["website"] or None),
            "rules_verified_at": rule["fetched_at"] if rule else None,
        })


def base_name(name: str) -> str:
    """«Экономика (профиль …)» и «Менеджмент — …)» -> название направления."""
    return re.split(r"\s+[—(]|;|\.\s|,\s*профиль", name)[0].strip()


def build_directions(seed: Seed, A: list[dict]):
    names = defaultdict(Counter)
    for a in A:
        if a["napravlenie_code"]:
            names[a["napravlenie_code"]][base_name(a["napravlenie_name"])] += 1
    for code, group, subjects in DIRECTIONS:
        name = min(names[code], key=lambda n: (len(n), n))
        seed.directions.append({
            "id": "napr-" + code.replace(".", "-"), "name": name,
            "subject_codes": subjects or GROUP_SUBJECTS[group],
        })


def build_other(seed: Seed):
    for o in OTHER_OLYMPIADS:
        oid = o["id"]
        pid = f"{oid}-{o['subject']}"
        city, region = OTHER_CITY[oid]
        seed.olympiads[oid] = {
            "id": oid, "name": o["name"], "organizer": OTHER_ORGANIZER, "kind": "other",
            "official_url": None, "format": None, "final_city": city, "final_region_code": region,
        }
        seed.profiles[pid] = {
            "id": pid, "olympiad_id": oid, "subject_code": o["subject"],
            "profile_slug": o["subject"], "profile_name": None, "level": None,
            "school_year": SCHOOL_YEAR, "grades_from": o["grades"][0],
            "grades_to": o["grades"][1], "source_id": None, "_c": {"etapy": None},
        }
        for vuz, points in o["benefits"]:
            seed.benefits.append({
                "id": f"{pid}__{vuz}__2026__extra_points", "olympiad_profile_id": pid,
                "university_id": vuz, "admission_year": 2026, "benefit": "extra_points",
                "extra_points": points, "ege_min": None, "diploma_grades": None,
                "note": "Баллы за индивидуальные достижения, не больше 10 в сумме",
                "source_id": None,
            })


def build() -> Seed:
    seed = Seed()
    C_doc = json.loads((DATA / "olimpiady_spravochnik.json").read_text(encoding="utf-8"))
    C = C_doc["olimpiady"]
    missing = load("missing_in_C.json", "missing_in_C")
    A = load("vuz_napravleniya.json", "vuz_napravleniya")
    B = load("vuz_napravlenie_olimpiady.json", "vuz_napravlenie_olimpiady")
    manifest = load("sources_manifest.json", "sources")
    vuzy = load("vuzy_catalog.json", "vuzy", SPEC)

    seed.subjects = [{"code": c, "name": n} for c, n in SUBJECTS]
    build_directions(seed, A)
    build_universities(seed, vuzy, A, B, manifest)
    perechen_title = (f"Перечень олимпиад школьников на {C_doc['_perechen_edition']} уч. г. "
                      f"({C_doc['_perechen_order']})")
    build_profiles(seed, C, missing, perechen_title)
    build_stages(seed)
    olympiad_format(seed)
    build_benefits(seed, B, UNIVERSITY_SHORT)
    seed.benefits.sort(key=lambda b: b["id"])
    keep_later_sources(seed)
    return seed


# ---------------------------------------------------------------------------
# Направления вузов и льготы по направлениям (0021)
# ---------------------------------------------------------------------------


@dataclass
class UniversityDirections:
    directions: list[dict] = field(default_factory=list)
    pairs: list[dict] = field(default_factory=list)
    benefits: list[dict] = field(default_factory=list)
    sources: dict[str, dict] = field(default_factory=dict)
    own_sources: list[str] = field(default_factory=list)  # нет в 0003 — откат их удаляет
    stats: Counter = field(default_factory=Counter)


def direction_id(code: str) -> str:
    return "napr-" + code.replace(".", "-")


HYPHEN_PREFIXES = {"бизнес"}  # у них дефис свой, а не перенос строки


def _join_wrapped(m: re.Match) -> str:
    head, tail = m.group(1), m.group(2)
    return f"{head}-{tail}" if head.lower() in HYPHEN_PREFIXES else head + tail


def program_title(name: str) -> str:
    """Название программы для людей: без лишних пробелов, непарной скобки,
    переносов строки и хвостовой «;».

    В выгрузке МГУ хвост с профилем теряет открывающую скобку:
    «Менеджмент — Менеджмент в культуре)». В таблицах ВШЭ слово
    переносится по слогам: «Информацион- ная безопасность», а буква
    откалывается: «информационны х продуктов».
    """
    name = re.sub(r"\s+\)", ")", " ".join(name.split()))
    if name.endswith(")") and name.count(")") > name.count("("):
        name = name[:-1].rstrip()
    name = re.sub(r"(\w+)- (?!(?:и|или)\b)(\w)", _join_wrapped, name)
    # Буква, отколотая от слова: «информационны х продуктов».
    name = re.sub(r"(?<=[а-яё]) ([бгджзйлмнпртфхцчшщъыьэю])(?=[\s,.;»)]|$)", r"\1", name)
    return name.rstrip(" ;")


def program_labels(programs: list[dict]) -> dict[str, tuple[str, str | None]]:
    """Подписи программ одного направления вуза: название, а если оно на
    направлении не одно — ещё и факультет или кампус («Физика» ВШЭ в Москве и
    Петербурге, ПМИ МГУ на ВМК и в филиалах). Факультет из кодов укрупнённых
    групп (так в выгрузке КФУ) подписью не служит."""
    names = Counter(program_title(p["program_name"]) for p in programs)
    out = {}
    for p in programs:
        name = program_title(p["program_name"])
        faculty = (p.get("faculty") or "").strip()
        usable = names[name] > 1 and faculty and not faculty[0].isdigit()
        out[p["program_id"]] = (name, faculty if usable else None)
    return out


def label_text(label: tuple[str, str | None]) -> str:
    name, faculty = label
    return f"{name} ({faculty})" if faculty else name


GRANT_RANK = {"БВИ": 2, "100_ballov": 1}
GRANT_LABEL = {
    (2, 2): "БВИ", (2, 1): "БВИ победителю, 100 баллов призёру", (2, 0): "БВИ только победителю",
    (1, 1): "100 баллов", (1, 0): "100 баллов только победителю",
}
NO_GRANT = (0, 0)


def program_grant(records: list[dict]) -> tuple[int, int]:
    """Что программа даёт победителю и призёру: 2 — БВИ, 1 — 100 баллов, 0 —
    ничего. Победитель получает не меньше призёра — как в aggregate_key."""
    win = max((GRANT_RANK[r["benefit_type"]] for r in records), default=0)
    pri = max((GRANT_RANK[r["benefit_type"]] for r in records
               if r["diploma_status"] == "prizyor"), default=0)
    return win, pri


def quoted(label: tuple[str, str | None]) -> str:
    name, faculty = label
    s = "«" + name.replace("«", "„").replace("»", "“") + "»"
    return f"{s} ({faculty})" if faculty else s


def listed(labels: list[tuple[str, str | None]]) -> str:
    """«A», «A» и «B», «A», «B» и «C», а длиннее — «A», «B» и ещё N программах."""
    shown = [quoted(x) for x in labels]
    if len(shown) > 3:
        return f"{shown[0]}, {shown[1]} и ещё {len(shown) - 2} программах"
    return shown[0] if len(shown) == 1 else ", ".join(shown[:-1]) + " и " + shown[-1]


def varies_note(grants: list[tuple[tuple[str, str | None], tuple[int, int]]]) -> str | None:
    """Пояснение к льготе направления, если программы дают разное.

    Сводная льгота строки — лучшее по всем программам, поэтому пояснение
    перечисляет программы, где хуже. Если хуже только тем, что льготы нет, и
    программ с льготой меньше, называются они: «льгота только на «A»».
    """
    by_grant: dict[tuple, set] = defaultdict(set)
    for label, grant in grants:
        by_grant[grant].add(label)
    if len(by_grant) < 2:
        return None
    head = (max(g[0] for g in by_grant), max(g[1] for g in by_grant))
    groups = {g: sorted(ls, key=lambda x: (x[0], x[1] or "")) for g, ls in by_grant.items()}
    worse = sorted((g for g in groups if g != head), reverse=True)
    if worse == [NO_GRANT] and len(groups[head]) < len(groups[NO_GRANT]):
        return "Зависит от программы: льгота только на " + listed(groups[head])
    parts = [f"на {listed(groups[g])} льготы нет" if g == NO_GRANT
             else f"на {listed(groups[g])} — {GRANT_LABEL[g]}" for g in worse]
    return "Зависит от программы: " + ", ".join(parts)


def build_university_directions(seed: Seed) -> UniversityDirections:
    """Направления вузов и льготы по направлениям — сид 0021.

    * Программа — program_id. В A она повторяется на каждую профильную
      группу, поэтому число программ и места считаются по program_id.
      Программа со статусом not_offered хоть в одном датасете не считается.
    * Пара вуз–направление — программы вуза с этим кодом. status = offered,
      если хоть одна программа пары в B offered (её правила приёма
      разобраны), иначе to_check: программы есть, льготы уточняются. Так
      вышли НГУ 01.03.01–01.03.03 и СПбГУ 06.03.01: в A программы есть, в B
      все to_check. Пара только из A — тоже to_check.
    * budget_places — сумма известных мест программ пары; NULL, если вуз не
      опубликовал места ни одной.
    * Направления — все коды пар. Шестнадцать направлений онбординга уже
      есть в 0003, им проставляются только группы. Новым имя — как в
      build_directions, предметы — ключевые предметы их групп по порядку
      групп, группы — по алфавиту.
    * Льгота — как build_benefits, но ключ (профиль, вуз, направление, год) и
      только offered-программы направления. varies — программы направления
      дают разное (program_grant): где-то льготы нет, льгота другого вида или
      призёру не то, что на другой программе. Тогда note начинается с
      «Зависит от программы: …» (varies_note).
    """
    A = load("vuz_napravleniya.json", "vuz_napravleniya")
    B = load("vuz_napravlenie_olimpiady.json", "vuz_napravlenie_olimpiady")
    ud = UniversityDirections()

    programs: dict[str, dict] = {}
    excluded: set[str] = set()
    for rows, in_b in ((A, False), (B, True)):
        for r in rows:
            pid = r["program_id"]
            if not pid or not r["napravlenie_code"]:
                continue  # у вуза нет программ группы: not_offered без программы
            if r["status"] == "not_offered":
                excluded.add(pid)
            p = programs.setdefault(pid, {
                "program_id": pid, "vuz": r["vuz_id"], "code": r["napravlenie_code"],
                "program_name": r["program_name"], "faculty": r["faculty"],
                "places": None, "group_places": False,
                "groups": set(), "b_status": None, "year": r["admission_year"], "records": [],
            })
            p["groups"].add(r["profile_group"])
            if p["places"] is None:
                # Число есть, но демо — места не программы, а её конкурсной
                # группы (МФТИ): одно число на все программы группы.
                p["places"], p["group_places"] = r["budget_places_2026"], not in_b and r["is_demo"]
            if in_b:
                p["b_status"], p["records"] = r["status"], r["prinimaemye_olimpiady"]
    by_pair: dict[tuple, list[dict]] = defaultdict(list)
    for pid in sorted(programs):
        if pid not in excluded:
            p = programs[pid]
            by_pair[(p["vuz"], p["code"])].append(p)

    names = defaultdict(Counter)
    for a in A:
        if a["napravlenie_code"]:
            names[a["napravlenie_code"]][base_name(a["napravlenie_name"])] += 1
    groups: dict[str, set] = defaultdict(set)
    for (_, code), progs in by_pair.items():
        for p in progs:
            groups[code] |= p["groups"]
    onboarding = {d["id"]: d for d in seed.directions}
    for code in sorted(groups):
        did, gs = direction_id(code), sorted(groups[code])
        if did in onboarding:
            name, subjects = onboarding[did]["name"], onboarding[did]["subject_codes"]
        else:
            name = min(names[code], key=lambda n: (len(n), n))
            subjects = []
            for g in gs:
                subjects += [s for s in GROUP_SUBJECTS[g] if s not in subjects]
        ud.directions.append({"id": did, "code": code, "name": name, "subject_codes": subjects,
                              "groups": gs, "onboarding": did in onboarding})

    for (vuz, code) in sorted(by_pair):
        progs = by_pair[(vuz, code)]
        labels = program_labels(progs)
        places, groups_seen = [], set()
        for p in progs:
            if p["places"] is None:
                continue
            if p["group_places"]:
                if (p["faculty"], p["places"]) in groups_seen:
                    continue
                groups_seen.add((p["faculty"], p["places"]))
            places.append(p["places"])
        offered = [p for p in progs if p["b_status"] == "offered"]
        ud.pairs.append({
            "university_id": vuz, "direction_id": direction_id(code),
            "status": "offered" if offered else "to_check", "programs": len(progs),
            "budget_places": sum(places) if places else None,
            "program_names": sorted({label_text(labels[p["program_id"]]) for p in progs}),
        })
        build_direction_benefits(ud, seed, vuz, code, offered, labels)
    # Самый частый источник записей направления бывает не тем, что у вуза в
    # целом (у ВШЭ правила — постранично): такие источники есть только здесь.
    ud.own_sources = sorted(k for k in ud.sources if k not in seed.sources)
    ud.stats["direction_benefits"] = len(ud.benefits)
    ud.stats["direction_benefits_varies"] = sum(b["varies"] for b in ud.benefits)
    return ud


def build_direction_benefits(ud: UniversityDirections, seed: Seed, vuz: str, code: str,
                             offered: list[dict], labels: dict):
    keys: dict[tuple, dict[str, list]] = defaultdict(lambda: defaultdict(list))
    for p in offered:
        for r in p["records"]:
            prof = seed.profiles.get(r["olympiad_id"])
            if prof is None:
                ud.stats["benefit_unknown_profile"] += 1
                continue
            if not level_ok(prof["level"], r["min_level_required"], prof["id"].startswith("vsosh-")):
                ud.stats["benefit_level_filtered"] += 1
                continue
            keys[(r["olympiad_id"], p["year"])][p["program_id"]].append(r)
    for (pid, year) in sorted(keys):
        per = keys[(pid, year)]
        progs = [p for p in offered if p["year"] == year]
        doubtful = any(pid in doubtful_olympiads(p["code"], p["records"]) for p in progs)
        agg = aggregate_key([r for p in progs for r in per.get(p["program_id"], [])], doubtful)
        vnote = varies_note([(labels[p["program_id"]], program_grant(per.get(p["program_id"], [])))
                             for p in progs])
        src = None
        if not agg["demo"]:
            src = add_source(ud, agg["src_url"], agg["src_page"], "rules",
                             f"{UNIVERSITY_SHORT[vuz]}: особые права победителей и призёров олимпиад, {year}",
                             agg["src_date"])
        notes = [vnote, agg["note"]]
        if vnote and "100 баллов" in vnote and agg["subject100"] not in (agg["note"] or ""):
            notes.append(agg["subject100"])
        ud.benefits.append({
            "olympiad_profile_id": pid, "university_id": vuz, "direction_id": direction_id(code),
            "admission_year": year, "benefit": agg["benefit"], "ege_min": agg["ege_min"],
            "diploma_grades": agg["diploma_grades"],
            "note": ". ".join(n for n in notes if n) or None,
            "varies": vnote is not None, "source_id": src,
        })


# ---------------------------------------------------------------------------
# SQL
# ---------------------------------------------------------------------------


def build_demo() -> Seed:
    demo = Seed()
    build_other(demo)
    build_stages(demo)
    olympiad_format(demo)
    return demo


def lit(v) -> str:
    if v is None:
        return "NULL"
    if isinstance(v, bool):
        return "true" if v else "false"
    if isinstance(v, int):
        return str(v)
    if isinstance(v, list):
        if not v:
            return "'{}'"
        if all(isinstance(x, int) for x in v):
            return "ARRAY[" + ",".join(str(x) for x in v) + "]"
        return "ARRAY[" + ",".join(lit(x) for x in v) + "]"
    return "'" + str(v).replace("'", "''") + "'"


def insert(table: str, cols: list[str], rows: list[dict], key: str = "id",
           casts: dict[str, str] | None = None) -> str:
    if not rows:
        return ""
    casts = casts or {}

    def value(r, c):
        s = lit(r[c])
        return f"{s}::{casts[c]}" if c in casts and r[c] is not None else s

    values = ",\n".join("  (" + ", ".join(value(r, c) for c in cols) + ")" for r in rows)
    updates = ",\n  ".join(f"{c} = EXCLUDED.{c}" for c in cols if c != key)
    return (f"INSERT INTO {table} ({', '.join(cols)}) VALUES\n{values}\n"
            f"ON CONFLICT ({key}) DO UPDATE SET\n  {updates};\n")


def render(seed: Seed) -> str:
    olympiads = [seed.olympiads[k] for k in sorted(seed.olympiads)]
    profiles = [seed.profiles[k] for k in sorted(seed.profiles)]
    sources = [seed.sources[k] for k in sorted(seed.sources)]
    parts = [
        "-- Контент «Траектории»: предметы, направления, вузы, источники, олимпиады,\n"
        "-- профили, этапы и льготы.\n"
        "--\n"
        "-- ФАЙЛ СГЕНЕРИРОВАН: datasets/parser/build_seed.py (make seed). Руками не править —\n"
        "-- поменяйте датасет или генератор и перегенерируйте.\n"
        "--\n"
        f"-- Строк: предметы {len(seed.subjects)}, направления {len(seed.directions)}, "
        f"вузы {len(seed.universities)},\n"
        f"-- источники {len(sources)}, олимпиады {len(olympiads)}, профили {len(profiles)}, "
        f"этапы {len(seed.stages)}, льготы {len(seed.benefits)}.\n"
        "--\n"
        "-- Демонстрационные данные помечены в самих строках: stages.is_demo, льготы и\n"
        "-- этапы без source_id, источники без verified_at. API по ним отдаёт\n"
        "-- stages_are_demo и source: null, а интерфейс — «данные уточняются».\n",
        "-- +goose Up\n",
        insert("subjects", ["code", "name"], seed.subjects, key="code"),
        insert("directions", ["id", "name", "subject_codes"], seed.directions,
               casts={"subject_codes": "text[]"}),
        insert("universities", ["id", "short_name", "name", "city", "directions", "ege_note",
                                "rules_url", "rules_verified_at"], seed.universities,
               casts={"directions": "text[]", "rules_verified_at": "date"}),
        insert("sources", ["id", "kind", "title", "url", "verified_at"], sources,
               casts={"verified_at": "date"}),
        insert("olympiads", ["id", "name", "organizer", "kind", "official_url", "format",
                             "final_city", "final_region_code"], olympiads),
        insert("olympiad_profiles", ["id", "olympiad_id", "subject_code", "profile_slug",
                                     "profile_name", "level", "school_year", "grades_from",
                                     "grades_to", "source_id"], profiles),
        insert("stages", ["id", "olympiad_profile_id", "kind", "title", "starts_at", "ends_at",
                          "deadline_at", "is_online", "is_demo", "source_id"], seed.stages,
               casts={"starts_at": "timestamptz", "ends_at": "timestamptz",
                      "deadline_at": "timestamptz"}),
        insert("benefits", ["id", "olympiad_profile_id", "university_id", "admission_year",
                            "benefit", "extra_points", "ege_min", "diploma_grades", "note",
                            "source_id"], seed.benefits, casts={"diploma_grades": "int[]"}),
        "-- +goose Down\n"
        "-- Откат удаляет весь контент. Если на него уже ссылаются пользовательские\n"
        "-- данные (трекер, вузы траектории), откат упадёт на внешнем ключе — так и надо.\n"
        "DELETE FROM benefits;\nDELETE FROM stages;\nDELETE FROM olympiad_profiles;\n"
        "DELETE FROM olympiads;\nDELETE FROM sources;\nDELETE FROM universities;\n"
        "DELETE FROM directions;\nDELETE FROM subjects;\n",
    ]
    return "\n".join(p for p in parts if p)


def render_demo(demo: Seed) -> str:
    ids = ", ".join(lit(k) for k in sorted(demo.olympiads))
    return "\n".join([
        "-- Вымышленные олимпиады вне перечня — чтобы локально показать блок F16.\n"
        "-- Накатываются только на локальный стенд (seed-demo) и в CI, в прод не попадают.\n"
        "--\n"
        "-- ФАЙЛ СГЕНЕРИРОВАН: datasets/parser/build_seed.py (make seed). Руками не править.\n",
        "-- +goose Up\n",
        insert("olympiads", ["id", "name", "organizer", "kind", "official_url", "format",
                             "final_city", "final_region_code"],
               [demo.olympiads[k] for k in sorted(demo.olympiads)]),
        insert("olympiad_profiles", ["id", "olympiad_id", "subject_code", "profile_slug",
                                     "profile_name", "level", "school_year", "grades_from",
                                     "grades_to", "source_id"],
               [demo.profiles[k] for k in sorted(demo.profiles)]),
        insert("stages", ["id", "olympiad_profile_id", "kind", "title", "starts_at", "ends_at",
                          "deadline_at", "is_online", "is_demo", "source_id"], demo.stages,
               casts={"starts_at": "timestamptz", "ends_at": "timestamptz",
                      "deadline_at": "timestamptz"}),
        insert("benefits", ["id", "olympiad_profile_id", "university_id", "admission_year",
                            "benefit", "extra_points", "ege_min", "diploma_grades", "note",
                            "source_id"], sorted(demo.benefits, key=lambda b: b["id"]),
               casts={"diploma_grades": "int[]"}),
        "-- Вставка этапов и льгот рождает события изменения (0006) — о демо не уведомляем.\n"
        "DELETE FROM content_changes WHERE notified_at IS NULL AND entity_id LIKE 'other-%';\n",
        "-- +goose Down\n"
        f"DELETE FROM olympiads WHERE id IN ({ids});\n",
    ])


def values(rows: list[dict], cols: list[str], casts: dict[str, str] | None = None) -> str:
    casts = casts or {}

    def value(r, c):
        s = lit(r[c])
        return f"{s}::{casts[c]}" if c in casts and r[c] is not None else s

    return ",\n".join("  (" + ", ".join(value(r, c) for c in cols) + ")" for r in rows)


def insert_select(table: str, cols: list[str], rows: list[dict], key: list[str],
                  exists: list[str], casts: dict[str, str]) -> str:
    """INSERT … SELECT из VALUES: строка, чей вуз или профиль исчез из базы,
    пропускается, а не роняет миграцию."""
    if not rows:
        return ""
    sel = ", ".join(f"v.{c}::{casts[c]}" if c in casts else f"v.{c}" for c in cols)
    updates = ",\n  ".join(f"{c} = EXCLUDED.{c}" for c in cols if c not in key)
    return (f"INSERT INTO {table} ({', '.join(cols)})\nSELECT {sel}\nFROM (VALUES\n"
            f"{values(rows, cols)}\n) AS v({', '.join(cols)})\n"
            f"WHERE " + "\n  AND ".join(exists) + "\n"
            f"ON CONFLICT ({', '.join(key)}) DO UPDATE SET\n  {updates};\n")


def render_university_directions(ud: UniversityDirections) -> str:
    onboarding = [d for d in ud.directions if d["onboarding"]]
    new = [d for d in ud.directions if not d["onboarding"]]
    sources = [ud.sources[k] for k in sorted(ud.sources)]
    to_check = sum(p["status"] == "to_check" for p in ud.pairs)
    varies = sum(b["varies"] for b in ud.benefits)
    return "\n".join([
        "-- Направления вузов и льготы олимпиад по направлениям (схема — 0020).\n"
        "--\n"
        "-- ФАЙЛ СГЕНЕРИРОВАН: datasets/parser/build_seed.py (make seed) из датасетов A\n"
        "-- (vuz_napravleniya.json) и B (vuz_napravlenie_olimpiady.json). Руками не править —\n"
        "-- поменяйте датасет или генератор и перегенерируйте. Правила сборки — в\n"
        "-- build_university_directions.\n"
        "--\n"
        f"-- Строк: направления {len(ud.directions)} (онбординга {len(onboarding)}, "
        f"новых {len(new)}), пары вуз–направление {len(ud.pairs)}\n"
        f"-- (to_check {to_check}), источники {len(sources)}, льготы по направлениям "
        f"{len(ud.benefits)} (разные у программ {varies}).\n"
        "--\n"
        "-- Направлениям онбординга проставляются только группы: имя и предметы — из 0003.\n"
        "-- Вставки идемпотентны: повторный прогон ничего не меняет. Триггеров на новых\n"
        "-- таблицах нет, событий «изменились льготы» миграция не рождает.\n"
        "--\n"
        "-- Откат удаляет этот контент, а каскадом — и выбор пользователей по нему: все\n"
        "-- строки trajectory_university_directions и строки trajectory_directions с новыми\n"
        "-- направлениями (устаревшее trajectories.direction_id с ними обнуляется).\n"
        f"-- Из источников удаляются только {len(ud.own_sources)} своих, которых нет в 0003, "
        "и лишь если\n"
        "-- на них больше ничто не ссылается.\n",
        "-- +goose Up\n",
        "UPDATE directions d SET groups = v.groups\nFROM (VALUES\n"
        + values(onboarding, ["id", "groups"], {"groups": "text[]"})
        + "\n) AS v(id, groups)\nWHERE d.id = v.id;\n",
        "INSERT INTO directions (id, name, subject_codes, groups, onboarding) VALUES\n"
        + values(new, ["id", "name", "subject_codes", "groups", "onboarding"],
                 {"subject_codes": "text[]", "groups": "text[]"})
        + "\nON CONFLICT (id) DO NOTHING;\n",
        "INSERT INTO sources (id, kind, title, url, verified_at) VALUES\n"
        + values(sources, ["id", "kind", "title", "url", "verified_at"], {"verified_at": "date"})
        + "\nON CONFLICT (id) DO NOTHING;\n",
        insert_select(
            "university_directions",
            ["university_id", "direction_id", "status", "programs", "budget_places", "program_names"],
            ud.pairs, key=["university_id", "direction_id"],
            exists=["EXISTS (SELECT 1 FROM universities u WHERE u.id = v.university_id)"],
            casts={"programs": "smallint", "budget_places": "integer", "program_names": "text[]"}),
        insert_select(
            "direction_benefits",
            ["olympiad_profile_id", "university_id", "direction_id", "admission_year", "benefit",
             "ege_min", "diploma_grades", "note", "varies", "source_id"],
            ud.benefits, key=["olympiad_profile_id", "university_id", "direction_id", "admission_year"],
            exists=["EXISTS (SELECT 1 FROM olympiad_profiles p WHERE p.id = v.olympiad_profile_id)",
                    "EXISTS (SELECT 1 FROM university_directions ud\n"
                    "              WHERE ud.university_id = v.university_id AND ud.direction_id = v.direction_id)"],
            casts={"admission_year": "smallint", "ege_min": "smallint", "diploma_grades": "int[]",
                   "note": "text", "source_id": "text"}),
        "-- +goose Down\n"
        "DELETE FROM direction_benefits;\n"
        "-- Каскадом уходит trajectory_university_directions.\n"
        "DELETE FROM university_directions;\n"
        "-- Каскадом уходят строки trajectory_directions с новыми направлениями.\n"
        "DELETE FROM directions WHERE NOT onboarding;\n"
        "UPDATE directions SET groups = '{}';\n"
        + (("DELETE FROM sources s WHERE s.id IN (\n"
            + ",\n".join("  " + ", ".join(lit(k) for k in ud.own_sources[i:i + 6])
                         for i in range(0, len(ud.own_sources), 6))
            + ")\n"
            "  AND NOT EXISTS (SELECT 1 FROM benefits b WHERE b.source_id = s.id)\n"
            "  AND NOT EXISTS (SELECT 1 FROM stages st WHERE st.source_id = s.id)\n"
            "  AND NOT EXISTS (SELECT 1 FROM olympiad_profiles p WHERE p.source_id = s.id);\n")
           if ud.own_sources else ""),
    ])


def main():
    seed = build()
    OUT.write_text(render(seed), encoding="utf-8")
    DEMO_OUT.write_text(render_demo(build_demo()), encoding="utf-8")
    ud = build_university_directions(seed)
    UD_OUT.write_text(render_university_directions(ud), encoding="utf-8")
    counts = {
        "subjects": len(seed.subjects), "directions": len(seed.directions),
        "universities": len(seed.universities), "sources": len(seed.sources),
        "olympiads": len(seed.olympiads), "profiles": len(seed.profiles),
        "stages": len(seed.stages), "benefits": len(seed.benefits),
    }
    print(f"{OUT.relative_to(REPO)}: {counts}")
    print(dict(sorted(seed.stats.items())))
    ud_counts = {
        "directions": len(ud.directions), "pairs": len(ud.pairs),
        "to_check": sum(p["status"] == "to_check" for p in ud.pairs),
        "sources": len(ud.sources), "direction_benefits": len(ud.benefits),
    }
    print(f"{UD_OUT.relative_to(REPO)}: {ud_counts}")
    print(dict(sorted(ud.stats.items())))


if __name__ == "__main__":
    main()
