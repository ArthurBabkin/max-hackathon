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
from difflib import SequenceMatcher
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
    f = "olymp_list__olymp_benefits.pdf"
    return msu_rows(load_pages("msu", f), meta("msu", f)["url"])


def msu_rows(pages: list[dict], url: str) -> list[dict]:
    """Профиль, перечень, класс и предмет ЕГЭ — объединённые ячейки на
    несколько уровней или статусов («Математика, *: I — БВИ, II — 100 баллов»).
    pdfplumber кладёт текст в первую строку, у продолжений ячейки пустые, и без
    переноса вниз терялось 88 строк: химия II уровня на химфаке, призёры и т. п."""
    out, faculty, napravlenie, above = [], None, None, None
    for pg in pages:
        for table in pg["tables"]:
            for row in table:
                c = [clean(x) for x in row]
                if not any(c):
                    continue
                if c[0] and not any(c[3:]) and len(c[0]) > 8:
                    faculty = c[0]
                    napravlenie = above = None
                    continue
                if len(c) < 9 or c[0].lower().startswith("направление"):
                    continue
                if c[0]:
                    napravlenie, above = c[0], None
                if c[1]:
                    above = c
                elif above:
                    c = [x or (above[i] if i in (1, 2, 3, 5, 7) else x) for i, x in enumerate(c)]
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


MIPT_URL = "https://pk.mipt.ru/bachelor/2026_olympiads/"


def rows_mipt():
    return mipt_rows(load_html("mipt", "olymp_list__2026_olympiads"), MIPT_URL)


def _mipt_schools(header: list[str]) -> tuple[int, list[str]] | None:
    """Колонки физтех-школ начинаются с «ФРКТ». Школа — по позиции в той же
    строке заголовка: срез непустых ячеек сдвигал каждую школу на соседнюю."""
    if "ФРКТ" not in header or "ФПМИ" not in header:
        return None
    first = header.index("ФРКТ")
    return first, header[first:first + 11]


def _mipt_clauses(school: str, cell: str) -> list[dict]:
    """Ячейка школы: «Все конкурсные группы ФБМФ, ВШБИ Победителям …» или
    конкурсные группы в кавычках, иногда несколько условий через «;»."""
    out = []
    for clause in clean(cell).split(";"):
        clause = clause.strip()
        if not clause:
            continue
        m = re.match(r"Все конкурсные группы (.+?)\s+(?:Победител|победител|Призер|призер|Член)", clause)
        out.append({
            "match": {"school": m.group(1) if m else school,
                      "groups": None if m else re.findall(r"[\"«]([^\"»]+)[\"»]", clause)},
            "statuses": _statuses(clause), "ege_subject": _subject_from(clause),
            "ege_score": _score(clause),
        })
    return out


def mipt_rows(html: str, url: str) -> list[dict]:
    """Матрица 146x16: 11 правых колонок — физтех-школы, БВИ зависит от школы."""
    out, cols = [], None
    for c in html_rows(html):
        cols = cols or _mipt_schools(c)
        if not (c and c[0].isdigit() and len(c) >= 6):
            continue
        num, name, profile, level = int(c[0]), c[1], c[2], _level(c[3])
        if c[4]:                                    # общая колонка «100 баллов»
            out.append({"match": None, "olympiad_name": name, "profile": profile,
                        "level": level, "statuses": _statuses(c[4]), "benefit": HUNDRED,
                        "ege_subject": _subject_from(c[4]), "ege_score": _score(c[4]),
                        "grades": None, "page": None, "url": url, "number": num})
        if not cols:
            continue
        first, schools = cols
        for school, cell in zip(schools, c[first:first + len(schools)]):
            for cl in _mipt_clauses(school, cell):
                out.append({**cl, "olympiad_name": name, "profile": profile, "level": level,
                            "benefit": BVI, "grades": None, "page": None, "url": url,
                            "number": num})
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
    return mipt_vsosh_rows(load_html("mipt", "olymp_list__2026_olympiads"), MIPT_URL)


