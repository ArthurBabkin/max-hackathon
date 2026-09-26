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
    """Профиль, перечень, уровень, класс и предмет ЕГЭ — объединённые ячейки на
    несколько уровней или статусов («Математика, *: I — БВИ, II — 100 баллов»).
    pdfplumber кладёт текст в первую строку, у продолжений ячейки пустые, и без
    переноса вниз терялось 88 строк: химия II уровня на химфаке, призёры и т. п."""
    out, faculty, napravlenie, above, prev = [], None, None, None, None
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
                    # уровень — своя объединённая ячейка внутри профиля: победитель
                    # и призёр одного уровня, поэтому берётся у предыдущей строки
                    c = [x or (prev[i] if i == 4 else above[i] if i in (1, 2, 3, 5, 7) else x)
                         for i, x in enumerate(c)]
                prev = c
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
    f = "vsosh_list__olymp_disciplines.pdf"
    return msu_vsosh_rows(load_pages("msu", f), meta("msu", f)["url"])


def msu_vsosh_rows(pages: list[dict], url: str) -> list[dict]:
    out, faculty, napravlenie, olympiad = [], None, None, None
    for pg in pages:
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
                    napravlenie, olympiad = c[0], None
                olympiad = c[1] or olympiad
                subject = c[2]
                # «Международная биологическая олимпиада» — не ВсОШ, а сборные.
                if not (napravlenie and subject) or "всероссийская" not in (olympiad or "").lower():
                    continue
                if not profile_slug(subject.lower()):
                    continue
                out.append({"match": {"faculty": faculty, "napravlenie": napravlenie},
                            "vsosh": True, "olympiad_name": None, "profile": subject.lower(),
                            "level": "ВсОШ", "statuses": [POB, PRIZ], "benefit": BVI,
                            "ege_subject": subject, "ege_score": None, "grades": None,
                            "page": pg["page"], "url": url, "score_is_demo": True})
    return out


HSE_VSOSH_CAMPUSES = ("Москва", "Пермь", "Нижний Новгород", "Санкт-Петербург")


def rows_hse_vsosh():
    f = "vsosh_list__1133957386"
    return hse_vsosh_rows(((n, [(c, m[0]) for c, m in rows]) for n, rows in _hse_pages(SNAP / "hse" / f)),
                          meta("hse", f)["url"])


def _hse_vsosh_subject(cell: str) -> tuple[str, str] | None:
    """-> (предмет ВсОШ, предмет ЕГЭ). «Экономика (предоставляется 100 баллов
    по математике)»; «Для дипломов 2026 года выдачи: Информатика (профили …);
    для дипломов, выданных ранее 2026 года: Информатика»."""
    t = clean(cell)
    if "для дипломов" in t.lower():
        t = t.rsplit(":", 1)[-1]
    by = re.search(r"\(предоставляется 100 баллов по ([^)]*)\)", t)
    subject = clean(re.sub(r"\(.*", "", t))
    if not subject or not profile_slug(subject.lower()):
        return None
    return subject, (by.group(1).capitalize() if by else subject)


def hse_vsosh_rows(pages, url: str) -> list[dict]:
    """Приложение 1: ОП -> предметы ВсОШ на БВИ и на 100 баллов, четыре
    таблицы подряд — Москва, Пермь, Нижний Новгород, Петербург (каждая
    начинается шапкой «№ п/п»). № и название ОП — объединённые ячейки на
    блок строк, их текст посередине блока, у блока на стыке страниц номер
    теряется, поэтому ОП строки — по геометрии (_hse_pages). Код направления
    бывает в названии: «09.03.04 Программная инженерия / "Разработка …"»
    (Пермь), «… (по направлению 09.03.04 …)», «… (направление подготовки
    38.03.01 Экономика)».

    pages — [(номер, [(ячейки, ОП строки)])]."""
    out, campus_no = [], -1
    for page_no, rows in pages:
        for c, name in rows:
            c = [clean(x) for x in c]
            if len(c) < 4:
                continue
            if c[0].lower().startswith(("no п/п", "№")):
                campus_no += 1
                continue
            if not 0 <= campus_no < len(HSE_VSOSH_CAMPUSES) or not name:
                continue
            aside = re.search(r"\((?:по )?направлени[^)]*\)", name)
            if "/" in name:
                head, tail = name.split("/", 1)
                codes = CODE_RE.findall(head)
                quoted = re.search(r"[\"«]([^\"»]+)[\"»]", tail)
                program = quoted.group(1) if quoted else tail
            else:
                codes = CODE_RE.findall(aside.group(0)) if aside else []
                program = name.replace(aside.group(0), "") if aside else name
            for col, benefit in ((2, BVI), (3, HUNDRED)):
                subj = _hse_vsosh_subject(c[col])
                if not subj:
                    continue
                out.append({"match": {"campus": HSE_VSOSH_CAMPUSES[campus_no], "codes": codes,
                                      "program": clean(program)},
                            "vsosh": True, "olympiad_name": None, "profile": subj[0].lower(),
                            "level": "ВсОШ", "statuses": [POB, PRIZ], "benefit": benefit,
                            "ege_subject": subj[1], "ege_score": None, "grades": None,
                            "page": page_no, "url": url, "score_is_demo": True})
    return out


DASH = ("", "—", "-", "–")


def _levels(text: str) -> list[str]:
    """«I–III» -> [I, II, III], «II» -> [II]: СПбГУ задаёт уровень точно,
    льгота у уровней разная."""
    t = clean(text).replace("–", "-").replace("—", "-")
    m = re.fullmatch(r"(I{1,3})\s*-\s*(I{1,3})", t)
    order = ["I", "II", "III"]
    if m:
        return order[order.index(m.group(1)):order.index(m.group(2)) + 1]
    return [t] if t in order else []


def rows_spbu():
    f = "olymp_list__bac_spec_olymp_2_2026.pdf"
    return spbu_rows(load_pages("spbu", f), meta("spbu", f)["url"])


