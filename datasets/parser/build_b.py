#!/usr/bin/env python3
"""Датасет B — принимаемые олимпиады по каждой программе из датасета A.

Три разные модели льгот у вузов, и схема обязана вместить все:
  named   — вуз перечисляет конкретные олимпиады (МГУ, СПбГУ, ВШЭ, МФТИ, ИТМО, Сеченов, КГМУ);
  profile — вуз задаёт профиль/предмет олимпиады, подходит любая перечневая с этим профилем (НГУ, КФУ, Иннополис);
  matrix  — МФТИ: льгота зависит ещё и от физтех-школы.
Одна запись = одна пара (olympiad_id, diploma_status): победитель и призёр
не сворачиваются, даже если льгота у них совпадает (п. 3 спеки).
"""
import json, re, sys
from collections import defaultdict
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))
from common import (ADMISSION_YEAR, CODE_RE, DATA, SNAP, TODAY, clean, fetched_date,
                    html_rows, load_html, load_pages, meta, to_int, write_json)
from olymp_match import (level_in_perechen, match_number, match_number_for_profile,
                         olympiad_id, profile_slug, vsosh_id, _index)

BVI = "БВИ"
HUNDRED = "100_ballov"
POB, PRIZ = "pobeditel", "prizyor"


def _benefit(text: str) -> str | None:
    t = clean(text).lower()
    if not t or t in ("—", "-", "нет"):
        return None
    if "без вступительных" in t or "бви" in t or "без экзаменов" in t:
        return BVI
    if "100" in t or "максимальное количество" in t or "приравн" in t:
        return HUNDRED
    if "дополнительн" in t and "балл" in t:
        return "dop_bally"
    return None


def _statuses(text: str) -> list[str]:
    """«Победителям и призерам» -> оба статуса; «Победителям» -> только победитель."""
    t = clean(text).lower()
    has_p = "побед" in t
    has_z = "призер" in t or "призёр" in t
    if has_p and has_z:
        return [POB, PRIZ]
    if has_z:
        return [PRIZ]
    if has_p:
        return [POB]
    return [POB, PRIZ]


def _grades(text: str) -> list[int] | None:
    found = sorted({int(g) for g in re.findall(r"\b(9|10|11)\b", clean(text))})
    return found or None


THRESHOLD = re.compile(r"(?:не\s+ниже|не\s+менее|от)\s+(\d{2,3})|(\d{2,3})\s*балл\w*\s*и\s*выше|(\d{2,3})\s*и\s*более")


def _score(*cells) -> int | None:
    """Ячейка часто содержит два числа: размер льготы (100) и порог её
    подтверждения (75/80/85). Нужен именно порог, поэтому сначала ищем его
    по формулировке, и только потом откатываемся на первое число."""
    for c in cells:
        t = clean(c)
        m = THRESHOLD.search(t)
        if m:
            return int(next(g for g in m.groups() if g))
    for c in cells:
        for m in re.findall(r"\b(\d{2,3})\b", clean(c)):
            v = int(m)
            if 40 <= v <= 100:
                return v
    return None


def _level(text: str) -> str | None:
    t = clean(text).upper().replace("III", "3").replace("II", "2").replace("I", "1")
    nums = sorted(int(x) for x in re.findall(r"[123]", t))
    return {1: "I", 2: "II", 3: "III"}[nums[-1]] if nums else None


# =============================================================== парсеры вузов
SPLIT_NAMES = re.compile(r",\s*(?=[А-ЯЁ«\"])")


def rows_msu():
    """olymp_benefits.pdf: секции по факультетам, 9 колонок.

    Модель гибридная. В колонке «Перечень олимпиад» либо перечислены конкретные
    олимпиады через запятую, либо стоит «*», что по сноске документа означает
    «Все олимпиады школьников соответствующего уровня, включенные в Перечень» —
    то есть профильная модель. Игнорировать «*» значило бы потерять большую
    часть льгот МГУ (489 строк из 627).
    """
    out, faculty, napravlenie = [], None, None
    f = "olymp_list__olymp_benefits.pdf"
    url = meta("msu", f)["url"]
    for pg in load_pages("msu", f):
        for table in pg["tables"]:
            for row in table:
                c = [clean(x) for x in row]
                if not any(c):
                    continue
                if c[0] and not any(c[3:]) and len(c[0]) > 8:
                    faculty = c[0]
                    napravlenie = None
                    continue
                if len(c) < 9 or c[0].lower().startswith("направление"):
                    continue
                if c[0]:
                    napravlenie = c[0]
                benefit = _benefit(c[8])
                if not benefit or not napravlenie:
                    continue
                common = {
                    "match": {"faculty": faculty, "napravlenie": napravlenie},
                    "profile": c[1], "level": _level(c[4]),
                    "statuses": _statuses(c[6]), "benefit": benefit,
                    "ege_subject": c[7] or c[2], "ege_score": 75,
                    "grades": _grades(c[5]), "page": pg["page"], "url": url,
                }
                listed = c[3]
                if not listed or listed == "*":
                    out.append({**common, "by_profile": True, "olympiad_name": None})
                    continue
                for name in SPLIT_NAMES.split(listed):
                    name = clean(name).strip("«»\" ")
                    if len(name) > 5:
                        out.append({**common, "olympiad_name": name})
    return out


