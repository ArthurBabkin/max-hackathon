#!/usr/bin/env python3
"""Датасет A — реальные образовательные программы вузов по 4 профильным группам.

Единица учёта — программа (program_id), а не код ОКСО: у одного кода бывает
несколько программ на разных факультетах и кампусах, и списки принимаемых
олимпиад у них разные (п. 0.8 спеки).
"""
import json, re, sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))
from classify import GROUPS, classify
from common import (ADMISSION_YEAR, CODE_RE, DATA, ROOT, TODAY, clean, fetched_date,
                    html_rows, load_html, load_pages, meta, slugify, to_int, write_json)

LEVEL_BY_CODE = lambda code: "specialitet" if code[3:5] == "05" else "bakalavriat"
# Филиал — отдельный вуз со своим приёмом, в каталоге только головные вузы.
# Шапка секции филиала называет его «филиалом» или юрлицом головного вуза
# («Набережночелнинский институт ФГАОУ ВО …»): свои институты так не пишут.
BRANCH_RE = re.compile(r"филиал|ФГАОУ", re.I)


def _src(vuz_id, filename, page):
    m = meta(vuz_id, filename)
    return {"source_url": m["url"], "source_page": page}


# ---------------------------------------------------------------- МГУ
def parse_msu():
    """kcp_bak.pdf: строки-заголовки факультетов, затем [код, название, бюджет, платно, ВИ]."""
    out, faculty = [], None
    for pg in load_pages("msu", "kcp__kcp_bak.pdf"):
        for table in pg["tables"]:
            for row in table:
                cells = [clean(c) for c in row]
                if not any(cells):
                    continue
                first = cells[0]
                # Заголовок секции опознаём по форме строки, а не по словарю слов:
                # иначе теряются «Филиал МГУ в г. Грозном» и подобные, и их программы
                # молча приписываются предыдущему факультету.
                if first and not CODE_RE.fullmatch(first) and not any(cells[2:4]):
                    if len(first) > 5 and not first[0].isdigit():
                        faculty = first
                    continue
                if not CODE_RE.fullmatch(first) or BRANCH_RE.search(faculty or ""):
                    continue
                name = re.sub(r'^(Направление подготовки|Специальность)\s*', "", cells[1])
                name = re.sub(r'\s*\((очная|заочная|очно).*$', "", name, flags=re.I)
                name = re.sub(r'\s*\(образовательная программа\s*', " — ", name)
                name = name.strip('"«» ').replace('"', "")
                out.append({
                    "program_name": name or cells[1], "faculty": faculty or "не указан",
                    "napravlenie_code": first, "napravlenie_name": name or cells[1],
                    "education_level": LEVEL_BY_CODE(first),
                    "budget_places_2026": to_int(cells[2]),
                    **_src("msu", "kcp__kcp_bak.pdf", pg["page"]),
                })
    return out


# ---------------------------------------------------------------- СПбГУ
def parse_spbu():
    """Одна большая таблица: строки-заголовки УГСН, затем строки программ."""
    out = []
    pg = load_pages("spbu", "programs__bak_spec_prog_VI_2026.pdf")[0]
    for table in pg["tables"]:
        for row in table:
            cells = [clean(c) for c in row]
            code_cell, prog = cells[0] if cells else "", cells[1] if len(cells) > 1 else ""
            m = CODE_RE.search(code_cell)
            if not (m and prog):
                continue
            code = m.group(1)
            napr = clean(code_cell.replace(code, "")).strip("—- ")
            out.append({
                "program_name": prog, "faculty": f"СПбГУ, УГСН {code[:2]}.00.00",
                "napravlenie_code": code, "napravlenie_name": napr or prog,
                "education_level": LEVEL_BY_CODE(code),
                "budget_places_2026": to_int(cells[3]) if len(cells) > 3 else None,
                **_src("spbu", "programs__bak_spec_prog_VI_2026.pdf", pg["page"]),
            })
    return out