def spbu_rows(pages: list[dict], url: str) -> list[dict]:
    """bac_spec_olymp_2_2026.pdf — перечневые олимпиады: 12 колонок, порог
    ЕГЭ отдельной колонкой. «—» вместо названия — любая олимпиада перечня
    этого уровня с указанным профилем, а если и профиль «—», то любая, чей
    профиль соответствует предмету. Это не ВсОШ: ВсОШ — в отдельном
    документе (spbu_vsosh_rows)."""
    out, prog, code = [], None, None
    for pg in pages:
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
                levels = _levels(c[6])
                if not benefit or not prog or not levels:
                    continue
                name, profile = c[7], c[8]
                base = {"match": {"spbu": prog, "code": code}, "levels": levels,
                        "level": levels[-1], "statuses": _statuses(c[10]), "benefit": benefit,
                        "ege_subject": c[9], "ege_score": to_int(c[5]),
                        "grades": None, "page": pg["page"], "url": url}
                if name not in DASH and profile in DASH:
                    # любой профиль этой олимпиады, соответствующий предмету
                    out.append({**base, "by_subject": True, "olympiad_name": name, "profile": c[9]})
                elif name not in DASH:
                    out.append({**base, "olympiad_name": name, "profile": profile})
                elif profile not in DASH:
                    out.append({**base, "by_profile": True, "olympiad_name": None, "profile": profile})
                else:
                    out.append({**base, "by_subject": True, "olympiad_name": None, "profile": c[9]})
    return out


def rows_spbu_vsosh():
    f = "vsosh_list__bac_spec_olymp_1_2026.pdf"
    return spbu_vsosh_rows(load_pages("spbu", f), meta("spbu", f)["url"])


def spbu_vsosh_rows(pages: list[dict], url: str) -> list[dict]:
    """bac_spec_olymp_1_2026.pdf: предмет ВсОШ -> направление, иногда с
    уточнением «(ОП «…»)»; победители и призёры — без ВИ."""
    out = []
    for pg in pages:
        for table in pg["tables"]:
            for row in table:
                c = [clean(x) for x in row]
                if len(c) < 2 or not CODE_RE.match(c[1]) or not profile_slug(c[0].lower()):
                    continue
                op = re.search(r"\(ОП «(.+)»\)", c[1])
                out.append({"match": {"spbu": op.group(1) if op else None, "code": CODE_RE.match(c[1]).group(1)},
                            "vsosh": True, "olympiad_name": None, "profile": c[0].lower(),
                            "level": "ВсОШ", "statuses": [POB, PRIZ], "benefit": BVI,
                            "ege_subject": c[0], "ege_score": None, "grades": None,
                            "page": pg["page"], "url": url, "score_is_demo": True})
    return out


HSE_OLYMP = {"olymp_list__1170753498": "Москва", "olymp_list__1170734747": "Нижний Новгород",
             "olymp_list__1170736201": "Пермь", "olymp_list__1120660311": "Санкт-Петербург"}


def rows_hse():
    """Четыре приложения по кампусам.

    Ширина строки гуляет (11 колонок на первой странице блока, 9-10 на
    продолжениях: pdfplumber выбрасывает пустые ведущие колонки), поэтому
    колонки отсчитываем от якоря «Вид особого права», а не по номеру.
    Приложение сгруппировано по направлению подготовки; имя ОП указано
    только там, где внутри направления их несколько, — в объединённой ячейке,
    которую таблица теряет, поэтому ОП строки берётся по геометрии
    (hse_programs).
    """
    out = []
    for f, campus in HSE_OLYMP.items():
        out += hse_rows(_hse_pages(SNAP / "hse" / f, merged=2), meta("hse", f)["url"], campus)
    return out


HSE_HEAD = re.compile(r"(?:Направлени[ея] подготовки|Специальность)\s+(\d{2}\.\d{2}\.\d{2}.*)")
# Смещения колонок от «Вида особого права», если заголовка таблицы не видно:
# так устроены приложения Москвы, Нижнего Новгорода и Перми.
HSE_LAYOUT = {"status": 2, "grades": 3, "ege": -2}


def _hse_layout(head: list[str]) -> dict:
    """Смещения колонок от «Вида особого права» — по заголовку. В приложении
    Петербурга нет «Предмета зачета 100 баллов»: статус и классы стоят на
    колонку левее, чем у остальных кампусов."""
    low = [x.lower() for x in head]
    h = next(i for i, x in enumerate(low) if x.startswith("вид особого права"))

    def off(*prefixes):
        return next((i - h for p in prefixes for i, x in enumerate(low) if x.startswith(p)), None)
    return {"status": off("кому предоставляется"), "grades": off("в каких классах"),
            "ege": off("предмет егэ, который подтверждает", "один или несколько предметов")}


def hse_rows(pages, url: str, campus: str) -> list[dict]:
    """pages — [(номер, [(ячейки, (ОП, олимпиада) строки)])]. Название
    олимпиады — тоже объединённая ячейка на блок профилей; у блока на стыке
    страниц ячейка в таблице пуста, и перенос сверху подставлял название
    предыдущей олимпиады («Турнир городов» с информатикой и экономикой)."""
    out = []
    codes = name = profile = None
    layout, who, ege = HSE_LAYOUT, "", ""
    for page_no, rows in pages:
        for c, (program, olympiad) in rows:
            c = [clean(x) for x in c]
            if not c:
                continue
            if any(x.lower().startswith("вид особого права") for x in c):
                layout = _hse_layout(c)
                continue
            m = HSE_HEAD.match(c[0])
            if m:
                # «Направления подготовки 01.03.01 Математика; 01.03.04 Прикладная математика»
                codes, name, profile = CODE_RE.findall(m.group(1)), None, None
                continue
            # В пермском приложении «право на …» со строчной буквы.
            j = next((i for i, x in enumerate(c) if x.lower().startswith("право на")), None)
            if j is None or j < 4 or not codes:
                continue
            benefit = _benefit(c[j])
            if not benefit:
                continue
            cell = c[j - 5] if j >= 5 else ""
            # Ячейка надёжнее геометрии (соседний блок иногда подменяет текст),
            # кроме обрывка названия, разбитого по строкам («"Высшая проба"»).
            # Пустая ячейка — геометрия: перенос сверху давал чужую олимпиаду.
            if cell and olympiad and _squash(cell) != _squash(olympiad) and _squash(cell) in _squash(olympiad):
                cell = olympiad
            new_name = cell or olympiad or name
            if new_name != name:
                name, profile, who, ege = new_name, None, "", ""
            if j >= 4 and c[j - 4]:
                profile = c[j - 4]
            if not name or not profile:
                continue

            def col(off):
                return c[j + off] if off is not None and 0 <= j + off < len(c) else ""
            # «Кому» и предмет подтверждения — объединённые ячейки на несколько
            # строк (БВИ за 10–11 класс и 100 баллов за 9–11 у одного профиля):
            # у продолжений они пусты и берутся сверху, а не «всем».
            who = col(layout["status"]) or who
            ege = col(layout["ege"]) or ege
            if not who:
                WARNINGS.append(f"ВШЭ {campus}, стр. {page_no}: нет статуса у «{name}» / {profile}")
                continue
            out.append({
                "match": {"codes": codes, "campus": campus, "program": program or None},
                "olympiad_name": name, "profile": profile, "level": None,
                "statuses": _statuses(who),
                "benefit": benefit,
                "ege_subject": ege,
                "ege_score": _score(c[j - 1]),
                "grades": _grades(col(layout["grades"])),
                "page": page_no, "url": url,
            })
    return out