def rows_msu_vsosh():
    """olymp_disciplines.pdf: предметы ВсОШ, дающие льготу, по направлениям.
    Лежит в отдельном файле от перечневых олимпиад — легко потерять целиком."""
    out, faculty, napravlenie = [], None, None
    f = "vsosh_list__olymp_disciplines.pdf"
    url = meta("msu", f)["url"]
    for pg in load_pages("msu", f):
        for table in pg["tables"]:
            for row in table:
                c = [clean(x) for x in row]
                if not any(c):
                    continue
                if c[0] and not any(c[1:]) and len(c[0]) > 8:
                    faculty, napravlenie = c[0], None
                    continue
                if len(c) < 3 or c[0].lower().startswith(("направление", "специальность")):
                    continue
                if c[0]:
                    napravlenie = c[0]
                subject = c[2]
                if not (napravlenie and subject) or not profile_slug(subject.lower()):
                    continue
                out.append({"match": {"faculty": faculty, "napravlenie": napravlenie},
                            "vsosh": True, "olympiad_name": None, "profile": subject.lower(),
                            "level": "ВсОШ", "statuses": [POB, PRIZ], "benefit": BVI,
                            "ege_subject": subject, "ege_score": None, "grades": None,
                            "page": pg["page"], "url": url, "score_is_demo": True})
    return out


def rows_hse_vsosh():
    """Приложение 1: направление -> предметы ВсОШ на БВИ и на 100 баллов."""
    out, program = [], None
    f = "vsosh_list__1133957386"
    url = meta("hse", f)["url"]
    for pg in load_pages("hse", f):
        for table in pg["tables"]:
            for row in table:
                c = [clean(x) for x in row]
                if len(c) < 4 or c[1].lower().startswith("направление"):
                    continue
                if c[1]:
                    program = c[1]
                if not program:
                    continue
                for col, benefit in ((2, BVI), (3, HUNDRED)):
                    subject = c[col]
                    if not subject or not profile_slug(subject.lower()):
                        continue
                    out.append({"match": {"program": program}, "vsosh": True,
                                "olympiad_name": None, "profile": subject.lower(),
                                "level": "ВсОШ", "statuses": [POB, PRIZ], "benefit": benefit,
                                "ege_subject": subject, "ege_score": None, "grades": None,
                                "page": pg["page"], "url": url, "score_is_demo": True})
    return out


def rows_spbu():
    """bac_spec_olymp_2_2026.pdf: 12 колонок, порог ЕГЭ отдельной колонкой."""
    out, prog, code = [], None, None
    f = "olymp_list__bac_spec_olymp_2_2026.pdf"
    url = meta("spbu", f)["url"]
    for pg in load_pages("spbu", f):
        for table in pg["tables"]:
            for row in table:
                c = [clean(x) for x in row]
                if len(c) < 12:
                    continue
                if c[0]:
                    m = CODE_RE.search(c[0])
                    code = m.group(1) if m else code
                if c[1]:
                    prog = c[1]
                benefit = _benefit(c[11])
                if not benefit or not prog:
                    continue
                name, profile = c[7], c[8]
                out.append({
                    "match": {"program": prog, "code": code},
                    "olympiad_name": name, "profile": profile, "level": _level(c[6]),
                    "statuses": _statuses(c[10]), "benefit": benefit,
                    "ege_subject": c[9], "ege_score": to_int(c[5]),
                    "grades": None, "page": pg["page"], "url": url,
                    "vsosh": name in ("", "—", "-", "–"), "vsosh_subject": c[9],
                })
    return out


HSE_OLYMP = {"olymp_list__1170753498": "Москва", "olymp_list__1170734747": "Нижний Новгород",
             "olymp_list__1170736201": "Пермь", "olymp_list__1120660311": "Санкт-Петербург"}


def rows_hse():
    """Четыре приложения по кампусам.

    Ширина строки гуляет (11 колонок на первой странице блока, 9-10 на
    продолжениях: pdfplumber выбрасывает пустые ведущие колонки), поэтому
    колонки отсчитываем от якоря «Вид особого права», а не по номеру.
    Приложение сгруппировано по направлению подготовки; имя ОП указано
    только там, где внутри направления их несколько.
    """
    out = []
    for f, campus in HSE_OLYMP.items():
        url = meta("hse", f)["url"]
        code = napr = name = profile = None
        for pg in load_pages("hse", f):
            for table in pg["tables"]:
                for row in table:
                    c = [clean(x) for x in row]
                    if not c:
                        continue
                    head = c[0]
                    m = re.match(r"(?:Направление подготовки|Специальность)\s+(\d{2}\.\d{2}\.\d{2})\s*(.*)", head)
                    if m:
                        code, napr, name, profile = m.group(1), clean(m.group(2)), None, None
                        continue
                    j = next((i for i, x in enumerate(c) if x.startswith("Право на")), None)
                    if j is None or j < 4 or not code:
                        continue
                    benefit = _benefit(c[j])
                    if not benefit:
                        continue
                    if j >= 5 and c[j - 5]:
                        name = c[j - 5]
                        profile = None
                    if j >= 4 and c[j - 4]:
                        profile = c[j - 4]
                    program = c[j - 6] if j >= 6 and c[j - 6] else None
                    if not name or not profile:
                        continue
                    out.append({
                        "match": {"code": code, "campus": campus, "program": program},
                        "olympiad_name": name, "profile": profile, "level": None,
                        "statuses": _statuses(c[j + 2] if len(c) > j + 2 else ""),
                        "benefit": benefit,
                        # Колонки справа от якоря есть не во всех строках
                        # (узкие строки теряют хвост), слева смещение стабильно:
                        # j-3 — предмет олимпиады, j-2 — предметы ВИ.
                        "ege_subject": (c[j - 3] if j >= 3 and c[j - 3]
                                        else (c[j - 2] if j >= 2 else "")),
                        "ege_score": _score(c[j - 1]),
                        "grades": _grades(c[j + 3] if len(c) > j + 3 else ""),
                        "page": pg["page"], "url": url,
                    })
    return out


