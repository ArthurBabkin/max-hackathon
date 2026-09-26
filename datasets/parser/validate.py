#!/usr/bin/env python3
"""Проверка датасетов A и B по чек-листу раздела 5 спецификации.

Выходной код 1, если нарушено хоть одно жёсткое требование.
"""
import json, re, sys
from collections import Counter, defaultdict
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))
from common import DATA, ROOT

VUZY = {"msu", "spbu", "hse", "mipt", "itmo", "nsu", "kfu", "innopolis", "sechenov", "kazan-gmu"}
GROUPS = {"ИТ", "Физика", "Биомед", "Экономика"}
BENEFITS = {"БВИ", "100_ballov", "dop_bally"}
STATUSES = {"pobeditel", "prizyor"}
LEVELS = {"I", "II", "III", "ВсОШ", None}
OID_RE = re.compile(r"^(p669-\d+|vsosh)-[a-z0-9-]+$")

errors, warnings = [], []

# Эталонные факты: программа, олимпиада и что она даёт — {(статус, льгота)},
# пустое множество — льготы быть не должно. Каждый сверен с первоисточником
# вручную; это регрессия на ошибки привязки, из-за которых льгота уходила
# чужим программам (задача #68). Структурные проверки их не ловят.
POB, PRIZ = "pobeditel", "prizyor"
GOLDEN = [
    # МГУ, olymp_disciplines.pdf стр. 43: в Севастополе биология — только «Психология».
    ("msu__prikladnaya-matematika-i-informatika-filial-mgu-v-g-sev", "vsosh-biologiya", set()),
    ("msu__prikladnaya-matematika-i-informatika-filial-mgu-v-g-sev", "vsosh-matematika",
     {(POB, "БВИ"), (PRIZ, "БВИ")}),
    # МФТИ, 2026_olympiads: «Физтех» по биологии — ФБМФ/ВШБИ и ФБВТ, не ЛФИ.
    ("mipt__obschaya-i-prikladnaya-fizika", "p669-54-biologiya", set()),
    ("mipt__biofizika-i-bioinformatika-fbmf", "p669-54-biologiya", {(POB, "БВИ")}),
    # МФТИ, приложение 2: ВсОШ по биологии на ФПМИ — только «ЕКН» и «ММТУ».
    ("mipt__prikladnaya-matematika-i-informatika", "vsosh-biologiya", set()),
    ("mipt__estestvennye-i-kompyuternye-nauki", "vsosh-biologiya", {(POB, "БВИ"), (PRIZ, "БВИ")}),
    # МФТИ, 2026_olympiads: «100 баллов по физике/химии» — только где предмет
    # среди ВИ (2026_rules): у ПМИ ВИ математика, информатика, русский; у ЛФИ
    # химии нет; у Биотехнологии ФБМФ математика есть.
    ("mipt__prikladnaya-matematika-i-informatika", "p669-54-fizika", set()),
    ("mipt__obschaya-i-prikladnaya-fizika", "p669-54-himiya", set()),
    ("mipt__biotehnologiya-fbmf", "p669-54-matematika", {(POB, "БВИ"), (POB, "100_ballov"), (PRIZ, "100_ballov")}),
    # МФТИ: Всесиб по математике — БВИ группе «Системное программирование и
    # прикладная математика»; программа «Математика» — в группе ПМИ, ей 100 баллов.
    ("mipt__matematika", "p669-14-matematika", {(POB, "100_ballov"), (PRIZ, "100_ballov")}),
    ("mipt__sistemnoe-programmirovanie-i-prikladnaya-matemat", "p669-14-matematika",
     {(POB, "БВИ"), (POB, "100_ballov"), (PRIZ, "100_ballov")}),
    # ИТМО, vsosh_2026.pdf стр. 2: физика — 03.03.02; «Инноватика» — в блоке химии и биологии.
    ("itmo__teoreticheskaya-i-eksperimentalnaya-fizika", "vsosh-fizika", {(POB, "БВИ"), (PRIZ, "БВИ")}),
    ("itmo__tehnologii-i-innovacii", "vsosh-biologiya", {(POB, "БВИ"), (PRIZ, "БВИ")}),
    # ВШЭ, Москва, стр. 1: Всесибирская по информатике на «Математике» — 100 баллов победителю.
    ("hse__matematika", "p669-14-informatika", {(POB, "100_ballov")}),
    # КГМУ, стр. 1 и 3: Сеченовская по химии — первой секции, не «Медицинской биофизике».
    ("kazan-gmu__lechebnoe-delo", "p669-11-himiya", {(POB, "100_ballov"), (PRIZ, "100_ballov")}),
    ("kazan-gmu__medicinskaya-biofizika", "p669-11-himiya", set()),
    # КФУ, приложение 3 стр. 29: «Физтех» по физике — БВИ там, где физика —
    # первое ВИ; где физика — ВИ, но не первое, — 100 баллов (стр. 1–2).
    ("kfu__prikladnaya-matematika-i-informatika", "p669-54-fizika", {(POB, "100_ballov"), (PRIZ, "100_ballov")}),
    ("kfu__astrofizika-i-kosmologiya", "p669-54-fizika", {(POB, "БВИ"), (PRIZ, "БВИ")}),
    # ВШЭ, Санкт-Петербург, стр. 16: «Финатлон» на «Бизнес-информатике» —
    # «Победителям». В приложении нет колонки «Предмет зачета 100 баллов»,
    # статус читался из колонки классов и давал льготу и призёрам.
    ("hse__biznes-informatika-380305", "p669-8-finansovaya-gramotnost", {(POB, "БВИ")}),
    # НГУ, olimpiady-privilege: льготы — на направление целиком. «Математика и
    # механика» — это 01.03.01–03, «Физика» (03.03.02) — и «Физическая информатика».
    ("nsu__prikladnaya-matematika", "vsosh-matematika", {(POB, "БВИ"), (PRIZ, "БВИ")}),
    ("nsu__prikladnaya-matematika", "vsosh-biologiya", set()),
    ("nsu__fizicheskaya-informatika", "vsosh-fizika", {(POB, "БВИ"), (PRIZ, "БВИ")}),
    # НГУ, «Биология» (06.03.01): матрица «степень × уровень», «Без экзам.» —
    # БВИ; ПМФ (03.03.01): «Информатика и ИКТ — Без экзаменов».
    ("nsu__obschaya-biologiya", "p669-50-biologiya", {(POB, "БВИ"), (PRIZ, "БВИ")}),
    ("nsu__molekulyarnaya-biologiya", "p669-8-matematika", {(POB, "100_ballov"), (PRIZ, "100_ballov")}),
    ("nsu__prikladnye-matematika-i-fizika", "p669-37-informatika", {(POB, "БВИ"), (PRIZ, "БВИ")}),
    # Граница B — каталог продукта (C и missing_in_C), а не слова в профиле:
    # Сеченов, приложение 5 стр. 7: «29. Московская олимпиада школьников
    # вероятность и статистика математика математика право на 100 баллов»;
    # МФТИ, 2026_olympiads: «Физтех» научно-технический — ЛФИ БВИ победителям,
    # 100 баллов по физике победителям и призёрам.
    ("sechenov__mehanika-i-matematicheskoe-modelirovanie", "p669-37-veroyatnost-i-statistika",
     {(POB, "100_ballov"), (PRIZ, "100_ballov")}),
    ("mipt__obschaya-i-prikladnaya-fizika", "p669-54-nauchno-tehnicheskiy",
     {(POB, "БВИ"), (POB, "100_ballov"), (PRIZ, "100_ballov")}),
    # Сеченов, Правила стр. 13 (п. 5.1): ВсОШ — БВИ по таблице «код — профили»:
    # «31.05.01 Лечебное дело Химия, Биология», «38.03.02 Менеджмент Математика,
    # Обществознание, Экономика». Стр. 14 (п. 5.3): 100 баллов за ВИ по профилю —
    # физика у Биотехнологии среди ВИ, обществознания у Менеджмента нет.
    ("sechenov__lechebnoe-delo", "vsosh-biologiya",
     {(POB, "БВИ"), (PRIZ, "БВИ"), (POB, "100_ballov"), (PRIZ, "100_ballov")}),
    ("sechenov__menedzhment", "vsosh-ekonomika", {(POB, "БВИ"), (PRIZ, "БВИ")}),
    ("sechenov__biotehnologiya", "vsosh-fizika", {(POB, "100_ballov"), (PRIZ, "100_ballov")}),
    # КГМУ, «Информация о предоставлении особых прав» стр. 2 (п. 6.4):
    # 30.05.01–34.03.01 — «Химия, биология, русский язык»; «30.05.02 Медицинская
    # биофизика — Физика, математика, биология, русский язык», химии нет.
    ("kazan-gmu__stomatologiya", "vsosh-biologiya", {(POB, "БВИ"), (PRIZ, "БВИ")}),
    ("kazan-gmu__medicinskaya-biofizika", "vsosh-fizika", {(POB, "БВИ"), (PRIZ, "БВИ")}),
    ("kazan-gmu__medicinskaya-biofizika", "vsosh-himiya", set()),
    # МФТИ, приложение 2: ВсОШ по информатике на ФРКТ — только группа «КТВТ»;
    # «Все конкурсные группы ФРКТ» — у подпрофилей ИБ и робототехники.
    ("mipt__radiotehnika-i-kompyuternye-tehnologii", "vsosh-informatika", set()),
    # Иннополис, Правила п. 57–62 и приказ от 19.01.2026: приложение 3 — БВИ и
    # 100 баллов по предмету графы 5 (Innopolis Open — «информатика»); ВсОШ —
    # БВИ и особое преимущество по графе 3 приложения 2.
    ("innopolis__informatika-i-vychislitelnaya-tehnika", "p669-22-informatika",
     {(POB, "БВИ"), (PRIZ, "БВИ"), (POB, "100_ballov"), (PRIZ, "100_ballov")}),
    ("innopolis__informatika-i-vychislitelnaya-tehnika", "vsosh-fizika",
     {(POB, "БВИ"), (PRIZ, "БВИ"), (POB, "100_ballov"), (PRIZ, "100_ballov")}),
    # СПбГУ, приложение 9 (bac_spec_olymp_2_2026.pdf): «Победитель, призѐр» — обоим
    # статусам (стр. 1); колонка «предмет или УГН» точная: УГН «математика и
    # механика» не «Математика» (стр. 5), «Астрономия» — своя строка (стр. 2).
    ("spbu__prikladnaya-matematika-processy-upravleniya-i-is", "p669-50-matematika",
     {(POB, "БВИ"), (PRIZ, "БВИ")}),
    ("spbu__matematika-s-dopolnitelnoy-kvalifikaciey-special", "p669-64-informatika",
     {(POB, "БВИ"), (PRIZ, "100_ballov")}),
    ("spbu__ekonomika", "p669-26-matematika", {(POB, "100_ballov"), (PRIZ, "100_ballov")}),
    ("spbu__ekonomika", "p669-50-mehanika-i-matematicheskoe-modelirovanie", set()),
    ("spbu__programmnaya-inzheneriya", "p669-8-inzhenernye-nauki", set()),
    ("spbu__lechebnoe-delo", "p669-36-estestvennye-nauki", set()),
    ("spbu__astronomiya", "p669-37-astronomiya", {(POB, "БВИ"), (PRIZ, "БВИ")}),
    ("spbu__prikladnye-kompyuternye-tehnologii-i-iskusstvenn", "p669-5-yadernye-tehnologii",
     {(POB, "БВИ"), (PRIZ, "БВИ")}),
    ("spbu__lechebnoe-delo", "p669-59-medicina", {(POB, "БВИ"), (PRIZ, "БВИ")}),
    # СПбГУ, Правила п. 7.5 и приложение 7 (стр. 19, 78): ВсОШ — 100 баллов за
    # соответствующее ВИ и там, где БВИ по приложению 4 нет; где БВИ есть — только оно.
    ("spbu__programmnaya-inzheneriya", "vsosh-matematika", {(POB, "100_ballov"), (PRIZ, "100_ballov")}),
    ("spbu__fundamentalnaya-i-prikladnaya-fizika", "vsosh-matematika", {(POB, "БВИ"), (PRIZ, "БВИ")}),
    # ИТМО, приложение 6 (rsosh_100): 100 баллов — по дипломам, «не дающие право
    # поступления без вступительных испытаний»; уровень — точное множество:
    # «Техника и технологии … 2 или 3» в приложении 5 не про «Звезду» I уровня.
    ("itmo__kompyuternye-sistemy-i-tehnologii", "p669-37-matematika", {(POB, "БВИ"), (PRIZ, "БВИ")}),
    ("itmo__ai360-ml-native", "p669-50-informatika", {(POB, "БВИ"), (PRIZ, "100_ballov")}),
    ("itmo__fizika-nanostruktur", "p669-36-tehnika-i-tehnologii", {(POB, "100_ballov"), (PRIZ, "100_ballov")}),
    # ВШЭ, Москва: экономика на МП «Экономика и финансы» — строка олимпиады
    # Кондратьева (её нет в перечне №669), а не Московской (стр. 94);
    # «Физтех» в кавычках “ ” на «Физике» (стр. 26); ОММО (№41) — только 100 баллов
    # победителю, БВИ шли от «Будущего Сибири» (стр. 27); строки «Юриспруденции»
    # (стр. 110) — не ОП «Правовое регулирование бизнеса».
    ("hse__mezhdunarodnaya-programma-po-ekonomike-i-finansa", "p669-37-ekonomika", set()),
    ("hse__fizika", "p669-54-fizika", {(POB, "БВИ"), (PRIZ, "БВИ"), (POB, "100_ballov"), (PRIZ, "100_ballov")}),
    ("hse__fizika", "p669-41-fizika", {(POB, "100_ballov")}),
    ("hse__yurisprudenciya-pravovoe-regulirovanie-biznesa-o", "p669-8-obschestvoznanie", set()),
]