def hse_programs(top: float, bottom: float, edges: list[float], words: list[dict],
                 row_mids: list[float]) -> list[str]:
    """ОП строки таблицы ВШЭ. Колонка ОП — объединённая ячейка на блок строк,
    блоки разделены горизонтальными отрезками. Текст ячейки напечатан на
    каждой странице, через которую проходит блок, даже за краем страницы
    (y < 0 или y > высоты): слово за краем относится к крайнему блоку."""
    ys = sorted({top, bottom, *[y for y in edges if top < y < bottom]})
    spans = list(zip(ys, ys[1:]))

    def span_of(y):
        y = min(max(y, top), bottom - 0.01)
        return next(i for i, (a, b) in enumerate(spans) if a <= y < b)

    text = defaultdict(list)
    for w in sorted(words, key=lambda w: (round(w["top"]), w["x0"])):
        text[span_of(w["top"])].append(w["text"])
    return [clean(" ".join(text[span_of(y)])) for y in row_mids]


def _hse_pages(pdf_path, merged: int = 1):
    """[(номер, [(ячейки, текст объединённых ячеек строки)])]: колонки 1..merged
    слева (ОП, название олимпиады) — объединённые ячейки, их текст — по
    геометрии (hse_programs), кортежем по колонкам."""
    import pdfplumber
    with pdfplumber.open(pdf_path) as pdf:
        xs = None
        for page in pdf.pages:
            tables = page.find_tables()
            if xs is None and tables:
                xs = sorted({round(cell[0]) for cell in tables[0].cells})   # № | ОП | олимпиада …
            words = page.extract_words()
            rows = []
            for t in tables:
                x0, top, x1, bottom = t.bbox
                mids = [(r.bbox[1] + r.bbox[3]) / 2 for r in t.rows]
                cols = []
                for k in range(1, merged + 1):
                    left, right = xs[k], xs[k + 1]
                    edges = [v for r in page.rects if r["height"] < 12
                             and r["x0"] <= left + 2 and r["x1"] >= right - 2
                             for v in (r["top"], r["bottom"])]
                    inside = [w for w in words if w["x0"] >= left and w["x1"] <= right + 2]
                    cols.append(hse_programs(top, bottom, edges, inside, mids))
                rows += list(zip(t.extract(), zip(*cols)))
            yield page.page_number, rows


MIPT_URL = "https://pk.mipt.ru/bachelor/2026_olympiads/"
MIPT_RULES = "rules__2026_rules"


def rows_mipt():
    return mipt_rows(load_html("mipt", "olymp_list__2026_olympiads"), MIPT_URL)


def mipt_plan(html: str) -> dict[tuple[str, str], dict]:
    """Правила приёма МФТИ: (код направления, программа) -> конкурсная группа
    и предметы ВИ. Одна программа бывает в двух направлениях (ВШБИ — 03.03.01
    и 19.03.01) с разными группами и ВИ, поэтому ключ — с кодом."""
    out, code = {}, None
    for c in html_rows(html):
        m = re.match(r"Направление\s+(\d{2}\.\d{2}\.\d{2})", c[0]) if c else None
        if m:
            code = m.group(1)
        elif len(c) == 3 and code and c[0] and c[0] != "Образовательные программы":
            out[(code, _squash(c[0]))] = {"group": c[1], "exams": subject_keys(c[2])}
    return out


_MIPT_PLAN = None


def _mipt_program(p: dict) -> dict | None:
    global _MIPT_PLAN
    if _MIPT_PLAN is None:
        _MIPT_PLAN = mipt_plan(load_html("mipt", MIPT_RULES))
    return _MIPT_PLAN.get((p["napravlenie_code"], _squash(p["program_name"])))


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
        # Общая колонка «100 баллов по <предмету>»: только программам, где
        # этот предмет — вступительное испытание. В ячейке бывает два предмета.
        for clause in filter(None, map(str.strip, clean(c[4]).split(";"))):
            subject = _subject_from(clause)
            out += _by_grades({"match": {"mipt_exam": subject}, "olympiad_name": name, "profile": profile,
                               "level": level, "statuses": _statuses(clause), "benefit": HUNDRED,
                               "ege_subject": subject, "ege_score": _score(clause),
                               "page": None, "url": url, "number": num}, num)
        if not cols:
            continue
        first, schools = cols
        for school, cell in zip(schools, c[first:first + len(schools)]):
            for cl in _mipt_clauses(school, cell):
                out += _by_grades({**cl, "olympiad_name": name, "profile": profile, "level": level,
                                   "benefit": BVI, "page": None, "url": url, "number": num}, num, school)
    return out


def mipt_grades(num: int, status: str, school: str = "") -> list[int]:
    """П. 4–6 «Порядка»: результат — за 11 класс; победителям «Физтеха»
    (№ 54) — и за 10; победителям олимпиады по ИИ (№ 7) за 10 класс — только
    на программах ВШПИ."""
    if status == POB and (num == 54 or (num == 7 and "ВШПИ" in school)):
        return [10, 11]
    return [11]


def _by_grades(row: dict, num: int, school: str = "") -> list[dict]:
    """Строка на каждую группу статусов с одинаковыми классами."""
    groups: dict[tuple, list[str]] = {}
    for st in row["statuses"]:
        groups.setdefault(tuple(mipt_grades(num, st, school)), []).append(st)
    return [{**row, "statuses": sts, "grades": list(g)} for g, sts in groups.items()]