def rows_mipt():
    """Матрица 146x16: 11 правых колонок — физтех-школы, БВИ зависит от школы."""
    out = []
    url = "https://pk.mipt.ru/bachelor/2026_olympiads/"
    rows = html_rows(load_html("mipt", "olymp_list__2026_olympiads"))
    schools = None
    for c in rows:
        if schools is None and "ФПМИ" in c and "ФРКТ" in c:
            schools = [x for x in c if x][:11]
            continue
        if not (c and c[0].isdigit() and len(c) >= 6):
            continue
        num, name, profile, level = int(c[0]), c[1], c[2], _level(c[3])
        if c[4]:                                    # общая колонка «100 баллов»
            out.append({"match": None, "olympiad_name": name, "profile": profile,
                        "level": level, "statuses": _statuses(c[4]), "benefit": HUNDRED,
                        "ege_subject": _subject_from(c[4]), "ege_score": _score(c[4]),
                        "grades": None, "page": None, "url": url, "number": num})
        for i, cell in enumerate(c[5:16]):
            if not clean(cell) or not schools:
                continue
            out.append({"match": {"school": schools[i] if i < len(schools) else None},
                        "olympiad_name": name, "profile": profile, "level": level,
                        "statuses": _statuses(cell), "benefit": BVI,
                        "ege_subject": _subject_from(cell), "ege_score": _score(cell),
                        "grades": None, "page": None, "url": url, "number": num})
    return out


def _subject_from(text: str) -> str | None:
    t = clean(text).lower()
    for s in ("информатик", "математик", "физик", "хими", "биолог", "обществознани", "экономик"):
        if s in t:
            return {"информатик": "Информатика", "математик": "Математика", "физик": "Физика",
                    "хими": "Химия", "биолог": "Биология", "обществознани": "Обществознание",
                    "экономик": "Экономика"}[s]
    return None


ITMO_COLS = [136, 248, 371, 427, 462, 551]


def _itmo_rows(pdf_path, cols=None):
    """Таблицы ИТМО нарисованы прямоугольниками, а не линиями: pdfplumber их
    не находит и отдаёт только рамки. Берём границы строк из прямоугольников,
    колонки задаём явно — иначе текстовый слой читается вперемешку по колонкам."""
    import pdfplumber
    with pdfplumber.open(pdf_path) as pdf:
        for page_no, page in enumerate(pdf.pages, start=1):
            ys = sorted({round(v) for r in page.rects for v in (r["top"], r["bottom"])})
            ys = [y for i, y in enumerate(ys) if i == 0 or y - ys[i - 1] > 3]
            if len(ys) < 3:
                continue
            table = page.extract_table({
                "vertical_strategy": "explicit", "explicit_vertical_lines": cols or ITMO_COLS,
                "horizontal_strategy": "explicit", "explicit_horizontal_lines": ys,
            }) or []
            for row in table:
                yield page_no, [clean(c) for c in row]


def rows_mipt_vsosh():
    """Приложение 2 на той же странице: предмет ВсОШ x физтех-школы.
    Лежит отдельной таблицей от перечневых олимпиад — легко пропустить."""
    out = []
    url = "https://pk.mipt.ru/bachelor/2026_olympiads/"
    tables = re.findall(r"<table.*?</table>", load_html("mipt", "olymp_list__2026_olympiads"), re.S)
    if len(tables) < 3:
        return out
    schools = None
    for c in html_rows(tables[2]):
        if schools is None and "ФПМИ" in c and "ФРКТ" in c:
            schools = [x for x in c if x][:11]
            continue
        if not schools or len(c) < 2 or not c[0]:
            continue
        subject = c[0].split(";")[0].split(",")[0]
        if not profile_slug(subject.lower()):
            continue
        for i, cell in enumerate(c[1:12]):
            if not clean(cell) or i >= len(schools):
                continue
            out.append({"match": {"school": schools[i]}, "vsosh": True,
                        "olympiad_name": None, "profile": subject.lower(), "level": "ВсОШ",
                        "statuses": _statuses(cell), "benefit": BVI,
                        "ege_subject": subject, "ege_score": None, "grades": None,
                        "page": None, "url": url, "score_is_demo": True})
    return out