# Эталонные условия: у всех записей пары (программа, олимпиада) поле равно
# ожидаемому. Классы и предмет подтверждения — то, что видит ученик в карточке.
GOLDEN_CONDITIONS = [
    # КФУ, приложение 3 стр. 1: «победители и призеры олимпиад школьников за 10 и 11 класс».
    ("kfu__biologiya", "p669-8-biologiya", "eligible_grades", [10, 11]),
    ("kfu__biologiya", "vsosh-biologiya", "eligible_grades", None),
    # Сеченов, приложение 5 стр. 1: «должны быть получены за 10 или 11 класс».
    ("sechenov__lechebnoe-delo", "p669-11-biologiya", "eligible_grades", [10, 11]),
    # КГМУ, «Информация о предоставлении особых прав» стр. 3–4, таблица п. 6.5:
    # «Класс обучения» — 11; профиль «Медицина» — «Химия / Биология».
    ("kazan-gmu__lechebnoe-delo", "p669-59-medicina", "eligible_grades", [11]),
    ("kazan-gmu__lechebnoe-delo", "p669-59-medicina", "ege_confirm_subject", "Химия или Биология"),
    ("kazan-gmu__lechebnoe-delo", "p669-11-himiya", "eligible_grades", [11]),
    # МФТИ, 2026_olympiads п. 4: «Результат олимпиады должен быть получен в
    # олимпиаде за 11 класс»; п. 5 — победителям «Физтеха» и за 10 класс.
    ("mipt__programmnaya-inzheneriya", "p669-50-informatika", "eligible_grades", [11]),
    ("mipt__obschaya-i-prikladnaya-fizika", "vsosh-fizika", "eligible_grades", None),
    # НГУ, 15.03.06: сноска «обучавшихся в период участия в олимпиаде в 9-11 класс».
    ("nsu__deep-robotics", "p669-37-informatika", "eligible_grades", [9, 10, 11]),
    # СПбГУ, приложение 9 стр. 1: «только при получении результатов за 10 или 11
    # класс»; порог — объединённая ячейка ВИ, предмет — ВИ строки (стр. 4, 6).
    ("spbu__lechebnoe-delo", "p669-62-himiya", "eligible_grades", [10, 11]),
    ("spbu__lechebnoe-delo", "p669-62-himiya", "ege_confirm_min_score", 75),
    ("spbu__lechebnoe-delo", "p669-59-medicina", "ege_confirm_subject", "Биология"),
    ("spbu__biznes-informatika", "p669-8-finansovaya-gramotnost", "ege_confirm_subject", "Обществознание"),
    ("spbu__biznes-informatika", "p669-8-finansovaya-gramotnost", "ege_confirm_min_score", 75),
    ("spbu__ai360-matematika-mashinnogo-obucheniya", "p669-81-matematika", "ege_confirm_min_score", 85),
    # ИТМО, заголовки приложений 5 и 6: дипломы, «полученные в 10-м или 11-м классе».
    ("itmo__ai360-ml-native", "p669-26-informatika", "eligible_grades", [10, 11]),
    # ВШЭ, Москва, стр. 7: порог 90 — одна ячейка на строки БВИ и 100 баллов.
    ("hse__prikladnaya-matematika-i-informatika", "p669-8-informatika", "ege_confirm_min_score", 90),
]
GOLDEN_STATUS_GRADES = [
    ("mipt__obschaya-i-prikladnaya-fizika", "p669-54-fizika", POB, [10, 11]),
    ("mipt__obschaya-i-prikladnaya-fizika", "p669-54-fizika", PRIZ, [11]),
]