def _subject_from(text: str) -> str | None:
    t = clean(text).lower()
    for s in ("информатик", "математик", "физик", "хими", "биолог", "обществознани", "экономик",
              "русск", "иностранн"):
        if s in t:
            return {"информатик": "Информатика", "математик": "Математика", "физик": "Физика",
                    "хими": "Химия", "биолог": "Биология", "обществознани": "Обществознание",
                    "экономик": "Экономика", "русск": "Русский язык",
                    "иностранн": "Иностранный язык"}[s]
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
        bvi = {"match": kfu_match(targets or KFU_ALL, d["vi"]), "vsosh": True,
               "olympiad_name": None, "profile": subject.lower(), "level": "ВсОШ",
               "statuses": [POB, PRIZ], "benefit": BVI,
               "ege_subject": d["vi"], "ege_score": None,
               "grades": None, "page": d["page"], "url": url, "score_is_demo": True}
        out += [bvi, {**bvi, "benefit": HUNDRED, "match": {**bvi["match"], "hundred": True}}]
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
                blocks.append([codes, right, page_no])
            first = False
    out = []
    for codes, subjects, page_no in blocks:
        subjects = re.sub(r"\([^)]*\)?", " ", subjects)
        for word in dict.fromkeys(w.lower() for w in re.findall(r"[А-ЯЁ][а-яё]{4,}", subjects)):
            if not profile_slug(word):
                continue
            out.append({"match": {"codes": codes}, "vsosh": True, "olympiad_name": None,
                        "profile": word, "level": "ВсОШ", "statuses": [POB, PRIZ],
                        "benefit": BVI, "ege_subject": word.capitalize(), "ege_score": None,
                        "grades": None, "page": page_no, "url": url, "score_is_demo": True})
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
    out, name, last = [], "", None
    for page_no, row, k in rows:
        # Название — тоже объединённая ячейка: на следующей странице блока
        # («Все из перечня олимпиад школьников») её текста нет.
        if row[1] or group[k] != last:
            name = row[1]
        last = group[k]
        out.append((page_no, [clean(" ".join(merged[group[k]])), name] + row[2:]))
    return out


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
        only = re.search(r"\((?:учитывается )?(?:только )?на [^)]*\)?", profile)
        programs = None
        if only:
            clause = only.group(0)
            codes = CODE_RE.findall(clause) or codes
            if "программ" in clause:
                # «(учитывается только на программе Робототехника и ИИ)»,
                # «(… на программах «Робототехника и ИИ» и «Технологии защиты информации»)»
                names = re.findall(r"«([^»]+)»", clause) or [re.sub(r".*программе\s+|\)$", "", clause)]
                programs = [clean(re.sub(r"\bИИ\b", "искусственный интеллект", n)) for n in names]
            profile = clean(profile.replace(clause, ""))
        # «Все из перечня олимпиад школьников» — не название, а правило:
        # подходит любая перечневая олимпиада этого профиля.
        by_profile = name.lower().startswith("все из перечня")
        out.append({"match": {"codes": codes, "programs": programs},
                    "by_profile": by_profile,
                    "olympiad_name": None if by_profile else name,
                    "profile": profile,
                    "level": _level(level), "statuses": _statuses(status),
                    "benefit": benefit, "ege_subject": subject or profile,
                    "ege_score": 75, "grades": None, "page": page_no, "url": url})
    return out


SECHENOV_OLYMP = "olymp_list__Pravila-priema_2026_2027_BS_pril5_Perechen-olimpiad.pdf"
SECHENOV_EXAMS = "rules__Pravila-priema_2026_2027_BS_pril2_Perechen-VI.pdf"


def rows_sechenov():
    return sechenov_rows(load_pages("sechenov", SECHENOV_OLYMP), meta("sechenov", SECHENOV_OLYMP)["url"])


def sechenov_rows(pages: list[dict], url: str) -> list[dict]:
    """Приложение 5: БВИ по перечневым Сеченов не даёт — только 100 баллов
    (стр. 1), и только на программы, где предмет профиля — среди ВИ.

    Вёрстка: ширина таблиц и сдвиг колонок гуляют, значение бывает задвоено
    в соседние колонки («биология», «биология») — остаётся правая копия.
    Поля считаются справа от «право на …»: предмет ЕГЭ, ВИ, профиль; что
    левее колонки профиля (её даёт строка с №) — куски названия. Два поля —
    тот же профиль с другим предметом. Строка без «права» — продолжение
    многострочных ячеек предыдущей. № — порядковый номер Сеченова, не
    номер перечня: олимпиада ищется по названию."""
    out, name, profile, last, split = [], None, None, [], {}
    for pg in pages:
        for table in pg["tables"]:
            for row in table:
                c = [clean(x) for x in row]
                if len(c) < 4 or any(x.startswith("Профиль олимпиады") for x in c):
                    continue
                vals = [(i, x) for i, x in enumerate(c) if x and not (i + 1 < len(c) and c[i + 1] == x)]
                if not vals:
                    continue
                numbered = re.fullmatch(r"\d+\.?", vals[0][1])
                if numbered:
                    vals = vals[1:]
                    if len(vals) >= 4 and "право" in vals[-1][1].lower():
                        split[len(c)] = vals[-4][0]
                    name, profile, last = "", None, []
                cut = split.get(len(c), 0)
                frags = [x for i, x in vals if i < cut]
                data = [x for i, x in vals if i >= cut]
                if frags and name is not None:
                    name = clean(f"{name} {' '.join(frags)}")
                if not data:
                    continue
                if "право" not in data[-1].lower():
                    keys = {1: ("profile",), 2: ("exam", "subject"), 3: ("profile", "exam", "subject")}.get(len(data), ())
                    for r in last:                # продолжение многострочных ячеек
                        for k, v in zip(keys, data):
                            r[k] = clean(f"{r[k]} {v}")
                    if "profile" in keys and last:
                        profile = last[0]["profile"]
                    continue
                benefit, data = _benefit(data[-1]), data[:-1]
                if len(data) == 3:
                    profile, last = data[0], []
                if len(data) not in (2, 3) or not (name and profile and benefit):
                    continue
                rec = {"name": name, "profile": profile, "exam": data[-2], "subject": data[-1],
                       "benefit": benefit, "page": pg["page"]}
                out.append(rec)
                last.append(rec)
    # Стр. 1: «Результаты победителя (призера) должны быть получены за 10 или 11 класс».
    clause = next((m for pg in pages for m in [re.search(r"получены\s+за\s+([\d\s,иили-]+?)\s*класс",
                                                           clean(pg.get("text") or ""))] if m), None)
    grades = _grades(clause.group(1)) if clause else None
    return [{"match": {"exams": sorted(subject_keys(r["exam"]))}, "olympiad_name": r["name"],
             "profile": r["profile"], "level": None, "statuses": [POB, PRIZ],
             "benefit": r["benefit"], "ege_subject": r["subject"], "ege_score": 75,
             "grades": grades, "page": r["page"], "url": url} for r in out]