def rows_kfu_vsosh():
    """Страницы 4-6 приложения 3: предмет ВсОШ -> направления подготовки."""
    out = []
    f = "olymp_list__prilozhenie_3_pp2026_1-ot-27.01-poslednyaya.pdf"
    url = meta("kfu", f)["url"]
    subject = None
    for pg in load_pages("kfu", f):
        if pg["page"] not in (4, 5, 6):
            continue
        for table in pg["tables"]:
            for row in table:
                c = [clean(x) for x in row]
                if len(c) < 4:
                    continue
                if c[1] and not c[1].lower().startswith("предмет"):
                    subject = c[1]
                codes = CODE_RE.findall(c[3])
                if not subject or not codes or not profile_slug(subject.lower()):
                    continue
                out.append({"match": {"codes": codes}, "vsosh": True,
                            "olympiad_name": None, "profile": subject.lower(), "level": "ВсОШ",
                            "statuses": [POB, PRIZ], "benefit": BVI,
                            "ege_subject": c[2] or subject, "ege_score": None,
                            "grades": None, "page": pg["page"], "url": url,
                            "score_is_demo": True})
    return out


def rows_itmo_vsosh():
    """Соотнесение профилей ВсОШ с направлениями. Вёрстка рваная: коды и
    предметы вытаскиваем регулярками из ячеек, а не по позициям."""
    out = []
    f = "vsosh_list__vsosh_2026.pdf"
    url = meta("itmo", f)["url"]
    for page_no, c in _itmo_rows(SNAP / "itmo" / f, cols=[36, 172, 425, 439, 530, 551]):
        codes = CODE_RE.findall(" ".join(c[:2]))
        tail = " ".join(c[2:])
        if not codes:
            continue
        for word in re.findall(r"[А-ЯЁ][а-яё]{4,}", tail):
            if not profile_slug(word.lower()):
                continue
            out.append({"match": {"codes": codes}, "vsosh": True, "olympiad_name": None,
                        "profile": word.lower(), "level": "ВсОШ", "statuses": [POB, PRIZ],
                        "benefit": BVI, "ege_subject": word, "ege_score": None,
                        "grades": None, "page": page_no, "url": url, "score_is_demo": True})
    return out


def rows_itmo():
    """Два плоских перечня на весь вуз: один даёт БВИ, другой — 100 баллов.
    Название олимпиады задано один раз на блок профилей, переносим вниз."""
    out = []
    for f, benefit in (("olymp_list__rsosh_bvi_2026.pdf", BVI),
                       ("olymp_list__rsosh_100_2026.pdf", HUNDRED)):
        url = meta("itmo", f)["url"]
        name = None
        for page_no, c in _itmo_rows(SNAP / "itmo" / f):
            if len(c) < 5:
                continue
            junk = ("Министерст", "государственное", "федеральное", "ыдаипмило",
                    "Наименование", "победителей", "Приложение", "УТВЕРЖДАЮ",
                    "учрежде", "университет", "ИТМО")
            if c[0] and not any(x in c[0] for x in junk):
                name = c[0]
            profile, subject, level, status = c[1], c[2], c[3], c[4]
            if not (name and profile and level and status):
                continue
            # «Все из перечня олимпиад школьников» — не название, а правило:
            # подходит любая перечневая олимпиада этого профиля.
            by_profile = name.lower().startswith("все из перечня")
            out.append({"match": None,
                        "by_profile": by_profile,
                        "olympiad_name": None if by_profile else name,
                        "profile": profile,
                        "level": _level(level), "statuses": _statuses(status),
                        "benefit": benefit, "ege_subject": subject or profile,
                        "ege_score": 75, "grades": None, "page": page_no, "url": url})
    return out


def rows_sechenov():
    """Приложение 5: БВИ по перечневым Сеченов не даёт — только 100 баллов (стр.1)."""
    out, name, profile = [], None, None
    f = "olymp_list__Pravila-priema_2026_2027_BS_pril5_Perechen-olimpiad.pdf"
    url = meta("sechenov", f)["url"]
    for pg in load_pages("sechenov", f):
        for table in pg["tables"]:
            for row in table:
                c = [clean(x) for x in row if clean(x) != ""]
                if len(c) < 4:
                    continue
                if c[0] and re.fullmatch(r"\d+\.?", c[0]) and len(c) > 1:
                    name = c[1]
                    c = c[1:]
                benefit = _benefit(c[-1])
                if not benefit or not name:
                    continue
                profile = c[1] if len(c) > 2 else profile
                out.append({"match": None, "olympiad_name": name, "profile": profile or "",
                            "level": None, "statuses": [POB, PRIZ], "benefit": benefit,
                            "ege_subject": c[-2] if len(c) > 2 else None, "ege_score": 75,
                            "grades": None, "page": pg["page"], "url": url})
    return out