# Предметы ЕГЭ: ege_confirm_subject — один из них или несколько через «или».
EGE_SUBJECTS = {"Математика", "Информатика", "Физика", "Химия", "Биология", "Обществознание", "История",
                "Литература", "География", "Русский язык", "Иностранный язык"}


def err(msg):
    errors.append(msg)


def warn(msg):
    warnings.append(msg)


def main() -> int:
    a = json.loads((DATA / "vuz_napravleniya.json").read_text(encoding="utf-8"))["vuz_napravleniya"]
    b = json.loads((DATA / "vuz_napravlenie_olimpiady.json").read_text(encoding="utf-8"))["vuz_napravlenie_olimpiady"]
    index = json.loads((DATA / "olympiad_index_669.json").read_text(encoding="utf-8"))["olympiads"]
    known_ids = {r["olympiad_id"] for r in index}
    vsosh_ok = {f"vsosh-{s['slug']}" for s in
                json.loads((DATA / "subject_slugs.json").read_text(encoding="utf-8"))["slugs"]}

    # --- Датасет A
    offered = [x for x in a if x["status"] == "offered"]
    for x in a:
        if x["vuz_id"] not in VUZY:
            err(f"A: неизвестный vuz_id {x['vuz_id']}")
        if x["profile_group"] not in GROUPS:
            err(f"A: неизвестный profile_group {x['profile_group']}")
        if x["admission_year"] != 2026:
            err(f"A: admission_year != 2026 у {x.get('program_id')}")
        if "diploma_valid_years" in x:
            err("A: поле diploma_valid_years не должно существовать")
        if not x.get("source_url"):
            err(f"A: нет source_url у {x['vuz_id']}/{x['profile_group']}")
        if x["status"] == "offered":
            if not x.get("program_id"):
                err(f"A: пустой program_id при offered ({x['vuz_id']})")
            if not x.get("faculty"):
                err(f"A: пустой faculty при offered ({x.get('program_id')})")
            if x.get("match_type") not in ("exact", "adjacent"):
                err(f"A: не проставлен match_type у {x.get('program_id')}")
            if x.get("match_type") == "adjacent" and not x.get("matched_reason"):
                err(f"A: adjacent без matched_reason у {x.get('program_id')}")
            if str(x["source_url"]).lower().endswith(".pdf") and x.get("source_page") is None:
                warn(f"A: PDF-источник без source_page у {x.get('program_id')}")

    # program_id идентифицирует программу: одна и та же программа может входить
    # в две профильные группы, но внутри группы дублей быть не должно.
    dup = [k for k, v in Counter((x["program_id"], x["profile_group"])
                                 for x in offered).items() if v > 1]
    if dup:
        err(f"A: дублей (program_id, profile_group): {len(dup)}, например {dup[:3]}")

    ids_per_program = defaultdict(set)
    for x in offered:
        ids_per_program[x["program_id"]].add((x["program_name"], x["faculty"], x["napravlenie_code"]))
    clash = {k: v for k, v in ids_per_program.items() if len(v) > 1}
    if clash:
        err(f"A: один program_id у разных программ: {len(clash)}, например {list(clash)[:3]}")

    covered = {(x["vuz_id"], x["profile_group"]) for x in a}
    missing = {(v, g) for v in VUZY for g in GROUPS} - covered
    if missing:
        err(f"A: нет записи по парам вуз+группа: {sorted(missing)}")

    # --- Датасет B
    a_pids = {x["program_id"] for x in offered}
    b_pids = {x["program_id"] for x in b if x["program_id"]}
    if b_pids - a_pids:
        err(f"B: program_id, которых нет в A: {sorted(b_pids - a_pids)[:5]}")
    if a_pids - b_pids:
        err(f"B: программы из A пропущены в B: {len(a_pids - b_pids)}")

    b_cells = {(x["vuz_id"], x["profile_group"]) for x in b}
    if covered - b_cells:
        err(f"B: пары вуз+группа из A отсутствуют в B: {sorted(covered - b_cells)[:5]}")

    total_benefits = 0
    for x in b:
        if x["admission_year"] != 2026:
            err(f"B: admission_year != 2026 у {x.get('program_id')}")
        if x["status"] != "offered" and x["prinimaemye_olimpiady"]:
            err(f"B: непустой список олимпиад при статусе {x['status']} ({x.get('program_id')})")
        for ben in x["prinimaemye_olimpiady"]:
            total_benefits += 1
            oid = ben["olympiad_id"]
            if not OID_RE.match(oid):
                err(f"B: olympiad_id не по формуле раздела 4: {oid}")
            elif oid.startswith("p669") and oid not in known_ids:
                err(f"B: olympiad_id {oid} отсутствует в перечне №669")
            elif oid.startswith("vsosh") and oid not in vsosh_ok:
                err(f"B: неизвестный слаг предмета ВсОШ: {oid}")
            if ben["diploma_status"] not in STATUSES:
                err(f"B: недопустимый diploma_status {ben['diploma_status']}")
            if ben["benefit_type"] not in BENEFITS:
                err(f"B: недопустимый benefit_type {ben['benefit_type']}")
            if ben["min_level_required"] not in LEVELS:
                err(f"B: недопустимый min_level_required {ben['min_level_required']}")
            if "eligible_grades" not in ben:
                err(f"B: пропущено поле eligible_grades ({oid})")
            if "diploma_valid_years" in ben:
                err("B: поле diploma_valid_years не должно существовать")
            if not ben.get("source_url"):
                err(f"B: льгота без source_url ({oid})")
            if oid.startswith("vsosh-"):
                # Ч. 4 ст. 71 273-ФЗ: ВсОШ результатом ЕГЭ не подтверждается —
                # порог 75 по ч. 12 касается только олимпиад школьников.
                if ben.get("ege_confirm_subject") or ben.get("ege_confirm_min_score") is not None:
                    err(f"B: у ВсОШ не бывает подтверждения ЕГЭ ({x['program_id']} ← {oid})")
            elif not ben.get("is_demo") and ben.get("ege_confirm_min_score") is None:
                err(f"B: балл подтверждения не заполнен и не помечен is_demo ({oid})")
            if str(ben["source_url"]).lower().endswith(".pdf") and ben.get("source_page") is None:
                warn(f"B: PDF-источник без source_page ({oid})")
            subj = ben.get("ege_confirm_subject")
            if subj is not None and not set(subj.split(" или ")) <= EGE_SUBJECTS:
                err(f"B: предмет ЕГЭ не из списка ЕГЭ: «{subj[:60]}» ({x['program_id']} ← {oid})")

    # --- Эталонные факты
    have = defaultdict(set)
    for x in b:
        for ben in x["prinimaemye_olimpiady"]:
            have[(x["program_id"], ben["olympiad_id"])].add((ben["diploma_status"], ben["benefit_type"]))
    for pid, oid, want in GOLDEN:
        if pid not in a_pids:
            err(f"эталон: программы {pid} нет в A")
        elif have[(pid, oid)] != want:
            err(f"эталон: {pid} ← {oid}: ждали {sorted(want)}, в B {sorted(have[(pid, oid)])}")
    fields = defaultdict(list)
    for x in b:
        for ben in x["prinimaemye_olimpiady"]:
            fields[(x["program_id"], ben["olympiad_id"])].append(ben)
    for pid, oid, field, want in GOLDEN_CONDITIONS:
        got = [ben.get(field) for ben in fields[(pid, oid)]]
        if not got or any(g != want for g in got):
            err(f"эталон условий: {pid} ← {oid}: {field} ждали {want!r}, в B {got!r}")
    for pid, oid, status, want in GOLDEN_STATUS_GRADES:
        got = [ben["eligible_grades"] for ben in fields[(pid, oid)] if ben["diploma_status"] == status]
        if not got or any(g != want for g in got):
            err(f"эталон условий: {pid} ← {oid} ({status}): классы ждали {want}, в B {got}")
    # ВсОШ СПбГУ — не из перечня РСОШ: БВИ из документа по ВсОШ, 100 баллов —
    # из Правил (п. 7.5, приложение 7).
    for x in b:
        if x["vuz_id"] == "spbu":
            for ben in x["prinimaemye_olimpiady"]:
                src = "olymp_1" if ben["benefit_type"] == "БВИ" else "pravila_priema"
                if ben["olympiad_id"].startswith("vsosh-") and src not in ben["source_url"]:
                    err(f"СПбГУ: ВсОШ из перечня РСОШ ({x['program_id']} ← {ben['olympiad_id']})")

    # --- Сводка
    print(f"Датасет A: {len(a)} объектов ({len(offered)} offered, "
          f"{len(a) - len(offered)} not_offered/to_check), уникальных программ: {len(a_pids)}")
    print(f"Датасет B: {len(b)} программ, записей о льготах: {total_benefits}")
    by_vuz = Counter(x["vuz_id"] for x in offered)
    print("программ по вузам: " + ", ".join(f"{k}={v}" for k, v in sorted(by_vuz.items())))

    if warnings:
        print(f"\nПредупреждений: {len(warnings)}")
        for w in list(dict.fromkeys(warnings))[:8]:
            print(f"  ! {w}")
    if errors:
        print(f"\nОШИБОК: {len(errors)}")
        for e in list(dict.fromkeys(errors))[:20]:
            print(f"  x {e}")
        return 1
    print("\nВсе жёсткие проверки чек-листа пройдены.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