def sechenov_exams(pages: list[dict]) -> dict[str, set[str]]:
    """Приложение 2: программа -> предметы общеобразовательных ВИ. Берётся
    первое вхождение программы: дальше — те же программы для других
    категорий поступающих."""
    out, key = {}, None
    for pg in pages:
        for table in pg["tables"]:
            for row in table:
                c = [clean(x) for x in row]
                if len(c) < 4:
                    continue
                m = CODE_RE.match(c[0])
                if m:
                    key = c[0][m.end():].strip()
                    if _squash(key) in out:
                        key = None
                    else:
                        out[_squash(key)] = set()
                elif c[0] and key and not any(c[1:3]):
                    # название продолжается строкой ниже
                    out[_squash(key + " " + c[0])] = out.pop(_squash(key))
                    key = key + " " + c[0]
                if key and c[3]:
                    out[_squash(key)] |= subject_keys(c[3])
    return out


_SECHENOV_EXAMS = None


def sechenov_link(exams: list[str], programs: list[dict]) -> list[dict]:
    global _SECHENOV_EXAMS
    if _SECHENOV_EXAMS is None:
        _SECHENOV_EXAMS = sechenov_exams(load_pages("sechenov", SECHENOV_EXAMS))
    return [p for p in programs if _SECHENOV_EXAMS.get(_squash(p["program_name"]), set()) & set(exams)]


SECHENOV_RULES = "rules__Pravila-priema_2026_2027_BS-s-izmeneniyami-_1_-_2_3.pdf"
KGMU_RIGHTS = "olymp_list__Informaciya_20o_20predostavlenii_20osobyh_20prav_20i_20osobo"


def vsosh_code_rows(pages: list[dict], url: str) -> list[dict]:
    """Таблица «код — специальность — профили ВсОШ» (Сеченов п. 5.1, КГМУ
    п. 6.4): победителям и призёрам — БВИ на направление целиком. Пустая
    ячейка профилей — объединённая, это профили строки выше."""
    out, profiles = [], None
    for pg in pages:
        for table in pg["tables"]:
            for row in table:
                c = [clean(x) for x in row]
                if len(c) < 3 or not re.fullmatch(r"\d{2}\.\d{2}\.\d{2}", c[0]):
                    continue
                profiles = [x.strip().lower() for x in c[2].split(",") if x.strip()] if c[2] else profiles
                for prof in profiles or []:
                    out.append({"vsosh": True, "olympiad_name": None, "profile": prof, "level": "ВсОШ",
                                "match": {"codes": [c[0]]}, "statuses": [POB, PRIZ], "benefit": BVI,
                                "ege_subject": None, "ege_score": None, "grades": None,
                                "page": pg["page"], "url": url})
    return out


def sechenov_vsosh_hundred_rows(pages: list[dict], url: str) -> list[dict]:
    """П. 5.3: особое преимущество ВсОШ — 100 баллов за ВИ, совпадающее с
    профилем (экономика — за обществознание), там, где это ВИ есть."""
    out = []
    for pg in pages:
        for table in pg["tables"]:
            if not table or not clean(table[0][0] or "").startswith("Профиль заключительного этапа"):
                continue
            for row in table[1:]:
                prof, exam = (clean(x or "").lower() for x in row[:2])
                if prof and exam:
                    out.append({"vsosh": True, "olympiad_name": None, "profile": prof, "level": "ВсОШ",
                                "match": {"exams": sorted(subject_keys(exam))},
                                "statuses": [POB, PRIZ], "benefit": HUNDRED,
                                "ege_subject": None, "ege_score": None, "grades": None,
                                "page": pg["page"], "url": url})
    return out


def rows_sechenov_vsosh():
    pages = load_pages("sechenov", SECHENOV_RULES)
    url = meta("sechenov", SECHENOV_RULES)["url"]
    return vsosh_code_rows(pages, url) + sechenov_vsosh_hundred_rows(pages, url)


def rows_kazan_gmu_vsosh():
    return vsosh_code_rows(load_pages("kazan-gmu", KGMU_RIGHTS), meta("kazan-gmu", KGMU_RIGHTS)["url"])


def rows_kazan_gmu():
    f = "olymp_list__download"
    return with_kgmu_rights(kazan_gmu_rows(load_pages("kazan-gmu", f), meta("kazan-gmu", f)["url"]),
                            kgmu_rights(load_pages("kazan-gmu", KGMU_RIGHTS)))


def kgmu_rights(pages: list[dict]) -> dict[str, tuple]:
    """Таблица п. 6.5 «Информации о предоставлении особых прав»: профиль
    олимпиады -> (предметы подтверждения, классы). В перечне олимпиад у
    профиля «медицина» вместо предмета — список укрупнённых групп."""
    out = {}
    for pg in pages:
        for table in pg["tables"]:
            for row in table:
                c = [clean(x or "") for x in row]
                if len(c) < 6 or not c[2] or not re.fullmatch(r"\d{1,2}", c[4]):
                    continue
                out[(canon_subject(c[2]) or c[2]).lower()] = (canon_subject(c[3]), _grades(c[4]))
    return out


def with_kgmu_rights(rows: list[dict], rights: dict[str, tuple]) -> list[dict]:
    out = []
    for r in rows:
        hit = rights.get((canon_subject(r["profile"]) or r["profile"]).lower())
        out.append({**r, "ege_subject": hit[0], "grades": hit[1]} if hit else r)
    return out