def rows_kazan_gmu():
    """5 колонок, причём № по перечню указан прямо в документе."""
    out = []
    f = "olymp_list__download"
    url = meta("kazan-gmu", f)["url"]
    num = name = None
    for pg in load_pages("kazan-gmu", f):
        for table in pg["tables"]:
            for row in table:
                c = [clean(x) for x in row]
                if len(c) < 5:
                    continue
                if c[0].isdigit():
                    num, name = int(c[0]), c[1]
                if not name or not c[2]:
                    continue
                out.append({"match": None, "olympiad_name": name, "profile": c[2],
                            "level": _level(c[4]), "statuses": [POB, PRIZ], "benefit": HUNDRED,
                            "ege_subject": c[3], "ege_score": 75, "grades": None,
                            "page": pg["page"], "url": url, "number": num,
                            "benefit_is_demo": True})
    return out


def rows_nsu():
    """Льготы заданы по предмету/профилю олимпиады, а не по её названию."""
    out = []
    url = "https://www.nsu.ru/n/education/apply-info/olimpiady-privilege/"
    html = load_html("nsu", "olymp_list__olimpiady-privilege")
    chunks = re.split(r'<span class="name line">([^<]+)</span>', html)
    for i in range(1, len(chunks), 2):
        header = clean(chunks[i])
        m = CODE_RE.search(header)
        if not m:
            continue
        prog = clean(header.split("(")[0])
        for c in html_rows(chunks[i + 1][:60000]):
            if len(c) < 2:
                continue
            benefit = _benefit(c[-1])
            if not benefit:
                continue
            profile = c[0]
            if not profile or "предмет" in profile.lower():
                continue
            base = {"match": {"program": prog, "code": m.group(1)},
                    "olympiad_name": None, "profile": profile, "benefit": benefit,
                    "statuses": [POB, PRIZ], "page": None, "url": url}
            if len(c) == 2:
                # таблица из двух колонок — это ВсОШ (предмет + льгота)
                out.append({**base, "vsosh": True, "level": "ВсОШ",
                            "ege_subject": profile, "ege_score": None,
                            "grades": None, "score_is_demo": True})
            else:
                out.append({**base, "by_profile": True, "level": None,
                            "ege_subject": c[1], "ege_score": 75, "grades": [10, 11]})
    return out


def rows_kfu():
    """Приложение 3: 7 колонок. № и название олимпиады заданы один раз на блок
    строк-профилей, поэтому их надо переносить вниз, иначе теряется почти всё."""
    out = []
    f = "olymp_list__prilozhenie_3_pp2026_1-ot-27.01-poslednyaya.pdf"
    url = meta("kfu", f)["url"]
    num = name = None
    for pg in load_pages("kfu", f):
        for table in pg["tables"]:
            for row in table:
                c = [clean(x) for x in row]
                if len(c) < 7:
                    continue
                if c[0].isdigit() and c[1]:
                    num, name = int(c[0]), c[1]
                profile, level, targets = c[2], _level(c[4]), c[6]
                if not (name and profile and level and targets):
                    continue
                codes = CODE_RE.findall(targets)
                all_directions = "на все направления" in targets.lower()
                if not codes and not all_directions:
                    continue
                out.append({
                    "match": {"codes": codes, "all": all_directions},
                    "olympiad_name": name, "profile": profile, "level": level,
                    "statuses": [POB, PRIZ], "benefit": BVI,
                    "ege_subject": c[5] or c[3], "ege_score": 75,
                    "grades": None, "page": pg["page"], "url": url, "number": num,
                })
    return out


def rows_innopolis():
    """Приказ от 19.01.2026. Приложение 2 (стр.4) — ВсОШ, приложение 3 (стр.5-16) —
    перечень 2025/26. Приложения 4-6 — перечни прошлых лет (диплом действует 4 года),
    для кампании 2026 они дублируют льготу и в датасет не берутся."""
    f = "olymp_list____2026.pdf"
    return innopolis_rows(load_pages("innopolis", f), meta("innopolis", f)["url"])


def innopolis_rows(pages: list[dict], url: str) -> list[dict]:
    """Одну и ту же таблицу приложения 3 pdfplumber режет на 5, 7 или 11
    колонок, смотря по странице, и по номеру колонки на стр. 8, 9, 11 и 16
    терялось 40 строк, среди них Innopolis Open. Поэтому ячейки берутся вокруг
    ячейки уровня: до неё название (если есть), профиль и предмет олимпиады,
    после — предмет вступительного испытания. Название, разорванное на две
    строки, склеивается: у всех строк олимпиады оно общее."""
    out = []  # (название, строка): название — [текст], продолжение дописывается
    name = None
    for pg in pages:
        page = pg["page"]
        if page not in range(4, 17):
            continue
        for table in pg["tables"]:
            name_col = None
            for row in table:
                c = [clean(x) for x in row]
                if page == 4:
                    if len(c) < 3 or not c[1] or not profile_slug(c[1].lower()):
                        continue
                    out.append((None, {"match": None, "vsosh": True, "olympiad_name": None,
                                       "profile": c[1].lower(), "level": "ВсОШ",
                                       "statuses": [POB, PRIZ], "benefit": BVI,
                                       "ege_subject": c[2] or c[1], "ege_score": None,
                                       "grades": [9, 10, 11], "page": page, "url": url}))
                    continue
                lv = next((i for i, x in enumerate(c) if x in ("I", "II", "III", "1", "2", "3")), None)
                cells = [(i, x) for i, x in enumerate(c[:lv]) if x]
                if lv is None:
                    if name and len(cells) == 1 and cells[0][0] == name_col:
                        name[0] += " " + cells[0][1]
                    continue
                if len(cells) >= 3 and len(cells[0][1]) > 6:
                    name_col, name = cells[0][0], [cells[0][1]]
                    cells = cells[1:]
                if not (name and cells):
                    continue
                profile = cells[0][1]
                out.append((name, {"match": None, "profile": profile, "level": _level(c[lv]),
                                   "statuses": [POB, PRIZ], "benefit": BVI,
                                   "ege_subject": next((x for x in c[lv + 1:] if x), profile),
                                   "ege_score": 75, "grades": [9, 10, 11], "page": page, "url": url}))
    return [dict(r, olympiad_name=n[0]) if n else r for n, r in out]