# ---------------------------------------------------------------- ВШЭ
# Нижний Новгород, Пермь и Петербург — филиалы со своим приёмом, не ВШЭ.
HSE_CAMPUS = {"programs__1120607478": "Москва"}


def parse_hse():
    """Приложение 1 — московский кампус.

    Строка программы опознаётся по порядковому номеру в первой ячейке;
    строки без номера — это продолжение списка вступительных испытаний
    той же программы, и раньше они утекали в датасет как «программы»
    с названиями вроде «алгебра и начала математического анализа».
    """
    out = []
    for fname, campus in HSE_CAMPUS.items():
        for code, napr, name, page in hse_rows(load_pages("hse", fname)):
            out.append({
                "program_name": name, "faculty": f"НИУ ВШЭ — {campus}",
                "napravlenie_code": code, "napravlenie_name": napr or name,
                "education_level": LEVEL_BY_CODE(code), "budget_places_2026": None,
                "budget_is_demo": True, "campus": campus, **_src("hse", fname, page),
            })
    return out


def hse_rows(pages: list[dict]) -> list[tuple[str, str | None, str, int]]:
    """Программы приложения ВШЭ: (код, направление, программа, страница).

    Название — ячейка с заглавной буквы: строчные рядом — предметы ВИ. Если
    у строки с номером названия нет, оно перенесено на строку ниже, бывает —
    на следующую страницу (Пермь, 38.03.05). Раздел «ОЧНО-ЗАОЧНАЯ ФОРМА
    ОБУЧЕНИЯ» не читается: там платные места и приём после колледжа,
    олимпиадных льгот нет.
    """
    out, code, napr, pending = [], None, None, None
    for pg in pages:
        for table in pg["tables"]:
            for row in table:
                cells = [clean(c) for c in row]
                joined = " ".join(cells)
                if "ЗАОЧНАЯ ФОРМА ОБУЧЕНИЯ" in joined.upper():
                    return out
                m = re.search(r"(?:Направлени\w*\s+подготовки|Специальност\w*)\s+(\d{2}\.\d{2}\.\d{2})\s*(.*)", joined)
                if m:
                    code, pending = m.group(1), None
                    napr = clean(re.split(r"\s{2,}", m.group(2))[0])
                    continue
                if not code or not cells:
                    continue
                name = next((c for c in cells[1:6] if len(c) > 4 and not c.isdigit() and c[0].isupper()), "")
                if cells[0].isdigit():
                    pending = None if name else (code, napr, pg["page"])
                    if name:
                        out.append((code, napr, name, pg["page"]))
                elif pending and name:
                    out.append((*pending[:2], name, pending[2]))
                    pending = None
    return out


# ---------------------------------------------------------------- МФТИ
MIPT_SCHOOLS = ("ФРКТ", "ЛФИ", "ФАКТ", "ПИШ ФАЛТ", "ФЭФМ", "ФПМИ", "ФБМФ",
                "ВШБИ", "КНТ", "ФБВТ", "ВШПИ", "ВШМ")


def parse_mipt():
    """План приёма: строка с кодом — итог по направлению, следующие строки несут
    физтех-школу и программу. Школа важна: БВИ у МФТИ даётся не «в вуз»,
    а в конкретную физтех-школу, и именно она связывает льготу с программой."""
    out, code, napr = [], None, None
    for cells in html_rows(load_html("mipt", "programs__2026_places")):
        if not cells:
            continue
        m = CODE_RE.match(cells[0])
        if m:
            code = m.group(1)
            napr = clean(cells[0][len(code):])
            continue
        school = cells[0]
        # ФБМФ и ВШБИ в плане приёма встречаются склеенными («ФБМФ/ ВШБИ»).
        if not any(school.startswith(x) for x in MIPT_SCHOOLS) or not code or len(cells) < 4:
            continue
        out.append({
            "program_name": clean(cells[1]) or napr, "faculty": school,
            "napravlenie_code": code, "napravlenie_name": napr or clean(cells[1]),
            "education_level": LEVEL_BY_CODE(code),
            # КЦП в плане приёма даны на направление целиком, а не на каждую
            # программу внутри него — помечаем как неточное.
            "budget_places_2026": to_int(cells[3]), "budget_is_demo": True,
            "source_url": "https://pk.mipt.ru/bachelor/2026_places/", "source_page": None,
        })
    return out