def mipt_vsosh_rows(html: str, url: str) -> list[dict]:
    """Приложение 2 на той же странице: предмет ВсОШ x физтех-школы.
    Лежит отдельной таблицей от перечневых олимпиад — легко пропустить."""
    out, cols = [], None
    for c in html_rows(html):
        if c and c[0] == "Общеобразовательный предмет":
            cols = _mipt_schools(c)
            continue
        if not cols or len(c) < 2 or not c[0]:
            continue
        subject = c[0].split(";")[0].split(",")[0]
        if not profile_slug(subject.lower()):
            continue
        first, schools = cols
        for school, cell in zip(schools, c[first:first + len(schools)]):
            for cl in _mipt_clauses(school, cell):
                out.append({**cl, "vsosh": True, "olympiad_name": None,
                            "profile": subject.lower(), "level": "ВсОШ", "benefit": BVI,
                            "ege_subject": subject, "ege_score": None, "grades": None,
                            "page": None, "url": url, "score_is_demo": True})
    return out


KFU_OLYMP = "olymp_list__prilozhenie_3_pp2026_1-ot-27.01-poslednyaya.pdf"
KFU_PLAN_DOC = "programs__plan_priema_2026_2027-bakalavriat-speczialitet-1.pdf"


def rows_kfu_vsosh():
    """Страницы 4-6 приложения 3: предмет ВсОШ -> направления подготовки.
    Список направлений предмета продолжается на следующих строках; пустой
    список — «все направления, где этот предмет — первое ВИ»."""
    url = meta("kfu", KFU_OLYMP)["url"]
    subjects: dict[str, dict] = {}
    subject = None
    for pg in load_pages("kfu", KFU_OLYMP):
        if pg["page"] not in (4, 5, 6):
            continue
        for table in pg["tables"]:
            for row in table:
                c = [clean(x) for x in row]
                if len(c) < 4 or c[1].lower().startswith("предмет"):
                    continue
                if c[1]:
                    subject = c[1]
                    subjects.setdefault(subject, {"vi": c[2], "targets": [], "page": pg["page"]})
                if subject and c[3]:
                    subjects[subject]["targets"].append(c[3])
    out = []
    for subject, d in subjects.items():
        if not profile_slug(subject.lower()) or d["vi"] in ("", "-"):
            continue
        targets = " ".join(d["targets"])
        out.append({"match": kfu_match(targets or KFU_ALL, d["vi"]), "vsosh": True,
                    "olympiad_name": None, "profile": subject.lower(), "level": "ВсОШ",
                    "statuses": [POB, PRIZ], "benefit": BVI,
                    "ege_subject": d["vi"], "ege_score": None,
                    "grades": None, "page": d["page"], "url": url, "score_is_demo": True})
    return out


def rows_itmo_vsosh():
    import pdfplumber
    f = "vsosh_list__vsosh_2026.pdf"
    pages = []
    with pdfplumber.open(SNAP / "itmo" / f) as pdf:
        for page in pdf.pages:
            lines = [r["top"] for r in page.rects if r["height"] < 3 and r["width"] > 3
                     and r["x0"] >= ITMO_VSOSH_SPLIT - 4]
            pages.append((page.page_number, page.extract_words(), lines))
    return itmo_vsosh_rows(pages, meta("itmo", f)["url"])


ITMO_VSOSH_SPLIT = 426    # граница колонок «направление» и «предмет олимпиады»