PARSERS = {"msu": rows_msu, "msu_vsosh": rows_msu_vsosh, "hse_vsosh": rows_hse_vsosh, "spbu": rows_spbu, "hse": rows_hse, "mipt": rows_mipt,
           "itmo": rows_itmo, "nsu": rows_nsu, "kfu": rows_kfu,
           "innopolis": rows_innopolis, "sechenov": rows_sechenov, "kazan-gmu": rows_kazan_gmu,
           "mipt_vsosh": rows_mipt_vsosh, "kfu_vsosh": rows_kfu_vsosh,
           "itmo_vsosh": rows_itmo_vsosh}


# =============================================================== линковка и сборка

# Какие предметы олимпиад вообще дают льготу в какой профильной группе.
# Без этого фильтра плоский перечень вуза («список на весь университет»)
# приписывает биологию «Фундаментальной математике»: формально олимпиада
# в списке есть, но льгота по ней даётся только туда, где предмет совпадает
# с вступительным испытанием направления.
SUBJECT_GROUPS = {
    "информатик": {"ИТ", "Экономика"},
    "программирован": {"ИТ"},
    "математик": {"ИТ", "Физика", "Экономика"},
    "физик": {"Физика", "ИТ"},
    "хими": {"Биомед", "Физика"},
    "биолог": {"Биомед"},
    "медицин": {"Биомед"},
    "обществознан": {"Экономика"},
    "эконом": {"Экономика"},
    "финанс": {"Экономика"},
    "истори": set(),
    "русский": set(),
    "иностранн": set(),
    "литератур": set(),
    "географ": set(),
    "право": {"Экономика"},
    "искусств": set(),
    "рисунок": set(),
    "лингвист": set(),
    "журналист": set(),
    "педагог": set(),
    "дизайн": set(),
    "черчени": set(),
}


CANON_SUBJECTS = ("Информатика и ИКТ", "Информатика", "Математика", "Физика", "Химия",
                  "Биология", "Обществознание", "Экономика", "Русский язык",
                  "Иностранный язык", "История", "География", "Литература")


def canon_subject(subject: str) -> str | None:
    """«Информат ика» -> «Информатика»: экстракция рвёт слово на границе колонки."""
    t = clean(subject or "")
    if not t:
        return None
    flat = re.sub(r"[\s,]+", "", t).lower()
    for good in CANON_SUBJECTS:
        if re.sub(r"\s+", "", good).lower() in flat:
            return good
    return t


# Профили олимпиад, пересекающиеся со скоупом проекта. Датасет C по п.4 спеки
# собирается только по ним, поэтому и B обязан держаться той же границы:
# иначе join B->C даёт пропуски не из-за ошибки, а из-за разной ширины скоупа.
SCOPE_PROFILE_WORDS = {
    "ИТ": ("информатик", "программирован", "вычислительн", "компьютерн", "кибернетик",
           "искусственный интеллект", "данны", "информационн", "робототехник",
           "инфокоммуникац", "математик"),
    "Физика": ("физик", "астроном", "ядерн", "нанотехнолог", "фотоник", "техник",
               "инженерн", "механик", "высокие технологии", "наносистем", "математик"),
    "Биомед": ("биолог", "хими", "медицин", "генетик", "инфохими", "естественные науки",
               "экологи", "агро"),
    "Экономика": ("эконом", "финанс", "обществознан", "бизнес", "менеджм",
                  "предпринимат", "математик"),
}


def profile_in_scope(profile: str) -> bool:
    t = clean(profile or "").lower()
    return any(w in t for ws in SCOPE_PROFILE_WORDS.values() for w in ws)


def subject_fits(subject: str, profile_group: str) -> bool:
    # В PDF слова рвутся на границе колонки («Обществоз нание»), поэтому
    # сравниваем и со схлопнутыми пробелами, иначе фильтр молча пропускает мусор.
    t = clean(subject or "").lower()
    t = t + " " + re.sub(r"\s+", "", t)
    if not t:
        return True
    groups, matched = set(), False
    for key, gs in SUBJECT_GROUPS.items():
        if key in t:
            groups |= gs
            matched = True
    return profile_group in groups if matched else True


def _norm_prog(name: str) -> str:
    n = clean(name).lower().replace("ё", "е")
    n = re.sub(r"\(.*?\)", " ", n)
    n = re.sub(r"[«»\"',.:;\-–—/]", " ", n)
    return re.sub(r"\s+", " ", n).strip()