def kazan_gmu_rows(pages: list[dict], url: str) -> list[dict]:
    """5 колонок, причём № по перечню указан прямо в документе. Документ
    разбит на секции «Специальность (направление подготовки) – Лечебное
    дело, Педиатрия, …»: строка секции — только этим специальностям, а не
    всему вузу (химия уходила на «Медицинскую биофизику»)."""
    out = []
    num = name = programs = None
    for pg in pages:
        for table in pg["tables"]:
            for row in table:
                c = [clean(x) for x in row]
                if len(c) < 5:
                    continue
                head = re.match(r"Специальность \(направление подготовки\)\s*[–-]\s*(.+)", c[0])
                if head:
                    programs, num, name = [clean(x) for x in head.group(1).split(",")], None, None
                    continue
                if c[0].isdigit():
                    num, name = int(c[0]), c[1]
                if not name or not c[2] or not programs:
                    continue
                out.append({"match": {"names": programs}, "olympiad_name": name, "profile": c[2],
                            "level": _level(c[4]), "statuses": [POB, PRIZ], "benefit": HUNDRED,
                            "ege_subject": c[3], "ege_score": 75, "grades": None,
                            "page": pg["page"], "url": url, "number": num,
                            "benefit_is_demo": True})
    return out


def rows_nsu():
    return nsu_rows(load_html("nsu", "olymp_list__olimpiady-privilege"),
                    "https://www.nsu.ru/n/education/apply-info/olimpiady-privilege/")


def nsu_rows(html: str, url: str) -> list[dict]:
    """Льготы заданы по предмету/профилю олимпиады, а не по её названию."""
    out = []
    chunks = re.split(r'<span class="name line">([^<]+)</span>', html)
    for i in range(1, len(chunks), 2):
        header = clean(chunks[i])
        # Льгота — на направление целиком: его профили в A — отдельные
        # программы («Физика» и «Физическая информатика» в 03.03.02).
        # Заголовок группы «Математика и механика (01.03.00): Математика
        # (01.03.01); …» перечисляет её направления; код группы XX.XX.00 — не
        # направление. Подстрокой по названию нельзя: «Физика. Фундаментальная
        # и экспериментальная физика» совпадала только с программой «Физика».
        codes = [c for c in CODE_RE.findall(header) if not c.endswith(".00")]
        if not codes:
            continue
        for c in html_rows(chunks[i + 1][:60000]):
            if len(c) < 2:
                continue
            benefit = _benefit(c[-1])
            if not benefit:
                continue
            profile = c[0]
            if not profile or "предмет" in profile.lower():
                continue
            base = {"match": {"codes": codes},
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
    pages = load_pages("kfu", KFU_OLYMP)
    grades = kfu_grades(pages)
    num = name = None
    for pg in pages:
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
                bvi = {
                    "match": kfu_match(targets, c[5] or c[3]),
                    "olympiad_name": name, "profile": profile, "level": level,
                    "statuses": [POB, PRIZ], "benefit": BVI,
                    "ege_subject": c[5] or c[3], "ege_score": 75,
                    "grades": grades, "page": pg["page"], "url": url, "number": num,
                }
                out += [bvi, {**bvi, "benefit": HUNDRED, "match": {**bvi["match"], "hundred": True}}]
    return out


def kfu_grades(pages: list[dict]) -> list[int] | None:
    """Стр. 1: «победители и призеры олимпиад школьников за 10 и 11 класс»."""
    for pg in pages:
        m = re.search(r"олимпиад\s+школьников\s+за\s+([\d\s,иили-]+?)\s*класс", clean(pg.get("text") or ""))
        if m:
            return _grades(m.group(1))
    return None


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
    """Кому строка приложения 3 даёт БВИ, а с hundred — 100 баллов: «на
    направления, не соответствующие профилю олимпиады», если предмет
    олимпиады — среди ВИ программы (стр. 1–2, вне зависимости от уровня)."""
    if m.get("hundred"):
        bvi = {p["program_id"] for p in kfu_link({**m, "hundred": False}, programs)}
        subjects = set(m["first_vi"])
        return [p for p in programs if p["program_id"] not in bvi and (info := kfu_program(p))
                and info["vi"] and subjects & set().union(*info["vi"])]
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
           "itmo_vsosh": rows_itmo_vsosh, "spbu_vsosh": rows_spbu_vsosh,
           "sechenov_vsosh": rows_sechenov_vsosh, "kazan-gmu_vsosh": rows_kazan_gmu_vsosh}


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


EGE_STEMS = (("информат", "Информатика"), ("математ", "Математика"), ("физик", "Физика"),
             ("хими", "Химия"), ("биолог", "Биология"), ("обществозн", "Обществознание"),
             ("истори", "История"), ("литератур", "Литература"), ("географ", "География"),
             ("русск", "Русский язык"), ("иностран", "Иностранный язык"),
             ("английск", "Иностранный язык"), ("немецк", "Иностранный язык"),
             ("французск", "Иностранный язык"), ("испанск", "Иностранный язык"),
             ("китайск", "Иностранный язык"))


def canon_subject(subject: str) -> str | None:
    """Предметы ЕГЭ из ячейки, в порядке упоминания, через «или»:
    «физика / информатика» -> «Физика или Информатика». Экстракция рвёт
    слово на границе колонки («Обществоз нанию»), поэтому ищем по основам в
    тексте без пробелов. Не предмет ЕГЭ (обрывок соседней колонки,
    «клиническая медицина…», «—») -> None: в примечание мусор не идёт."""
    flat = re.sub(r"\s+", "", clean(subject or "").lower().replace("ё", "е"))
    found = sorted((flat.find(stem), name) for stem, name in EGE_STEMS if stem in flat)
    return " или ".join(dict.fromkeys(name for _, name in found)) or None


# Граница B — каталог продукта: олимпиады датасета C и missing_in_C (их сид
# добавляет сам). Других сид не знает, а у олимпиад каталога льготы терять
# нельзя: раньше границу держали слова в профиле, и «вероятность и
# статистика» или «Физтех» научно-технический выпадали, хотя в C они есть.
_CATALOG: set[str] | None = None


def in_catalog(oid: str) -> bool:
    global _CATALOG
    if _CATALOG is None:
        c = json.loads((DATA / "olimpiady_spravochnik.json").read_text(encoding="utf-8"))["olimpiady"]
        m = json.loads((DATA / "missing_in_C.json").read_text(encoding="utf-8"))["missing_in_C"]
        _CATALOG = {x["olympiad_id"] for x in c + m}
    return oid in _CATALOG


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


def _squash(name: str) -> str:
    """Название ОП только из букв и цифр, без «(реализуется на …)»: в A и в
    приложениях ВШЭ слова рвутся по-разному («Информацион- ная», «Sociolo gy»)."""
    n = re.sub(r"\(реализуется[^)]*\)?", "", clean(name).lower().replace("ё", "е"))
    return re.sub(r"[^a-zа-я0-9]", "", n)