def itmo_vsosh_rows(pages, url: str) -> list[dict]:
    """Соотнесение предметов ВсОШ с направлениями. Предмет — объединённая
    ячейка на блок направлений, блоки разделены отрезками в колонке
    предмета. Часть блока в начале страницы без предмета — продолжение
    блока с прошлой страницы. Пояснение в скобках («Информатика
    (Искусственный интеллект …)») предметом не считается.

    pages — [(номер, слова pdfplumber, [y отрезков колонки предмета])]."""
    blocks = []                        # [коды, текст предмета]
    for page_no, words, lines in pages:
        ys = sorted(lines)
        edges = [float("-inf"), *ys, float("inf")]
        first = True
        for lo, hi in zip(edges, edges[1:]):
            inside = sorted((w for w in words if lo <= w["top"] < hi), key=lambda w: (round(w["top"]), w["x0"]))
            left = " ".join(w["text"] for w in inside if w["x0"] < ITMO_VSOSH_SPLIT)
            right = " ".join(w["text"] for w in inside if w["x0"] >= ITMO_VSOSH_SPLIT)
            codes = CODE_RE.findall(left)
            if not codes:
                continue
            if first and not right.strip() and blocks:
                blocks[-1][0] += codes
            else:
                blocks.append([codes, right])
            first = False
    out = []
    for codes, subjects in blocks:
        subjects = re.sub(r"\([^)]*\)?", " ", subjects)
        for word in dict.fromkeys(w.lower() for w in re.findall(r"[А-ЯЁ][а-яё]{4,}", subjects)):
            if not profile_slug(word):
                continue
            out.append({"match": {"codes": codes}, "vsosh": True, "olympiad_name": None,
                        "profile": word, "level": "ВсОШ", "statuses": [POB, PRIZ],
                        "benefit": BVI, "ege_subject": word.capitalize(), "ege_score": None,
                        "grades": None, "page": None, "url": url, "score_is_demo": True})
    return out


# Колонки перечней ИТМО: направление, название, профиль, предмет ЕГЭ, уровень,
# статус диплома. У файлов разная сетка; прежняя общая начиналась с x=136 и
# отрезала направление, а файл «100 баллов» читала вообще не по его колонкам.
ITMO_LISTS = (("olymp_list__rsosh_bvi_2026.pdf", BVI, [36, 136, 248, 369, 425, 461, 551]),
              ("olymp_list__rsosh_100_2026.pdf", HUNDRED, [44, 163, 234, 347, 418, 467, 538]))
ITMO_LEVEL_COL = 4


def itmo_merged_rows(pages) -> list[tuple[int, list[str]]]:
    """Строки перечня ИТМО с объединёнными ячейками.

    Таблица нарисована отрезками. Строка — промежуток между отрезками в
    колонке уровня; объединённая ячейка другой колонки — промежуток между её
    собственными отрезками, текст собирается из всех её частей («Все из
    перечня олимпиад» + «школьников», направление на десяток строк).

    Блок направлений, разорванный страницей, режется на две части. Текст
    блока всегда начинается с кода, поэтому часть в начале страницы без
    кода в начале текста (или пустая) — продолжение блока с прошлой
    страницы, как и часть после текста, оборванного на запятой или коде;
    пустая часть в конце страницы — начало блока со следующей.
    У склеенного блока текст обеих частей.

    pages — [(номер, [(top, bottom, [ячейки])], {колонка: [y отрезков]})]."""
    rows, parts = [], []               # части блоков направлений: [страница, текст]
    for page_no, fine, lines in pages:
        if not fine:
            continue
        top, bottom = fine[0][0], fine[-1][1]

        def spans(col):
            ys = sorted({top, bottom, *[y for y in lines.get(col, []) if top <= y <= bottom]})
            return list(zip(ys, ys[1:]))

        col_spans = {j: spans(j) for j in range(6)}

        def span_of(j, y):
            return next((sp for sp in col_spans[j] if sp[0] <= y < sp[1]), None)

        def text(j, lo, hi):
            return clean(" ".join(cells[j] for t, b, cells in fine
                                  if lo <= (t + b) / 2 < hi and j < len(cells) and cells[j]))

        page_parts = {}
        for sp in col_spans[ITMO_LEVEL_COL]:
            mid = (sp[0] + sp[1]) / 2
            # Направление и название — объединённые ячейки на блок строк.
            # Остальное — в пределах строки: у колонки статуса отрезков
            # местами нет, и статусы соседних строк склеивались бы.
            row = [text(j, *span_of(j, mid)) if j < 2 else
                   (text(j, *sp) or text(j, *span_of(j, mid))) for j in range(6)]
            if not _level(row[ITMO_LEVEL_COL]):
                continue                  # шапка документа и таблицы
            dsp = span_of(0, mid)
            if dsp not in page_parts:
                page_parts[dsp] = len(parts)
                parts.append([page_no, row[0]])
            rows.append((page_no, row, page_parts[dsp]))
    group = list(range(len(parts)))
    for i in range(1, len(parts)):
        (pg, text), (prev_pg, prev_text) = parts[i], parts[i - 1]
        if pg == prev_pg:
            continue                      # внутри страницы блоки разделены отрезком
        cut = re.search(r"(,|\d{2}\.\d{2}\.\d{2})\s*$", prev_text)   # оборвано на полуслове
        if not CODE_RE.match(text) or not prev_text or cut:
            group[i] = group[i - 1]
    merged = defaultdict(list)
    for i, g in enumerate(group):
        merged[g].append(parts[i][1])
    return [(page_no, [clean(" ".join(merged[group[k]]))] + row[1:]) for page_no, row, k in rows]