def link(vuz_id: str, row: dict, programs: list[dict]) -> list[dict]:
    """Какие программы вуза покрывает эта строка льготы."""
    m = row.get("match")
    if m is None:                                   # плоский перечень на весь вуз
        return programs
    if "school" in m:                               # МФТИ: льгота адресована физтех-школе
        school = clean(m["school"])
        if not school:
            return []
        return [p for p in programs if school.split()[0] in (p["faculty"] or "")]
    if "codes" in m:                                # КФУ: перечислены коды направлений
        if m.get("all"):
            return programs
        return [p for p in programs if p["napravlenie_code"] in m["codes"]]
    if m.get("campus"):                             # ВШЭ: направление внутри кампуса
        pool = [p for p in programs if m["campus"] in (p["faculty"] or "")
                and p["napravlenie_code"] == m.get("code")]
        if m.get("program"):
            target = _norm_prog(m["program"])
            narrowed = [p for p in pool if target in _norm_prog(p["program_name"])
                        or _norm_prog(p["program_name"]) in target]
            if narrowed:
                return narrowed
        return pool
    if "program" in m:                              # СПбГУ / НГУ: адресовано ОП
        target = _norm_prog(m["program"])
        if len(target) < 4:
            return []
        pool = programs
        if m.get("campus"):
            pool = [p for p in pool if m["campus"] in (p["faculty"] or "")]
        exact = [p for p in pool if _norm_prog(p["program_name"]) == target]
        if exact:
            return exact
        partial = [p for p in pool if target in _norm_prog(p["program_name"])
                   or _norm_prog(p["program_name"]) in target]
        if partial:
            return partial
        if m.get("code"):
            return [p for p in pool if p["napravlenie_code"] == m["code"]]
        return []
    if "faculty" in m:                              # МГУ: секция факультета + направление
        want = clean(m.get("faculty") or "").lower()
        pool = [p for p in programs if want and want == clean(p["faculty"] or "").lower()]
        napr = _norm_prog(m.get("napravlenie", ""))
        code = CODE_RE.search(m.get("napravlenie", "") or "")
        if code:
            hit = [p for p in pool if p["napravlenie_code"] == code.group(1)]
            if hit:
                return hit
        if napr:
            hit = [p for p in pool if napr in _norm_prog(p["napravlenie_name"])
                   or _norm_prog(p["napravlenie_name"]) in napr]
            if hit:
                return hit
        return pool
    return []


def expand_profile(profile: str, min_level: str | None) -> list[tuple[str, str, str]]:
    """Вуз задал профиль, а не название: подходит любая перечневая олимпиада
    с этим профилем. Уровень I сильнее II и III, поэтому требование «не ниже
    уровня N» пропускает олимпиады с уровнем N и выше."""
    want = clean(profile).lower()
    rank = {"I": 1, "II": 2, "III": 3}
    cap = rank.get(min_level or "III", 3)
    hits = []
    for row in _index:
        if row["profile"].strip().lower() != want:
            continue
        if rank.get(row["level"], 3) > cap:
            continue
        hits.append((row["olympiad_id"], row["name"], row["level"]))
    return hits


def resolve_olympiad(row: dict) -> tuple[str | None, str | None, str | None]:
    """-> (olympiad_id, отображаемое имя, причина пропуска)."""
    profile = clean(row.get("profile") or "")
    if row.get("vsosh") or row.get("level") == "ВсОШ":
        oid = (vsosh_id(profile.lower())
               or vsosh_id(clean(row.get("vsosh_subject") or "").lower())
               or vsosh_id(clean(row.get("ege_subject") or "").lower()))
        return (oid, f"Всероссийская олимпиада школьников, {profile or row.get('ege_subject')}",
                None if oid else "не удалось сопоставить предмет ВсОШ")
    name = row.get("olympiad_name")
    if not name:
        return None, None, "строка без названия олимпиады"
    slug = profile_slug(profile.lower())
    if slug is None:
        return None, name, f"профиль «{profile}» отсутствует в перечне"
    num = row.get("number") or match_number(name)
    # Номер обязан существовать ВМЕСТЕ с этим профилем, иначе пара выдумана.
    if num is None or level_in_perechen(num, profile) is None:
        better = match_number_for_profile(name, profile)
        if better is None:
            return None, name, "пара «олимпиада + профиль» не найдена в перечне №669"
        num = better
    return f"p669-{num}-{slug}", name, None