def _same_program(a: str, b: str) -> bool:
    """Точное совпадение ОП, в том числе с одной из частей через «/»:
    «Международная программа по бизнесу и экономике/ International Program …».
    Не по префиксу: «Экономика» — не «Экономика и статистика»."""
    def names(x):
        return {_squash(x)} | {_squash(part) for part in x.split("/")} - {""}
    return bool(names(a) & names(b))


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
    if "spbu" in m:                                 # СПбГУ: ОП внутри направления, только точно
        pool = [p for p in programs if p["napravlenie_code"] == m["code"]]
        if not m["spbu"]:
            return pool
        return [p for p in pool if _norm_prog(p["program_name"]) == _norm_prog(m["spbu"])
                or _same_program(p["program_name"], m["spbu"])]
    if "exams" in m:                                # Сеченов: предмет — среди ВИ программы
        return sechenov_link(m["exams"], programs)
    if "names" in m:                                # КГМУ: секция перечисляет специальности
        return [p for p in programs if any(_same_program(p["program_name"], n) for n in m["names"])]
    if "mipt_exam" in m:                            # МФТИ, 100 баллов: предмет — среди ВИ программы
        return [p for p in programs
                if m["mipt_exam"] in (_mipt_program(p) or {}).get("exams", set())]
    if "school" in m:                               # МФТИ: льгота адресована физтех-школе
        return mipt_link(m["school"], m.get("groups"), programs)
    if m.get("kfu"):                                # КФУ: коды, профили, «первое ВИ», «кроме»
        return kfu_link(m, programs)
    if m.get("campus"):                             # ВШЭ: направление внутри кампуса
        campus = [p for p in programs if m["campus"] in (p["faculty"] or "")]
        pool = [p for p in campus if not m["codes"] or p["napravlenie_code"] in m["codes"]]
        if not m.get("program"):
            return pool                             # у направления одна ОП
        named = [p for p in pool if _same_program(p["program_name"], m["program"])]
        # Опечатка в коде заголовка («37.04.01 Психология» в Нижнем Новгороде
        # вместо 37.03.01): ОП с тем же названием в кампусе — та самая.
        return named or [p for p in campus if _same_program(p["program_name"], m["program"])]
    if "codes" in m:                                # ИТМО: перечислены коды направлений
        if m.get("all"):
            return programs
        pool = [p for p in programs if p["napravlenie_code"] in m["codes"]]
        if m.get("programs"):                       # «(учитывается только на программе …)»
            pool = [p for p in pool if any(_same_program(p["program_name"], n) for n in m["programs"])]
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
        napr = m.get("napravlenie", "") or ""
        code = CODE_RE.search(napr)
        if code:
            return [p for p in pool if p["napravlenie_code"] == code.group(1)]
        return [p for p in pool if msu_same_direction(napr, p)]
    return []


def _msu_split(name: str) -> tuple[str, str | None]:
    """«Фундаментальная и прикладная биология (группа программ «Физико-
    химическая биология. Общая биология»)», «Менеджмент (Менеджмент в
    спорте)», в A — «… — Физико-химическая биология. Общая биология)»,
    «Экономика (профиль Государственный и муниципальный аудит)» ->
    (направление, уточнение)."""
    t = clean(name)
    m = re.match(r"(.+?)\s+[—(]\s*(.*)$", t)
    if not m:
        return _squash(t), None
    sub = m.group(2).rstrip(")")
    q = re.search(r"«(.+)»", sub)
    sub = q.group(1) if q else re.sub(r"^(группа программ|профиль)\s*", "", sub)
    return _squash(m.group(1)), _squash(sub) or None


def msu_same_direction(napr: str, p: dict) -> bool:
    """Направление из документа МГУ — программа A того же факультета.
    Основа названия совпадает точно; уточнение документа (группа программ,
    профиль) обязано совпасть с уточнением программы. Подстрокой нельзя:
    «Биоинженерия и биотехнология. Биофизика» — не ФХБ, «Менеджмент в
    спорте» — не «Менеджмент в культуре». Не нашлось — льготы нет, а не
    льгота всему факультету (так ВсОШ по биологии для «Психологии» в
    Севастополе уходила на ПМИ)."""
    base, sub = _msu_split(napr)
    a_base, a_sub = _msu_split(p["program_name"])
    if base != a_base and base != _msu_split(p["napravlenie_name"] or "")[0]:
        return False
    return sub is None or sub == a_sub


def _school_tags(text: str) -> set[str]:
    """«ФБМФ/ ВШБИ», «ФБМФ, ВШБИ» -> {ФБМФ, ВШБИ}; «ВШ М» -> {ВШМ}."""
    return {t for t in re.split(r"[,/]", re.sub(r"\s+", "", text or "")) if t}


def mipt_link(school: str, groups: list[str] | None, programs: list[dict]) -> list[dict]:
    """Программы физтех-школы. У ФБМФ и ВШБИ факультет в A общий, а школа —
    в скобках названия: «Все конкурсные группы ФБМФ» ВШБИ не касается.
    Названные конкурсные группы сужают школу. Группа — это колонка
    «Конкурсная группа» правил приёма или название самой программы, точно:
    подстрокой «Системное программирование и прикладная математика» ловила
    программу «Математика» из группы ПМИ. Ячейка может назвать две программы
    через «и» («Авиационные технологии и беспилотные авиационные системы»)."""
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
    def fits(p, g):
        own = {_norm_prog(p["program_name"]), _norm_prog((_mipt_program(p) or {}).get("group", ""))} - {""}
        return any(g == o or SequenceMatcher(None, g, o).ratio() > 0.9 for o in own)

    out = []
    for g in map(_norm_prog, groups):
        hit = [p for p in pool if fits(p, g)]
        parts = g.split(" и ")
        for i in range(1, len(parts)):
            if hit:
                break
            a = [p for p in pool if fits(p, " и ".join(parts[:i]))]
            b = [p for p in pool if fits(p, " и ".join(parts[i:]))]
            hit = a + b if a and b else []
        out += [p for p in hit if p not in out]
    return out