# ---------------------------------------------------------------- ИТМО
def parse_itmo():
    """Открытый JSON каталога программ (единственный у десяти вузов)."""
    raw = json.loads((ROOT / "snapshots" / "itmo" / "programs__bachelor.json").read_bytes())
    url = meta("itmo", "programs__bachelor.json")["url"]
    out = []
    for p in raw["pageProps"]["initialPrograms"]:
        for d in p.get("programs", []) or [{}]:
            code = clean(d.get("directionCode"))
            if not CODE_RE.fullmatch(code):
                continue
            napr = clean(d.get("directionOfEducation", "")).replace(code, "").strip()
            out.append({
                "program_name": clean(p["name"]), "faculty": "Университет ИТМО",
                "napravlenie_code": code, "napravlenie_name": napr or clean(p["name"]),
                "education_level": LEVEL_BY_CODE(code), "budget_places_2026": None,
                "budget_is_demo": True, "source_url": url, "source_page": None,
            })
    return out


# ---------------------------------------------------------------- НГУ
def parse_nsu():
    """Карточки программ: code / direction / name в span'ах."""
    html = load_html("nsu", "programs__bachelor")
    out = []
    for block in re.findall(r'<a href="#prog_.*?</a>', html, re.S):
        code = re.search(r'<span class="code">([^<]+)</span>', block)
        direction = re.search(r'<span class="direction">([^<]+)</span>', block)
        name = re.search(r'<span class="name"[^>]*>([^<]+)</span>', block)
        if not (code and direction):
            continue
        code = clean(code.group(1))
        if not CODE_RE.fullmatch(code):
            continue
        napr = clean(direction.group(1))
        prog = clean(name.group(1)) if name else napr
        out.append({
            "program_name": prog, "faculty": napr,
            "napravlenie_code": code, "napravlenie_name": napr,
            "education_level": LEVEL_BY_CODE(code), "budget_places_2026": None,
            "budget_is_demo": True,
            "source_url": "https://www.nsu.ru/n/education/programs/bachelor/", "source_page": None,
        })
    return out


# ---------------------------------------------------------------- КФУ
def parse_kfu():
    """План приёма: строки-шапки институтов, затем [код, направление,
    профиль(программа), уровень, программы СПО, ...], бюджет дальше.
    Со стр. 10 идут филиалы — Елабуга, Набережные Челны, Джизак."""
    out, faculty = [], None
    f = "programs__plan_priema_2026_2027-bakalavriat-speczialitet-1.pdf"
    for pg in load_pages("kfu", f):
        for table in pg["tables"]:
            for row in table:
                cells = [clean(c) for c in row]
                if cells and cells[0] and not cells[0][0].isdigit() and not any(cells[1:]):
                    faculty = cells[0]
                    continue
                if len(cells) < 6 or not CODE_RE.fullmatch(cells[0]) or BRANCH_RE.search(faculty or ""):
                    continue
                code = cells[0]
                out.append({
                    "program_name": cells[2] or cells[1], "faculty": faculty or "КФУ",
                    "napravlenie_code": code, "napravlenie_name": cells[1],
                    "education_level": "specialitet" if "специалитет" in cells[3].lower() else LEVEL_BY_CODE(code),
                    "budget_places_2026": to_int(cells[14]) if len(cells) > 14 else None,
                    **_src("kfu", f, pg["page"]),
                })
    return out