def main() -> int:
    a_rows = json.loads((DATA / "vuz_napravleniya.json").read_text(encoding="utf-8"))["vuz_napravleniya"]
    offered = [x for x in a_rows if x["status"] == "offered"]
    by_vuz = defaultdict(list)
    seen_pid = set()
    for x in offered:
        if x["program_id"] in seen_pid:
            continue
        seen_pid.add(x["program_id"])
        by_vuz[x["vuz_id"]].append(x)

    per_program: dict[str, list] = defaultdict(list)
    skipped = defaultdict(int)
    reasons = defaultdict(lambda: defaultdict(int))

    for key, fn in PARSERS.items():
        vuz_id = key.replace("_vsosh", "")
        programs = by_vuz[vuz_id]
        linked_rows = 0
        for row in fn():
            if row.get("by_profile"):
                variants = [(oid, nm, lv) for oid, nm, lv in
                            expand_profile(row.get("profile") or "", row.get("level"))]
                if not variants:
                    skipped[vuz_id] += 1
                    reasons[vuz_id]["профиль не найден в перечне"] += 1
                    continue
            else:
                oid, disp, why = resolve_olympiad(row)
                if not oid:
                    skipped[vuz_id] += 1
                    reasons[vuz_id][why or "?"] += 1
                    continue
                variants = [(oid, disp, row.get("level"))]
            # У строк ВсОШ профиль помечен прочерком (он к ВсОШ неприменим),
            # предмет лежит в отдельной колонке — проверять надо его.
            probe = next((clean(row.get(k) or "") for k in
                          ("profile", "vsosh_subject", "ege_subject")
                          if clean(row.get(k) or "") not in ("", "—", "-", "–")), "")
            if not profile_in_scope(probe):
                skipped[vuz_id] += 1
                reasons[vuz_id]["профиль олимпиады вне скоупа 4 групп (как и в датасете C)"] += 1
                continue
            targets = link(vuz_id, row, programs)
            if row.get("match") is None or (row.get("match") or {}).get("all"):
                subj = row.get("ege_subject") or row.get("profile")
                before = len(targets)
                targets = [t for t in targets if subject_fits(subj, t["profile_group"])]
                if targets != [] and before != len(targets):
                    pass
            if not targets:
                skipped[vuz_id] += 1
                reasons[vuz_id]["направление вне скоупа 4 групп либо не сматчилось"] += 1
                continue
            linked_rows += 1
            for oid, disp, lvl in variants:
              row = dict(row, level=lvl or row.get("level"))
              for prog in targets:
                for status in row["statuses"]:
                    per_program[prog["program_id"]].append({
                        "olympiad_id": oid,
                        "olympiad_name_at_vuz": disp,
                        "min_level_required": row.get("level") or level_in_perechen(
                            int(oid.split("-")[1]) if oid.startswith("p669") else 0,
                            clean(row.get("profile") or "").lower()),
                        "diploma_status": status,
                        "benefit_type": row["benefit"],
                        "eligible_grades": row.get("grades"),
                        "ege_confirm_subject": canon_subject(row.get("ege_subject")),
                        "ege_confirm_min_score": row.get("ege_score"),
                        "source_url": row["url"],
                        "source_page": row.get("page"),
                        "source_date": fetched_date(row["url"]),
                        "is_demo": bool(row.get("score_is_demo") or row.get("benefit_is_demo")
                                        or row.get("ege_score") is None),
                    })
        print(f"  {vuz_id:11s} строк привязано: {linked_rows:5d} | пропущено: {skipped[vuz_id]:5d}"
              )
        for why, n in sorted(reasons[vuz_id].items(), key=lambda kv: -kv[1]):
            print(f"{'':16}{n:5d}  {why[:72]}")

    objects = []
    emitted = set()
    for x in a_rows:
        if x["status"] != "offered":
            objects.append({
                "program_id": None, "vuz_id": x["vuz_id"], "program_name": None,
                "faculty": None, "napravlenie_code": None, "profile_group": x["profile_group"],
                "status": x["status"], "admission_year": ADMISSION_YEAR,
                "checked_url": x["source_url"], "checked_page": x["source_page"],
                "checked_date": fetched_date(x["source_url"]), "budget_places_2026": None, "prinimaemye_olimpiady": [],
            })
            continue
        if x["program_id"] in emitted:
            continue
        emitted.add(x["program_id"])
        benefits, seen = [], set()
        for b in per_program.get(x["program_id"], []):
            key = (b["olympiad_id"], b["diploma_status"], b["benefit_type"])
            if key in seen:
                continue
            seen.add(key)
            benefits.append(b)
        benefits.sort(key=lambda b: (b["olympiad_id"], b["diploma_status"]))
        objects.append({
            "program_id": x["program_id"], "vuz_id": x["vuz_id"],
            "program_name": x["program_name"], "faculty": x["faculty"],
            "napravlenie_code": x["napravlenie_code"], "profile_group": x["profile_group"],
            "status": "offered" if benefits else "to_check",
            "admission_year": ADMISSION_YEAR,
            "checked_url": x["source_url"], "checked_page": x["source_page"],
            "checked_date": fetched_date(x["source_url"]), "budget_places_2026": x["budget_places_2026"],
            "prinimaemye_olimpiady": benefits,
        })

    total = sum(len(o["prinimaemye_olimpiady"]) for o in objects)
    empty = sum(1 for o in objects if o["status"] == "to_check")
    print(f"\nпрограмм: {len(objects)}, записей о льготах: {total}, без льгот (to_check): {empty}")
    write_json(DATA / "vuz_napravlenie_olimpiady.json", "vuz_napravlenie_olimpiady", objects,
               "Датасет B по data_format_spec_4.md. Ключ связи с A — program_id. "
               "Одна запись = одна пара (olympiad_id, diploma_status).")
    return 0


if __name__ == "__main__":
    sys.exit(main())