def expand_profile(profile: str, min_level: str | None) -> list[tuple[str, str, str]]:
    """Вуз задал профиль, а не название: подходит любая перечневая олимпиада
    с этим профилем. Уровень I сильнее II и III, поэтому требование «не ниже
    уровня N» пропускает олимпиады с уровнем N и выше.

    Если точного профиля нет, вуз мог назвать часть составного профиля НТО
    («Аэрокосмические системы» — из «беспилотный транспорт: аэрокосмические
    системы, …»), чуть иначе его записать («… финансовых технологий») или
    оборвать («виртуальные миры: …, технологии дополненной реальности»)."""
    want = re.sub(r"(\w)- (\w)", r"\1-\2", clean(profile).lower())   # перенос «бизнес- процессов»
    rank = {"I": 1, "II": 2, "III": 3}
    cap = rank.get(min_level or "III", 3)

    def fits(row, loose):
        have = row["profile"].strip().lower()
        if not loose:
            return have == want
        parts = [clean(x) for x in re.split(r"[:,]", have)]
        return (want in parts or SequenceMatcher(None, want, have).ratio() > 0.9
                or (len(want) >= 20 and have.startswith(want)))   # профиль без хвоста

    for loose in (False, True):
        hits = [(row["olympiad_id"], row["name"], row["level"]) for row in _index
                if fits(row, loose) and rank.get(row["level"], 3) <= cap]
        if hits or any(fits(row, loose) for row in _index):
            return hits
    return []


def expand_subject(subject: str, name: str | None = None) -> list[tuple[str, str, str]]:
    """Любая олимпиада перечня (или профиль названной), профиль которой
    соответствует предмету (графа «общеобразовательные предметы» перечня)."""
    want = subject_keys(subject)
    num = match_number(name) if name else None
    if name and num is None:
        return []
    # Только олимпиады каталога: иначе «любая, соответствующая математике»
    # тянет лингвистику и основы государственности.
    return [(row["olympiad_id"], row["name"], row["level"]) for row in _index
            if want & subject_keys(" ".join(row["subjects"])) and in_catalog(row["olympiad_id"])
            and (num is None or row["perechen_number_669"] == num)]


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


WARNINGS: list[str] = []


def program_exams(vuz_id: str, p: dict) -> set[str] | None:
    """Предметы ВИ программы — где вуз их публикует и они разобраны."""
    if vuz_id == "mipt":
        return (_mipt_program(p) or {}).get("exams") or None
    if vuz_id == "kfu":
        plan = kfu_program(p)
        return set().union(*plan["vi"]) if plan and plan["vi"] else None
    if vuz_id == "sechenov":
        global _SECHENOV_EXAMS
        if _SECHENOV_EXAMS is None:
            _SECHENOV_EXAMS = sechenov_exams(load_pages("sechenov", SECHENOV_EXAMS))
        return _SECHENOV_EXAMS.get(_squash(p["program_name"])) or None
    return None


def narrow_subject(subject: str | None, exams: set[str] | None) -> str | None:
    """Документ называет предметы профиля («математика, обществознание»), а
    подтверждать надо тот, что среди ВИ программы. Нет пересечения или ВИ
    неизвестны — как в документе."""
    if not subject or not exams:
        return subject
    keep = [s for s in subject.split(" или ") if s in exams]
    return " или ".join(keep) if keep else subject


def dedup_benefits(records: list[dict]) -> list[dict]:
    """Одна (олимпиада, статус, льгота) из нескольких строк документа — БВИ за
    10 класс и за 11 отдельными строками: классы объединяются (None — без
    ограничения — поглощает остальные), порог ЕГЭ берётся строже."""
    out: dict[tuple, dict] = {}
    for b in records:
        key = (b["olympiad_id"], b["diploma_status"], b["benefit_type"])
        a = out.get(key)
        if a is None:
            out[key] = dict(b)
            continue
        ga, gb = a.get("eligible_grades"), b.get("eligible_grades")
        a["eligible_grades"] = None if ga is None or gb is None else sorted(set(ga) | set(gb))
        sa, sb = a.get("ege_confirm_min_score"), b.get("ege_confirm_min_score")
        if sb is not None and (sa is None or sb > sa):
            a["ege_confirm_min_score"] = sb
    return list(out.values())


def conditions(oid: str, row: dict, vuz_id: str, prog: dict) -> dict:
    """Подтверждение ЕГЭ и пометка is_demo. ВсОШ результатом ЕГЭ не
    подтверждается (ч. 4 ст. 71 273-ФЗ; порог 75 по ч. 12 — только для
    олимпиад школьников), поэтому пустой порог у неё — факт, а не заглушка."""
    if oid.startswith("vsosh-"):
        return {"ege_confirm_subject": None, "ege_confirm_min_score": None,
                "is_demo": bool(row.get("benefit_is_demo"))}
    return {
        "ege_confirm_subject": narrow_subject(canon_subject(row.get("ege_subject")),
                                              program_exams(vuz_id, prog)),
        "ege_confirm_min_score": row.get("ege_score"),
        "is_demo": bool(row.get("score_is_demo") or row.get("benefit_is_demo")
                        or row.get("ege_score") is None),
    }


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
            if row.get("by_profile") or row.get("by_subject"):
                variants = (expand_subject(row["profile"], row.get("olympiad_name")) if row.get("by_subject") else
                            expand_profile(row.get("profile") or "", row.get("level")))
                if row.get("levels"):             # СПбГУ: уровень задан точно
                    variants = [v for v in variants if v[2] in row["levels"]]
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
            variants = [v for v in variants if in_catalog(v[0])]
            if not variants:
                skipped[vuz_id] += 1
                reasons[vuz_id]["олимпиады нет в каталоге (C и missing_in_C)"] += 1
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
                        **conditions(oid, row, vuz_id, prog),
                        "source_url": row["url"],
                        "source_page": row.get("page"),
                        "source_date": fetched_date(row["url"]),
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
        benefits = dedup_benefits(per_program.get(x["program_id"], []))
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

    for w in WARNINGS:
        print("  ! " + w)
    total = sum(len(o["prinimaemye_olimpiady"]) for o in objects)
    empty = sum(1 for o in objects if o["status"] == "to_check")
    print(f"\nпрограмм: {len(objects)}, записей о льготах: {total}, без льгот (to_check): {empty}")
    write_json(DATA / "vuz_napravlenie_olimpiady.json", "vuz_napravlenie_olimpiady", objects,
               "Датасет B по data_format_spec_4.md. Ключ связи с A — program_id. "
               "Одна запись = одна пара (olympiad_id, diploma_status).")
    return 0


if __name__ == "__main__":
    sys.exit(main())