# ---------------------------------------------------------------- Иннополис
def parse_innopolis():
    """КЦП: только УГСН, без кодов конкретных направлений."""
    out = []
    f = "kcp___.pdf"
    for pg in load_pages("innopolis", f):
        for table in pg["tables"]:
            for row in table:
                cells = [clean(c) for c in row]
                if len(cells) < 3:
                    continue
                code = next((c for c in cells if re.fullmatch(r"\d{2}\.00\.00", c)), None)
                if not code:
                    continue
                name = next((c for c in cells if len(c) > 8 and not c[0].isdigit()), "")
                out.append({
                    "program_name": name, "faculty": "Университет Иннополис",
                    "napravlenie_code": code, "napravlenie_name": name,
                    "education_level": "bakalavriat",
                    "budget_places_2026": next((to_int(c) for c in cells if to_int(c)), None),
                    **_src("innopolis", f, pg["page"]),
                })
    return out


# ---------------------------------------------------------------- Сеченов / КГМУ
def _parse_code_name_table(vuz_id, fname, code_col=0, name_col=1, budget_col=None):
    out = []
    for pg in load_pages(vuz_id, fname):
        for table in pg["tables"]:
            for row in table:
                cells = [clean(c) for c in row]
                if len(cells) <= max(code_col, name_col):
                    continue
                code = cells[code_col]
                if not CODE_RE.fullmatch(code) or not cells[name_col]:
                    continue
                out.append({
                    "program_name": cells[name_col], "faculty": None,
                    "napravlenie_code": code, "napravlenie_name": cells[name_col],
                    "education_level": LEVEL_BY_CODE(code),
                    "budget_places_2026": to_int(cells[budget_col]) if budget_col and len(cells) > budget_col else None,
                    **_src(vuz_id, fname, pg["page"]),
                })
    return out


def parse_sechenov():
    """Приложение 1. Стр. 3–4 — Бакинский и Брянский филиалы; конкурсные
    группы для иностранных граждан — не для школьников из России."""
    f = "programs__Pravila-priema_2026_2027_BS_pril1_Perechen-programm.pdf"
    branch = {pg["page"] for pg in load_pages("sechenov", f) if "филиал" in pg["text"].lower()}
    rows = [r for r in _parse_code_name_table("sechenov", f)
            if r["source_page"] not in branch and "иностранных граждан" not in r["program_name"]]
    for r in rows:
        r["faculty"] = "Сеченовский Университет"
    return rows


def parse_kazan_gmu():
    out = []
    f = "programs__download"
    for pg in load_pages("kazan-gmu", f):
        for table in pg["tables"]:
            for row in table:
                cells = [clean(c) for c in row]
                code = next((c for c in cells if CODE_RE.fullmatch(c) or re.fullmatch(r"\d{2}\.00\.00", c)), None)
                if not code:
                    continue
                name = next((c for c in cells if len(c) > 6 and not c[0].isdigit()), "")
                if not name:
                    continue
                out.append({
                    "program_name": name, "faculty": "Казанский ГМУ",
                    "napravlenie_code": code, "napravlenie_name": name,
                    "education_level": LEVEL_BY_CODE(code),
                    "budget_places_2026": next((to_int(c) for c in cells if to_int(c)), None),
                    **_src("kazan-gmu", f, pg["page"]),
                })
    return out


PARSERS = {
    "msu": parse_msu, "spbu": parse_spbu, "hse": parse_hse, "mipt": parse_mipt,
    "itmo": parse_itmo, "nsu": parse_nsu, "kfu": parse_kfu,
    "innopolis": parse_innopolis, "sechenov": parse_sechenov, "kazan-gmu": parse_kazan_gmu,
}