def _itmo_pages(pdf_path, cols):
    import pdfplumber
    with pdfplumber.open(pdf_path) as pdf:
        for page_no, page in enumerate(pdf.pages, start=1):
            ys = sorted({round(v) for r in page.rects for v in (r["top"], r["bottom"])})
            ys = [y for i, y in enumerate(ys) if i == 0 or y - ys[i - 1] > 3]
            if len(ys) < 3:
                continue
            t = page.find_table({"vertical_strategy": "explicit", "explicit_vertical_lines": cols,
                                 "horizontal_strategy": "explicit", "explicit_horizontal_lines": ys})
            if not t:
                continue
            fine = [(row.bbox[1], row.bbox[3], [clean(c) for c in cells])
                    for row, cells in zip(t.rows, t.extract())]
            lines = {}
            for j in range(len(cols) - 1):
                left, right = cols[j], cols[j + 1]
                lines[j] = [r["top"] for r in page.rects
                            if r["height"] < 3 and r["width"] > 3
                            and r["x0"] <= left + 4 and r["x1"] >= right - 4]
            yield page_no, fine, lines


def rows_itmo():
    out = []
    for f, benefit, cols in ITMO_LISTS:
        out += itmo_rows(itmo_merged_rows(_itmo_pages(SNAP / "itmo" / f, cols)),
                         meta("itmo", f)["url"], benefit)
    return out


def itmo_rows(rows, url: str, benefit: str) -> list[dict]:
    """Два перечня: один даёт БВИ, другой — 100 баллов. Льгота адресована
    направлениям из первой колонки; «(только на направление 10.03.01)» в
    профиле сужает блок."""
    out = []
    for page_no, c in rows:
        direction, name, profile, subject, level, status = c
        codes = CODE_RE.findall(direction)
        if not (codes and name and profile and _level(level) and _statuses(status) and "ыдаипмило" not in name):
            continue
        only = re.search(r"\((?:только|учитывается только) на[^)]*\)", profile)
        if only:
            codes = CODE_RE.findall(only.group(0)) or codes
            profile = clean(profile.replace(only.group(0), ""))
        # «Все из перечня олимпиад школьников» — не название, а правило:
        # подходит любая перечневая олимпиада этого профиля.
        by_profile = name.lower().startswith("все из перечня")
        out.append({"match": {"codes": codes},
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
    строк-профилей, поэтому их надо переносить вниз, иначе теряется почти всё.
    Берётся только перечень 2025/26 (стр. 7–38): дальше перечни прошлых лет со
    своей нумерацией, номер из них указал бы на другую олимпиаду №669."""
    out = []
    url = meta("kfu", KFU_OLYMP)["url"]
    num = name = None
    for pg in load_pages("kfu", KFU_OLYMP):
        if not 7 <= pg["page"] <= 38:
            continue
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
                if not CODE_RE.search(targets) and "на все направления" not in targets.lower():
                    continue
                out.append({
                    "match": kfu_match(targets, c[5] or c[3]),
                    "olympiad_name": name, "profile": profile, "level": level,
                    "statuses": [POB, PRIZ], "benefit": BVI,
                    "ege_subject": c[5] or c[3], "ege_score": 75,
                    "grades": None, "page": pg["page"], "url": url, "number": num,
                })
    return out


KFU_ALL = "На все направления"
SUBJECT_STEMS = {"информатик": "Информатика", "математик": "Математика", "физик": "Физика",
                 "хими": "Химия", "биолог": "Биология", "обществозн": "Обществознание",
                 "истори": "История", "иностран": "Иностранный язык", "русск": "Русский язык",
                 "литератур": "Литература", "географ": "География", "эконом": "Экономика"}


def subject_keys(text: str) -> set[str]:
    """«Математика / Информатика и ИКТ*» -> {Математика, Информатика}."""
    t = re.sub(r"\s+", "", clean(text).lower())
    return {v for k, v in SUBJECT_STEMS.items() if k in t}


def kfu_plan(pages) -> dict[tuple[str, str], dict]:
    """План приёма КФУ (приложение 1): программа -> институт и ВИ по
    приоритету. Ключ — (код, название программы), как в датасете A."""
    out, institute = {}, None
    for pg in pages:
        for table in pg["tables"]:
            for row in table:
                c = [clean(x) for x in row]
                if c and c[0] and not CODE_RE.fullmatch(c[0]) and not any(c[1:]):
                    institute = c[0]
                    continue
                if len(c) < 10 or not CODE_RE.fullmatch(c[0]):
                    continue
                items = re.split(r"\s*\d\)\s*", (row[9] or "").replace("\n", " "))[1:]
                out.setdefault((c[0], (c[2] or c[1]).lower()), {
                    "institute": institute or "", "vi": [subject_keys(x) for x in items]})
    return out


_KFU_PLAN = None


def kfu_program(p: dict) -> dict | None:
    global _KFU_PLAN
    if _KFU_PLAN is None:
        _KFU_PLAN = kfu_plan(load_pages("kfu", KFU_PLAN_DOC))
    return _KFU_PLAN.get((p["napravlenie_code"], clean(p["program_name"]).lower()))


def kfu_match(targets: str, vi: str) -> dict:
    """Графа 7 приложения 3: либо направления («КОД Название (профиль: A;
    B)»), либо «На все направления, где вступительные испытания
    соответствуют графе 6 … и являются первыми в Плане приема», и
    возможно «кроме 09.03.04 …» / «кроме Института …»."""
    head, _, excl = clean(targets).partition("кроме")
    starts = [m.start() for m in CODE_RE.finditer(head)]
    entries = []
    for a, b in zip(starts, starts[1:] + [len(head)]):
        entry = head[a:b]
        prof = re.search(r"\(профил[ья]:?\s*[\"«]?([^)]*?)[\"»]?\)", entry)
        entries.append((CODE_RE.match(entry).group(1),
                        [_norm_prog(x) for x in prof.group(1).split(";")] if prof else []))
    inst = re.search(r"институт\w*\s+(.+)", excl, re.I)
    return {"kfu": True, "entries": entries,
            "all": not entries and "на все направления" in head.lower(),
            "first_vi": sorted(subject_keys(vi)),
            "except_codes": CODE_RE.findall(excl),
            "except_institute": clean(inst.group(1)).lower() if inst else None}


def kfu_link(m: dict, programs: list[dict]) -> list[dict]:
    out = []
    for p in programs:
        info = kfu_program(p)
        if m["entries"]:
            # Код без профиля покрывает все программы направления, но не ту,
            # где предмета олимпиады нет даже среди ВИ: 44.03.05 указан у
            # биологии целиком, а «Математику и информатику» биология не касается.
            name = _norm_prog(p["program_name"])
            exams = set().union(*info["vi"]) if info and info["vi"] else None
            ok = any(code == p["napravlenie_code"] and
                     (any(x == name or x in name or name in x for x in profs) if profs
                      else exams is None or bool(exams & set(m["first_vi"])))
                     for code, profs in m["entries"])
        else:
            ok = bool(m["all"] and info and info["vi"] and info["vi"][0] & set(m["first_vi"]))
        if p["napravlenie_code"] in m["except_codes"]:
            ok = False
        if m["except_institute"] and info and m["except_institute"] in info["institute"].lower():
            ok = False
        if ok:
            out.append(p)
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
        return mipt_link(m["school"], m.get("groups"), programs)
    if m.get("kfu"):                                # КФУ: коды, профили, «первое ВИ», «кроме»
        return kfu_link(m, programs)
    if "codes" in m:                                # ИТМО: перечислены коды направлений
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


def _school_tags(text: str) -> set[str]:
    """«ФБМФ/ ВШБИ», «ФБМФ, ВШБИ» -> {ФБМФ, ВШБИ}; «ВШ М» -> {ВШМ}."""
    return {t for t in re.split(r"[,/]", re.sub(r"\s+", "", text or "")) if t}


def mipt_link(school: str, groups: list[str] | None, programs: list[dict]) -> list[dict]:
    """Программы физтех-школы. У ФБМФ и ВШБИ факультет в A общий, а школа —
    в скобках названия: «Все конкурсные группы ФБМФ» ВШБИ не касается.
    Названные конкурсные группы сужают школу; группа может объединять две
    программы («Авиационные технологии и беспилотные авиационные системы»)."""
    want = _school_tags(school)
    pool = []
    for p in programs:
        if not want & _school_tags(p["faculty"]):
            continue
        tag = re.search(r"\((ФБМФ|ВШБИ)\)", p["program_name"])
        if tag and tag.group(1) not in want:
            continue
        pool.append(p)
    if groups is None:
        return pool
    names = [_norm_prog(g) for g in groups]
    return [p for p in pool if any(
        _norm_prog(p["program_name"]) in g or g in _norm_prog(p["program_name"])
        or SequenceMatcher(None, g, _norm_prog(p["program_name"])).ratio() > 0.9
        for g in names)]


def expand_profile(profile: str, min_level: str | None) -> list[tuple[str, str, str]]:
    """Вуз задал профиль, а не название: подходит любая перечневая олимпиада
    с этим профилем. Уровень I сильнее II и III, поэтому требование «не ниже
    уровня N» пропускает олимпиады с уровнем N и выше.

    Если точного профиля нет, вуз мог назвать часть составного профиля НТО
    («Аэрокосмические системы» — из «беспилотный транспорт: аэрокосмические
    системы, …») или чуть иначе его записать («… финансовых технологий»)."""
    want = re.sub(r"(\w)- (\w)", r"\1-\2", clean(profile).lower())   # перенос «бизнес- процессов»
    rank = {"I": 1, "II": 2, "III": 3}
    cap = rank.get(min_level or "III", 3)

    def fits(row, loose):
        have = row["profile"].strip().lower()
        if not loose:
            return have == want
        parts = [clean(x) for x in re.split(r"[:,]", have)]
        return want in parts or SequenceMatcher(None, want, have).ratio() > 0.9

    for loose in (False, True):
        hits = [(row["olympiad_id"], row["name"], row["level"]) for row in _index
                if fits(row, loose) and rank.get(row["level"], 3) <= cap]
        if hits or any(fits(row, loose) for row in _index):
            return hits
    return []


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
            # КФУ проверяет «первое ВИ» по плану приёма — точнее, чем группа.
            if row.get("match") is None or (row["match"].get("all") and not row["match"].get("kfu")):
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