# Документ, которым подтверждается отсутствие группы у вуза (полный перечень направлений).
NOT_OFFERED_PROOF = {
    "msu": ("msu", "kcp__kcp_bak.pdf", 1), "spbu": ("spbu", "programs__bak_spec_prog_VI_2026.pdf", 1),
    "hse": ("hse", "programs__1120607478", 1), "mipt": (None, "https://pk.mipt.ru/bachelor/2026_places/", None),
    "itmo": (None, "https://abit.itmo.ru/bachelor", None), "nsu": (None, "https://www.nsu.ru/n/education/programs/bachelor/", None),
    "kfu": ("kfu", "programs__plan_priema_2026_2027-bakalavriat-speczialitet-1.pdf", 1),
    "innopolis": ("innopolis", "kcp___.pdf", 1),
    "sechenov": ("sechenov", "programs__Pravila-priema_2026_2027_BS_pril1_Perechen-programm.pdf", 1),
    "kazan-gmu": ("kazan-gmu", "programs__download", 1),
}


def main() -> int:
    objects, seen_ids, stats = [], {}, {}
    for vuz_id, parser in PARSERS.items():
        programs, seen_raw = [], set()
        for p in parser():
            key = (clean(p["program_name"]).lower(), clean(p["faculty"] or "").lower(), p["napravlenie_code"])
            if key in seen_raw:
                continue
            seen_raw.add(key)
            programs.append(p)
        found = {g: 0 for g in GROUPS}
        for p in programs:
            for hit in classify(p["program_name"], p["napravlenie_code"], p["napravlenie_name"]):
                group = hit["profile_group"]
                found[group] += 1
                base = f"{vuz_id}__{slugify(p['program_name'])}"
                pid, n = base, 2
                key = (base, p.get("campus") or p.get("faculty"), p["napravlenie_code"],
                       p["budget_places_2026"])
                if pid in seen_ids and seen_ids[pid] != key:
                    pid = f"{base}-{p['napravlenie_code'].replace('.', '')}"
                if pid in seen_ids and seen_ids[pid] != key:
                    extra = slugify(p.get("campus") or p.get("faculty") or "", 18)
                    pid = f"{base}-{extra}" if extra else base
                    while pid in seen_ids and seen_ids[pid] != key:
                        pid, n = f"{base}-{n}", n + 1
                seen_ids[pid] = key
                objects.append({
                    "vuz_id": vuz_id, "profile_group": group, "status": "offered",
                    "program_id": pid, "program_name": p["program_name"], "faculty": p["faculty"],
                    "napravlenie_code": p["napravlenie_code"], "napravlenie_name": p["napravlenie_name"],
                    "education_level": p["education_level"],
                    "budget_places_2026": p["budget_places_2026"],
                    "admission_year": ADMISSION_YEAR,
                    "match_type": hit["match_type"], "matched_reason": hit["matched_reason"],
                    "source_url": p["source_url"], "source_page": p["source_page"],
                    "source_date": fetched_date(p["source_url"]),
                    "is_demo": bool(p.get("budget_is_demo") or p["budget_places_2026"] is None),
                })
        for group in GROUPS:
            if found[group] == 0:
                vid, ref, page = NOT_OFFERED_PROOF[vuz_id]
                url = meta(vid, ref)["url"] if vid else ref
                objects.append({
                    "vuz_id": vuz_id, "profile_group": group, "status": "not_offered",
                    "program_id": None, "program_name": None, "faculty": None,
                    "napravlenie_code": None, "napravlenie_name": None, "education_level": None,
                    "budget_places_2026": None, "admission_year": ADMISSION_YEAR,
                    "match_type": None, "matched_reason": None,
                    "source_url": url, "source_page": page, "source_date": fetched_date(p["source_url"]), "is_demo": False,
                })
        stats[vuz_id] = found
        print(f"  {vuz_id:11s} программ найдено: {len(programs):4d} | " +
              " ".join(f"{g}:{found[g]}" for g in GROUPS))

    write_json(DATA / "vuz_napravleniya.json", "vuz_napravleniya", objects,
               "Датасет A по data_format_spec_4.md. Единица учёта — образовательная программа. "
               "match_type: exact — код/название в ядре группы; adjacent — смежная область.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
