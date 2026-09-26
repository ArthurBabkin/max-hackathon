"""Разбор перечней вузов в датасет B. Запуск:
python3 -m unittest discover -s datasets/parser -p 'test_*.py'."""
import unittest

import build_b as bb

URL = "https://innopolis.example/olymp.pdf"
HEADER = [["Полное наименование олимпиады", "Профиль олимпиады, соответствующий направлению", None,
           "Уровень олимпиады", "Общеобразовательный предмет"],
          [None, "Профиль олимпиады", "Общеобразовательные предметы", None, None]]


def page(n, *rows):
    return {"page": n, "tables": [list(rows)]}


def parsed(pages):
    return [(r["olympiad_name"], r["profile"], r["level"], r["ege_subject"], r["page"])
            for r in bb.innopolis_rows(pages, URL) if r["benefit"] == "БВИ"]


class InnopolisBenefitsTest(unittest.TestCase):
    """Правила п. 57–62: олимпиада из приложения 3 даёт и БВИ, и 100 баллов по
    предмету графы 5; графа 5 пустая — только БВИ. ВсОШ (приложение 2) — БВИ
    и особое преимущество: 100 баллов по предмету графы 3."""

    def test_hundred_along_with_bvi(self):
        rows = bb.innopolis_rows([page(5, *HEADER,
                                       ["«Формула Единства»/«Третье тысячелетие»", "математика", "математика", "II", "математика"],
                                       [None, "физика", "физика", "I", ""])], URL)
        self.assertEqual(sorted((r["profile"], r["benefit"]) for r in rows),
                         [("математика", "100_ballov"), ("математика", "БВИ"), ("физика", "БВИ")])

    def test_vsosh_hundred(self):
        rows = bb.innopolis_rows([page(4, ["1", "Математика", "Математика"])], URL)
        self.assertEqual(sorted(r["benefit"] for r in rows), ["100_ballov", "БВИ"])


class InnopolisTablesTest(unittest.TestCase):
    """Приложение 3 приказа Иннополиса: pdfplumber режет одну и ту же таблицу
    на 5, 7 или 11 колонок, смотря по странице. Строка олимпиады — там, где
    есть ячейка уровня; до неё название (если есть), профиль и предмет, после —
    предмет вступительного испытания."""

    def test_five_columns_name_carried_down(self):
        got = parsed([page(5, *HEADER,
                           ["«Формула Единства»/«Третье тысячелетие»", "математика", "математика", "II", "математика"],
                           [None, "физика", "физика", "III", "физика"])])
        self.assertEqual(got, [
            ("«Формула Единства»/«Третье тысячелетие»", "математика", "II", "математика", 5),
            ("«Формула Единства»/«Третье тысячелетие»", "физика", "III", "физика", 5),
        ])

    def test_level_in_arabic_digits(self):
        got = parsed([page(12, ["Московская олимпиада\nшкольников", "вероятность и\nстатистика", "математика", "II", "математика"],
                           [None, "информатика", "информатика", "1", "информатика"])])
        self.assertEqual([(p, lv) for _, p, lv, *_ in got], [("вероятность и статистика", "II"), ("информатика", "I")])

    def test_seven_columns_continue_olympiad_from_previous_page(self):
        got = parsed([
            page(7, ["Всероссийская междисциплинарная олимпиада школьников 8–11 классов «Национальная технологическая олимпиада»",
                     "автоматизация бизнес-процессов", "компьютерные и информационные науки", "II", "информатика"]),
            page(8, ["", "информационная безопасность", "компьютерные и информационные науки", None, None, "III", "информатика"],
                 [None, "инфохимия", "", "информатика и вычислительная", "", "III", "информатика"],
                 [None, None, None, "техника, химическая технология,", None, None, None]),
        ])
        nto = "Всероссийская междисциплинарная олимпиада школьников 8–11 классов «Национальная технологическая олимпиада»"
        self.assertEqual(got[1:], [
            (nto, "информационная безопасность", "III", "информатика", 8),
            (nto, "инфохимия", "III", "информатика", 8),
        ])

    def test_eleven_columns_innopolis_open(self):
        got = parsed([page(11,
                           ["Международная олимпиада Innopolis Open", "информатика", None, None, "информатика", None, None, "II", None, None, "информатика"],
                           [None, "искусственный интеллект", None, None, "информатика", None, None, "III", None, None, "информатика"],
                           [None, "математика", None, None, "математика", None, None, "II", None, None, "математика"],
                           [None, "", "физика", "", "", "физика", "", "", "II", "", "физика"])])
        self.assertEqual(got, [
            ("Международная олимпиада Innopolis Open", "информатика", "II", "информатика", 11),
            ("Международная олимпиада Innopolis Open", "искусственный интеллект", "III", "информатика", 11),
            ("Международная олимпиада Innopolis Open", "математика", "II", "математика", 11),
            ("Международная олимпиада Innopolis Open", "физика", "II", "физика", 11),
        ])

    def test_name_split_over_two_rows(self):
        got = parsed([page(16,
                           ["Олимпиада Курчатов", None, None, "математика", "математика", "II", "математика"],
                           ["", "Олимпиада школьников", "", "информатика", "информатика", "II", "информатика"],
                           [None, "«Гранит науки»", None, None, None, None, None])])
        self.assertEqual(got, [
            ("Олимпиада Курчатов", "математика", "II", "математика", 16),
            ("Олимпиада школьников «Гранит науки»", "информатика", "II", "информатика", 16),
        ])

    def test_only_appendix_3(self):
        row = ["«Формула Единства»/«Третье тысячелетие»", "математика", "математика", "II", "математика"]
        self.assertEqual([p for *_, p in parsed([page(n, row) for n in (3, 5, 16, 17)])], [5, 16])



def msu(*rows):
    return [(r["match"]["napravlenie"], r["profile"], r.get("olympiad_name"), r["level"], r["statuses"],
             r["benefit"], r["ege_subject"], r["grades"])
            for r in bb.msu_rows([{"page": 9, "tables": [list(rows)]}], URL)]


class MsuTablesTest(unittest.TestCase):
    """olymp_benefits.pdf МГУ: профиль, перечень, класс и предмет ЕГЭ —
    объединённые ячейки на несколько уровней или статусов. pdfplumber кладёт
    текст в первую строку, у продолжений ячейки пустые."""

    FACULTY = ["ГЕОЛОГИЧЕСКИЙ ФАКУЛЬТЕТ", "", "", "", "", "", "", "", ""]
    HEAD = ["Направление подготовки", "Профиль олимпиады", "Общеобразовательные предметы", "Перечень олимпиад",
            "Уровень олимпиады", "Класс", "Победитель / призер", "Предмет", "Льгота"]
    BVI_CELL = "Зачисление без вступительных испытаний"
    MAX_CELL = "Максимальное количество баллов по ЕГЭ"

    def test_continuation_rows_take_merged_cells_from_above(self):
        got = msu(self.FACULTY, self.HEAD,
                  ["Геология", "Математика", "Математика", "*", "I", "11", "Победитель, призер", "Математика", self.BVI_CELL],
                  ["", "", "", "", "II", "", "Победитель, призер", "", self.MAX_CELL],
                  ["", "Информатика", "Информатика", "Московская олимпиада школьников", "I", "11", "Победитель", "Информатика", self.BVI_CELL],
                  ["", "", "", "", "I", "", "Призер", "", self.MAX_CELL])
        self.assertEqual(got, [
            ("Геология", "Математика", None, "I", ["pobeditel", "prizyor"], "БВИ", "Математика", [11]),
            ("Геология", "Математика", None, "II", ["pobeditel", "prizyor"], "100_ballov", "Математика", [11]),
            ("Геология", "Информатика", "Московская олимпиада школьников", "I", ["pobeditel"], "БВИ", "Информатика", [11]),
            ("Геология", "Информатика", "Московская олимпиада школьников", "I", ["prizyor"], "100_ballov", "Информатика", [11]),
        ])

    def test_level_carried_from_previous_row(self):
        got = msu(self.FACULTY,
                  ["Фундаментальная и прикладная химия", "Химия", "Химия", "*", "I", "11", "Победитель", "Химия", self.MAX_CELL],
                  ["", "", "", "", "", "11", "Призер", "", self.MAX_CELL],
                  ["", "", "", "", "II", "11", "Победитель", "", self.MAX_CELL],
                  ["", "", "", "", "", "11", "Призер", "", self.MAX_CELL])
        self.assertEqual([(lv, st) for _, _, _, lv, st, *_ in got],
                         [("I", ["pobeditel"]), ("I", ["prizyor"]), ("II", ["pobeditel"]), ("II", ["prizyor"])])

    def test_no_carry_into_next_direction(self):
        got = msu(self.FACULTY,
                  ["Геология", "Математика", "Математика", "*", "I", "11", "Победитель, призер", "Математика", self.BVI_CELL],
                  ["Геофизика", "", "", "", "II", "", "Победитель, призер", "", self.MAX_CELL])
        self.assertEqual([(n, p) for n, p, *_ in got], [("Геология", "Математика"), ("Геофизика", "")])

    def test_merged_benefit_cell_carried_down(self):
        # ФиПФ, стр. 5: «Зачисление без ВИ» — одна ячейка на победителя I и II уровня.
        got = msu(self.FACULTY,
                  ["Физика", "Физика", "Физика", "*", "I", "11 класс", "Победитель", "Физика", self.BVI_CELL],
                  ["", "", "", "", "II", "11 класс", "Победитель", "", ""])
        self.assertEqual([(lv, b) for _, _, _, lv, _, b, *_ in got], [("I", "БВИ"), ("II", "БВИ")])

    def test_names_split_by_semicolon(self):
        # Экономфак, стр. 19–21: олимпиады перечислены через «;».
        got = msu(self.FACULTY, ["Экономика", "Математика", "Математика",
                                 "Московская олимпиада школьников; Олимпиада школьников «Физтех»", "I, II", "11",
                                 "Победитель, призер", "Математика", self.MAX_CELL])
        self.assertEqual([n for _, _, n, *_ in got], ["Московская олимпиада школьников", "Олимпиада школьников «Физтех»"])

    def test_star_row_levels_exact(self):
        # «I, III» нарочно пропускает II: уровни — множество, а не «не ниже».
        rows = bb.msu_rows([{"page": 9, "tables": [[self.FACULTY, ["Экономика", "Экономика", "Экономика", "*",
                                                                   "I, III", "11", "Победитель", "Математика",
                                                                   self.MAX_CELL]]]}], URL)
        self.assertEqual([r["levels"] for r in rows], [["I", "III"]])


SCHOOLS = ["ФРКТ", "ЛФИ", "ФАКТ", "ПИШ ФАЛТ", "ФЭФМ", "ФПМИ", "ФБМФ/ ВШБИ", "КНТ", "ФБВТ", "ВШПИ", "ВШ М"]


def prog(pid, name, faculty, code="01.03.02", group="ИТ"):
    return {"program_id": pid, "program_name": name, "faculty": faculty,
            "napravlenie_code": code, "napravlenie_name": name, "profile_group": group}


MIPT_PROGRAMS = [
    prog("pmi", "Прикладная математика и информатика", "ФПМИ"),
    prog("ekn", "Естественные и компьютерные науки", "ФПМИ", "09.03.01"),
    prog("mmtu", "Математическое моделирование и теория управления", "ФПМИ", "09.03.01"),
    prog("lfi", "Общая и прикладная физика", "ЛФИ", "03.03.01", "Физика"),
    prog("fbmf", "Биофизика и биоинформатика (ФБМФ)", "ФБМФ, ВШБИ", "03.03.01", "Физика"),
    prog("vshbi", "Биоинженерия и биотехнология (ВШБИ)", "ФБМФ, ВШБИ", "03.03.01", "Физика"),
    prog("frkt-rt", "Радиотехника и компьютерные технологии", "ФРКТ", "03.03.01", "Физика"),
    prog("frkt-kt", "Компьютерные технологии и вычислительная техника", "ФРКТ", "09.03.01"),
    prog("falt-av", "Авиационные технологии", "ПИШ ФАЛТ", "03.03.01", "Физика"),
    prog("falt-bas", "Беспилотные авиационные системы", "ПИШ ФАЛТ", "03.03.01", "Физика"),
    prog("vshm", "Фундаментальная математика", "ВШМ", "01.03.01"),
    prog("matem", "Математика", "ФПМИ"),
    prog("sppm", "Системное программирование и прикладная математика", "ФПМИ", "09.03.01"),
]


def html_table(*rows):
    tr = "".join("<tr>" + "".join(f"<td>{c}</td>" for c in r) + "</tr>" for r in rows)
    return f"<table>{tr}</table>"


HEAD_RULES = ["Образовательные программы", "Конкурсная группа", "Вступительные испытания (по приоритету)"]
# Правила приёма МФТИ: программа -> конкурсная группа и ВИ, как на pk.mipt.ru/bachelor/2026_rules.
MIPT_RULES = html_table(
    ["Физтех-школа прикладной математики и информатики (ФПМИ)"], HEAD_RULES,
    ["Направление 01.03.02 Прикладная математика и информатика"],
    ["Прикладная математика и информатика", "Прикладная математика и информатика", "Математика, информатика, русский язык"],
    ["Математика", "Прикладная математика и информатика", "Математика, информатика, русский язык"],
    ["Направление 09.03.01 Информатика и вычислительная техника"],
    ["Системное программирование и прикладная математика", "Системное программирование и прикладная математика",
     "Математика, информатика, физика, русский язык"],
    ["Естественные и компьютерные науки", "Естественные и компьютерные науки", "Математика, информатика/физика, русский язык"],
    ["Математическое моделирование и теория управления", "Математическое моделирование и теория управления",
     "Математика, информатика/физика, русский язык"],
    ["Физтех-школа физики и исследований им. Ландау (ЛФИ)"], HEAD_RULES,
    ["Направление 03.03.01 Прикладные математика и физика"],
    ["Общая и прикладная физика", "Общая и прикладная физика", "Математика, физика, русский язык"],
    ["Биофизика и биоинформатика (ФБМФ)", "Биофизика и биоинформатика", "Математика, физика/химия/информатика, русский язык"],
    ["Биоинженерия и биотехнология (ВШБИ)", "Биофизика и биоинформатика", "Математика, физика/химия/информатика, русский язык"],
    ["Радиотехника и компьютерные технологии", "Радиотехника и компьютерные технологии", "Математика, физика, русский язык"],
    ["Авиационные технологии", "Авиационные технологии и автономные транспортные системы",
     "Математика, физика/информатика, русский язык"],
    ["Беспилотные авиационные системы", "Авиационные технологии и автономные транспортные системы",
     "Математика, физика/информатика, русский язык"],
    ["Направление 09.03.01 Информатика и вычислительная техника"],
    ["Компьютерные технологии и вычислительная техника", "Компьютерные технологии и вычислительная техника",
     "Математика, информатика/физика, русский язык"],
    ["Направление 01.03.01 Математика"],
    ["Фундаментальная математика", "Фундаментальная математика", "Математика, физика/информатика, русский язык"],
    ["Категория лиц", "Перечень подтверждающих документов"],
)


class MiptPlanMixin:
    """Подменяет правила приёма МФТИ из снапшота на MIPT_RULES."""

    def setUp(self):
        self._plan, bb._MIPT_PLAN = bb._MIPT_PLAN, bb.mipt_plan(MIPT_RULES)

    def tearDown(self):
        bb._MIPT_PLAN = self._plan


def mipt_targets(rows):
    """(олимпиада, льгота, статусы, предмет ЕГЭ, порог) -> программы."""
    return {(r.get("olympiad_name") or r["profile"], r["benefit"], tuple(r["statuses"]),
             r["ege_subject"], r["ege_score"]):
            sorted(p["program_id"] for p in bb.link("mipt", r, MIPT_PROGRAMS))
            for r in rows}


class MiptMatrixTest(MiptPlanMixin, unittest.TestCase):
    """Матрица МФТИ «олимпиада × физтех-школа»: БВИ дают не вузу, а школе,
    а внутри школы — иногда только названным конкурсным группам."""

    HEAD = ["№ в перечне", "Полное наименование олимпиады", "Профиль олимпиады", "Уровень",
            "Особое право получения 100 баллов"] + SCHOOLS

    def rows(self, *cells, hundred="100 баллов по биологии при наличии результата ЕГЭ или ВИ по биологии 75 баллов и выше",
             benefit="БВИ"):
        # № не 54: у «Физтеха» классы победителя и призёра разные (MiptGradesTest),
        # и строки делятся по статусам — здесь проверяется только привязка.
        html = html_table(self.HEAD, ["56", "Олимпиада школьников \"Физтех\"", "биология", "2", hundred]
                          + (list(cells) or [""] * 11))
        return [r for r in bb.mipt_rows(html, URL) if r["benefit"] == benefit]

    def hundred(self, cell):
        return mipt_targets(self.rows(hundred=cell, benefit="100_ballov"))

    def test_rules_plan(self):
        plan = bb.mipt_plan(MIPT_RULES)
        self.assertEqual(plan[("09.03.01", bb._squash("Естественные и компьютерные науки"))],
                         {"group": "Естественные и компьютерные науки",
                          "exams": {"Математика", "Информатика", "Физика", "Русский язык"}})
        self.assertEqual(plan[("01.03.02", bb._squash("Математика"))]["group"], "Прикладная математика и информатика")

    def test_named_group_is_matched_exactly_not_as_substring(self):
        # «…и прикладная математика» — не программа «Математика» (она в группе ПМИ)
        cells = [""] * 11
        cells[5] = '"Системное программирование и прикладная математика" Победителям и призерам'
        self.assertEqual(list(mipt_targets(self.rows(*cells)).values()), [["sppm"]])

    def test_group_covers_all_its_programs(self):
        cells = [""] * 11
        cells[5] = '"Прикладная математика и информатика" Победителям'
        self.assertEqual(list(mipt_targets(self.rows(*cells)).values()), [["matem", "pmi"]])

    def test_hundred_points_only_where_subject_is_an_exam(self):
        # 100 баллов по химии — ни ПМИ, ни ЛФИ: химии нет среди их ВИ
        got = self.hundred("100 баллов по химии при наличии результата ЕГЭ или ВИ по химии 75 баллов и выше")
        self.assertEqual(got, {("Олимпиада школьников \"Физтех\"", "100_ballov", ("pobeditel", "prizyor"), "Химия", 75):
                               ["fbmf", "vshbi"]})

    def test_hundred_points_each_subject_of_the_cell(self):
        got = self.hundred("100 баллов по физике при наличии результата ЕГЭ или ВИ по физике 75 баллов и выше; "
                           "100 баллов по информатике и ИКТ при наличии результата ЕГЭ или ВИ по информатике и ИКТ 75 баллов и выше")
        self.assertEqual({k[3]: v for k, v in got.items()}, {
            "Физика": ["ekn", "falt-av", "falt-bas", "fbmf", "frkt-kt", "frkt-rt", "lfi", "mmtu", "sppm", "vshbi", "vshm"],
            "Информатика": ["ekn", "falt-av", "falt-bas", "fbmf", "frkt-kt", "matem", "mmtu", "pmi", "sppm", "vshbi", "vshm"],
        })

    def test_hundred_points_russian_goes_to_every_program(self):
        got = self.hundred("100 баллов по русскому языку при наличии результата ЕГЭ или ВИ по русскому языку 75 баллов и выше")
        self.assertEqual({k[3]: len(v) for k, v in got.items()}, {"Русский язык": len(MIPT_PROGRAMS)})

    def test_program_exams_from_rules(self):
        pmi = next(p for p in MIPT_PROGRAMS if p["program_id"] == "pmi")
        self.assertEqual(bb.program_exams("mipt", pmi), {"Математика", "Информатика", "Русский язык"})

    def test_hundred_points_subject_no_program_examines(self):
        got = self.hundred("100 баллов по иностранному языку при наличии результата ЕГЭ или ВИ по иностранному языку 75 баллов и выше")
        self.assertEqual(list(got.values()), [[]])

    def test_school_is_read_from_its_own_column(self):
        cells = [""] * 11
        cells[6] = "Все конкурсные группы ФБМФ, ВШБИ Победителям при наличии результата ЕГЭ или ВИ по биологии 75 баллов и выше"
        got = mipt_targets(self.rows(*cells))
        self.assertEqual(list(got.values()), [["fbmf", "vshbi"]])

    def test_only_named_competition_groups(self):
        cells = [""] * 11
        cells[5] = ('"Естественные и компьютерные науки", "Математическое моделирование и теория управления" '
                    "Победителям при наличии результата ЕГЭ или ВИ по математике 85 баллов и выше")
        got = mipt_targets(self.rows(*cells))
        self.assertEqual(got, {("Олимпиада школьников \"Физтех\"", "БВИ", ("pobeditel",), "Математика", 85): ["ekn", "mmtu"]})

    def test_clauses_with_own_conditions(self):
        cells = [""] * 11
        cells[0] = ('"Компьютерные технологии и вычислительная техника" Победителям при наличии результата ЕГЭ или ВИ '
                    'по информатике 75 баллов и выше; "Радиотехника и компьютерные технологии" Победителям и призерам '
                    "при наличии результата ЕГЭ или ВИ по физике 80 баллов и выше")
        got = mipt_targets(self.rows(*cells))
        self.assertEqual(got, {
            ("Олимпиада школьников \"Физтех\"", "БВИ", ("pobeditel",), "Информатика", 75): ["frkt-kt"],
            ("Олимпиада школьников \"Физтех\"", "БВИ", ("pobeditel", "prizyor"), "Физика", 80): ["frkt-rt"],
        })

    def test_school_named_in_cell_narrows_joint_column(self):
        cells = [""] * 11
        cells[6] = "Все конкурсные группы ФБМФ Победителям и призерам"
        self.assertEqual(list(mipt_targets(self.rows(*cells)).values()), [["fbmf"]])

    def test_group_name_covers_two_programs(self):
        cells = [""] * 11
        cells[3] = '"Авиационные технологии и беспилотные авиационные системы" Победителям и призерам'
        self.assertEqual(list(mipt_targets(self.rows(*cells)).values()), [["falt-av", "falt-bas"]])

    def test_last_school_column(self):
        cells = [""] * 11
        cells[10] = "Все конкурсные группы ВШМ Победителям и призерам"
        self.assertEqual(list(mipt_targets(self.rows(*cells)).values()), [["vshm"]])

    def test_vsosh_table(self):
        html = html_table(["Общеобразовательный предмет"] + [s.replace("ВШ М", "ВШМ") for s in SCHOOLS],
                          ["Биология", "", "", "", "", "",
                           '"Естественные и компьютерные науки", "Математическое моделирование и теория управления" Победителям и призерам',
                           "Все конкурсные группы ФБМФ, ВШБИ Победителям и призерам", "", "", "", ""])
        got = [sorted(p["program_id"] for p in bb.link("mipt", r, MIPT_PROGRAMS))
               for r in bb.mipt_vsosh_rows(html, URL)]
        self.assertEqual(sorted(got), [["ekn", "mmtu"], ["fbmf", "vshbi"]])


def words(*lines):
    """(top, левая колонка, правая колонка) -> слова pdfplumber."""
    out = []
    for top, left, right in lines:
        out += [{"x0": 41 + i, "top": top, "text": w} for i, w in enumerate(left.split())]
        out += [{"x0": 432 + i, "top": top, "text": w} for i, w in enumerate(right.split())]
    return out


class ItmoVsoshTest(unittest.TestCase):
    """Соотнесение ВсОШ ИТМО: предмет — объединённая ячейка на блок
    направлений, а не текст в строке с кодом."""

    def test_subject_covers_whole_block_and_continues_on_next_page(self):
        pages = [
            (1, words((240, "Направление подготовки", "Предмет олимпиады"),
                      (260, "03.03.02 Физика", "Физика, Астрономия,"),
                      (272, "12.03.03 Фотоника и оптоинформатика", "Технология"),
                      (284, "16.03.01 Техническая физика", ""),
                      (710, "27.03.05 Инноватика", "Химия, Биология")), [232, 249, 700]),
            (2, words((160, "38.03.05 Бизнес-информатика", ""),
                      (210, "01.03.02 Прикладная математика и информатика", "Информатика"),
                      (222, "02.03.03 Математическое обеспечение", "(Искусственный интеллект)")), [154, 204, 266]),
        ]
        got = {(code, r["profile"]) for r in bb.itmo_vsosh_rows(pages, URL) for code in r["match"]["codes"]}
        self.assertEqual(got, {
            ("03.03.02", "физика"), ("12.03.03", "физика"), ("16.03.01", "физика"),
            ("03.03.02", "астрономия"), ("12.03.03", "астрономия"), ("16.03.01", "астрономия"),
            ("27.03.05", "химия"), ("38.03.05", "химия"), ("27.03.05", "биология"), ("38.03.05", "биология"),
            ("01.03.02", "информатика"), ("02.03.03", "информатика"),
        })


class ItmoListsTest(unittest.TestCase):
    """Перечни ИТМО: направление и название — объединённые ячейки на блок
    строк, блок направлений бывает разорван страницей."""

    @staticmethod
    def page(n, rows, dir_lines, name_lines=None):
        fine, top = [], 100
        for cells in rows:
            fine.append((top, top + 20, cells))
            top += 20
        level = [100 + 20 * i for i in range(len(rows) + 1)]
        lines = {0: dir_lines, 1: name_lines if name_lines is not None else level,
                 2: level, 3: level, 4: level, 5: []}
        return n, fine, lines

    def test_blocks_across_pages(self):
        got = bb.itmo_merged_rows([
            self.page(1, [["01.03.02 ПМИ", "Все из перечня олимпиад", "Информатика", "Информатика", "1, 2 или 3", "Победитель"],
                          ["", "школьников", "Математика", "Математика", "1", "или призер"],
                          ["02.03.03 МОАИС,", "Московская олимпиада школьников", "Информатика", "Информатика", "1", "Победитель"]],
                      [100, 140, 160], [100, 140, 160]),
            self.page(2, [["09.03.04 Программная", "Олимпиада «Высшая проба»", "Математика", "Математика", "1", "Победитель"]],
                      [100, 120]),
            self.page(3, [["", "Турнир городов", "Математика", "Математика", "1", "Победитель"],
                          ["10.03.01 ИБ", "Олимпиада «Физтех»", "Физика", "Физика", "2", "Победитель"]],
                      [100, 120, 140]),
        ])
        self.assertEqual([(c[0], c[1], c[5]) for _, c in got], [
            ("01.03.02 ПМИ", "Все из перечня олимпиад школьников", "Победитель"),
            ("01.03.02 ПМИ", "Все из перечня олимпиад школьников", "или призер"),
            ("02.03.03 МОАИС, 09.03.04 Программная", "Московская олимпиада школьников", "Победитель"),
            ("02.03.03 МОАИС, 09.03.04 Программная", "Олимпиада «Высшая проба»", "Победитель"),
            ("02.03.03 МОАИС, 09.03.04 Программная", "Турнир городов", "Победитель"),
            ("10.03.01 ИБ", "Олимпиада «Физтех»", "Победитель"),
        ])

    def test_name_continues_on_next_page(self):
        got = bb.itmo_merged_rows([
            self.page(1, [["02.03.03 МОАИС,", "Все из перечня олимпиад школьников", "Информатика", "Информатика", "1", "Победитель"]],
                      [100, 120], [100, 120]),
            self.page(2, [["", "", "Виртуальные миры", "Информатика", "3", "Победитель"]], [100, 120], [100, 120]),
        ])
        self.assertEqual([c[1] for _, c in got], ["Все из перечня олимпиад школьников"] * 2)

    def test_only_on_direction(self):
        rows = bb.itmo_rows([(2, ["09.03.01 ИВТ, 10.03.01 ИБ", "Все из перечня олимпиад школьников",
                                  "Информационная безопасность (только на направление 10.03.01)",
                                  "Информатика", "1, 2 или 3", "Победитель или призер"])], URL, "100_ballov")
        self.assertEqual([(r["match"]["codes"], r["profile"]) for r in rows],
                         [(["10.03.01"], "Информационная безопасность")])


    def test_only_on_programs(self):
        progs = [prog("rob", "Робототехника и искусственный интеллект", "", "12.03.01"),
                 prog("pribor", "Приборостроение", "", "12.03.01"),
                 prog("tzi", "Технологии защиты информации", "", "10.03.01")]
        rows = bb.itmo_rows([(6, ["10.03.01 ИБ, 12.03.01 Приборостроение", "Все из перечня олимпиад школьников", p,
                                  "Информатика", "2 или 3", "Победитель или призер"]) for p in (
            "Автономные транспортные системы (учитывается только на программе Робототехника и ИИ)",
            "Летающая робототехника (учитывается на программах «Робототехника и ИИ» и «Технологии защиты информации» )")],
            URL, "БВИ")
        self.assertEqual([(r["profile"], sorted(p["program_id"] for p in bb.link("itmo", r, progs))) for r in rows],
                         [("Автономные транспортные системы", ["rob"]), ("Летающая робототехника", ["rob", "tzi"])])

    def test_level_cell_is_exact_set(self):
        # «2 или 3» — только II и III: олимпиада I уровня по этой строке БВИ не получает.
        self.assertEqual(bb._level_set("2 или 3"), ["II", "III"])
        self.assertEqual(bb._level_set("3"), ["III"])
        self.assertEqual(bb._level_set("1,2 или 3"), ["I", "II", "III"])
        self.assertEqual(bb._level_set("ОШ, ительн 2026 г"), [])

    def test_rows_carry_levels_and_grades(self):
        rows = bb.itmo_rows([(3, ["09.03.01 ИВТ", "Все из перечня олимпиад школьников", "Математика",
                                  "Математика", "2 или 3", "Победитель или призер"])], URL, "БВИ", grades=[10, 11])
        self.assertEqual([(r["levels"], r["grades"]) for r in rows], [(["II", "III"], [10, 11])])

    def test_grades_from_title(self):
        # Заголовки приложений 5 и 6: «…полученных в 10-м или 11-м классе…», «…в 10-м и 11-м класс…».
        self.assertEqual(bb.itmo_grades([{"page": 1, "text": "Учет дипломов … полученных в\n10-м или 11-м классе и"}]),
                         [10, 11])
        self.assertEqual(bb.itmo_grades([{"page": 1, "text": "баллов в отношении …, полученных в 10-м и\n11-м класс, не"}]),
                         [10, 11])

    def test_hundred_only_for_diplomas_without_bvi(self):
        # Приложение 6: 100 баллов — по дипломам, «не дающие право поступления без
        # вступительных испытаний»: победителю с БВИ 100 баллов не пишутся, призёру — да.
        recs = [{"olympiad_id": o, "diploma_status": s, "benefit_type": k} for o, s, k in (
            ("p1", "pobeditel", "БВИ"), ("p1", "pobeditel", "100_ballov"), ("p1", "prizyor", "100_ballov"),
            ("p2", "pobeditel", "100_ballov"))]
        got = [(r["olympiad_id"], r["diploma_status"], r["benefit_type"]) for r in bb.drop_hundred_under_bvi(recs)]
        self.assertEqual(got, [("p1", "pobeditel", "БВИ"), ("p1", "prizyor", "100_ballov"),
                               ("p2", "pobeditel", "100_ballov")])


class OlympiadNameTest(unittest.TestCase):
    """Название из документа вуза -> номер перечня. Общие слова («олимпиада
    школьников») не делают олимпиады одной: иначе Кондратьевская и «Газпром»
    становились Московской (ВШЭ)."""

    def test_generic_words_do_not_match(self):
        self.assertIsNone(bb.match_number_for_profile(
            "Всероссийская экономическая олимпиада школьников имени Н.Д. Кондратьева", "экономика"))
        self.assertIsNone(bb.match_number_for_profile("Отраслевая олимпиада школьников «Газпром»", "математика"))
        self.assertIsNone(bb.match_number_for_profile("Олимпиада школьников «Будущее Сибири»", "математика"))

    def test_typographic_quotes(self):
        self.assertEqual(bb.match_number_for_profile("Олимпиада “Физтех”", "физика"), 54)

    def test_real_names_still_match(self):
        self.assertEqual(bb.match_number_for_profile("Московская олимпиада школьников", "экономика"), 37)
        self.assertEqual(bb.match_number_for_profile("Олимпиада школьников «Ломоносов»", "математика"), 50)
        self.assertEqual(bb.match_number("Олимпиада школьников «Физтех»"), 54)
        # ОММО в перечне — «Объединенная межвузовская олимпиада школьников» (№41).
        self.assertEqual(bb.match_number_for_profile("Объединённая межвузовская математическая олимпиада школьников",
                                                     "математика"), 41)

    def test_other_olympiad_with_profile_is_not_substituted(self):
        # Иннополис: у «Росатома» (№70) нет инженерного дела — это не «Газпром» (№69).
        self.assertIsNone(bb.match_number_for_profile("Отраслевая физико- математическая олимпиада школьников «Росатом»",
                                                      "инженерное дело"))


class ExpandProfileTest(unittest.TestCase):
    def test_part_of_composite_nto_profile(self):
        got = [oid for oid, *_ in bb.expand_profile("Аэрокосмические системы", "II")]
        self.assertEqual(len(got), 1)
        self.assertTrue(got[0].startswith("p669-5-bespilotnyy-transport-aerokosmicheskie-sistemy"))

    def test_hyphen_split_by_line_break(self):
        self.assertEqual([o for o, *_ in bb.expand_profile("Автоматизация бизнес- процессов", "II")],
                         ["p669-5-avtomatizaciya-biznes-processov"])

    def test_composite_profile_without_tail(self):
        got = [o for o, *_ in bb.expand_profile("Виртуальные миры: разработка компьютерных игр, технологии "
                                                "виртуальной реальности, технологии дополненной реальности", "III")]
        self.assertEqual(len(got), 1)
        self.assertTrue(got[0].startswith("p669-5-virtualnye-miry"))

    def test_profile_missing_from_perechen(self):
        self.assertEqual(bb.expand_profile("Нанотехнологии", "I"), [])


KFU_PLAN = [{"page": 1, "tables": [[
    ["Институт вычислительной математики и информационных технологий", None, None, None, None, None, None, None, None, None],
    ["01.03.02", "Прикладная математика и информатика", "Прикладная математика и информатика", "бакалавриат", "", "", "", "", "",
     "1) Математика\n2) Информатика и ИКТ / Физика*\n3) Русский язык"],
    ["09.03.04", "Программная инженерия", "Программная инженерия", "бакалавриат", "", "", "", "", "",
     "1) Математика\n2) Информатика и ИКТ\n3) Русский язык"],
    ["Институт информационных технологий и интеллектуальных систем", None, None, None, None, None, None, None, None, None],
    ["09.03.03", "Прикладная информатика", "Прикладная информатика", "бакалавриат", "", "", "", "", "",
     "1) Математика\n2) Информатика и ИКТ\n3) Русский язык"],
    ["Институт физики", None, None, None, None, None, None, None, None, None],
    ["03.03.02", "Физика", "Физика", "бакалавриат", "", "", "", "", "",
     "1) Физика\n2) Математика\n3) Русский язык"],
    ["Институт психологии и образования", None, None, None, None, None, None, None, None, None],
    ["44.03.05", "Педагогическое образование (с двумя профилями подготовки)", "Биология и английский язык",
     "бакалавриат", "", "", "", "", "", "1) Биология\n2) Химия\n3) Русский язык"],
    ["44.03.05", "Педагогическое образование (с двумя профилями подготовки)", "Математика и информатика",
     "бакалавриат", "", "", "", "", "", "1) Математика\n2) Информатика и ИКТ\n3) Русский язык"],
]]}]

KFU_PROGRAMS = [
    prog("pmi", "Прикладная математика и информатика", "", "01.03.02"),
    prog("pi", "Программная инженерия", "", "09.03.04"),
    prog("itis", "Прикладная информатика", "", "09.03.03"),
    prog("phys", "Физика", "", "03.03.02", "Физика"),
    prog("ped-bio", "Биология и английский язык", "", "44.03.05", "Биомед"),
    prog("ped-math", "Математика и информатика", "", "44.03.05"),
]


class KfuTest(unittest.TestCase):
    """КФУ: «на все направления, где ВИ соответствуют графе 6 … и являются
    первыми в Плане приема (Приложение 1)», «кроме …», профиль в скобках."""

    def setUp(self):
        bb._KFU_PLAN = bb.kfu_plan(KFU_PLAN)

    def tearDown(self):
        bb._KFU_PLAN = None

    def targets(self, targets, first_vi):
        m = bb.kfu_match(targets, first_vi)
        return sorted(p["program_id"] for p in bb.link("kfu", {"match": m}, KFU_PROGRAMS))

    ALL = ("На все направления, где вступительные испытания соответствуют графе 6, профилю олимпиады "
           "(графа 3) и являются первыми в Плане приема (Приложение 1)")

    def test_all_directions_only_where_subject_is_first_exam(self):
        self.assertEqual(self.targets(self.ALL, "физика"), ["phys"])
        self.assertEqual(self.targets(self.ALL, "математика"), ["itis", "ped-math", "pi", "pmi"])

    def test_except_direction(self):
        self.assertEqual(self.targets(self.ALL + ", кроме 09.03.04 «Программная инженерия»", "математика"),
                         ["itis", "ped-math", "pmi"])

    def test_except_institute(self):
        self.assertEqual(self.targets(self.ALL + ", кроме Института информационных технологий и "
                                      "интеллектуальных систем", "математика"), ["ped-math", "pi", "pmi"])

    def test_profile_in_brackets(self):
        got = self.targets("03.03.02 Физика 44.03.05 Педагогическое образование (с двумя профилями подготовки) "
                           "(профиль: Биология и английский язык; Биология и химия)", "физика")
        self.assertEqual(got, ["ped-bio", "phys"])

    def test_hundred_points_where_subject_is_exam_but_not_first(self):
        m = {**bb.kfu_match(self.ALL, "информатика и ИКТ"), "hundred": True}
        self.assertEqual(sorted(p["program_id"] for p in bb.link("kfu", {"match": m}, KFU_PROGRAMS)),
                         ["itis", "ped-math", "pi", "pmi"])
        m = {**bb.kfu_match(self.ALL, "физика"), "hundred": True}
        self.assertEqual([p["program_id"] for p in bb.link("kfu", {"match": m}, KFU_PROGRAMS)], ["pmi"])

    def test_listed_code_without_profile_needs_subject_among_exams(self):
        self.assertEqual(self.targets("44.03.05 Педагогическое образование (с двумя профилями подготовки)", "биология"),
                         ["ped-bio"])


HSE_PROGRAMS = [
    prog("hse-math", "Математика", "НИУ ВШЭ — Москва", "01.03.01"),
    prog("hse-ib", "Информацион- ная безопасность", "НИУ ВШЭ — Москва", "10.03.01"),
    prog("nn-psy", "Психология в бизнесе", "НИУ ВШЭ — Нижний Новгород", "37.03.01", "Экономика"),
    prog("nn-cs", "Компьютерные науки и технологии", "НИУ ВШЭ — Нижний Новгород", "09.03.04"),
    prog("nn-cs-bi", "Компьютерные науки и технологии", "НИУ ВШЭ — Нижний Новгород", "38.03.05", "Экономика"),
]


class HseTest(unittest.TestCase):
    """Приложения ВШЭ: ОП — объединённая ячейка на блок строк, её текст
    напечатан посередине блока, иногда за краем страницы."""

    def test_program_of_rows_by_geometry(self):
        words = [{"top": -405, "x0": 98, "text": "Математика"},
                 {"top": 830, "x0": 97, "text": "Совместный"}, {"top": 838, "x0": 97, "text": "бакалавриат"}]
        self.assertEqual(bb.hse_programs(85, 539, [186, 186.5], words, [100, 150, 300, 520]),
                         ["Математика", "Математика", "Совместный бакалавриат", "Совместный бакалавриат"])

    def rows(self, head, program, *cells):
        return bb.hse_rows([(1, [([head], ("", "")), *[(c, (program, c[0])) for c in cells]])], URL, "Москва")

    ROW = ["Олимпиада «Высшая проба»", "математика", "математика", "математика", "75 и более",
           "право на 100 баллов", "математика", "победителям и призерам", "10, 11 класс"]

    def test_named_program_absent_from_a_gets_nothing(self):
        r = self.rows("Направление подготовки 01.03.01 Математика", "Совместный бакалавриат ВШЭ и ЦПМ", self.ROW)
        self.assertEqual(r[0]["benefit"], "100_ballov")
        self.assertEqual(bb.link("hse", r[0], HSE_PROGRAMS), [])

    def test_single_program_direction(self):
        r = self.rows("Направление подготовки 01.03.01 Математика", "", self.ROW)
        self.assertEqual([p["program_id"] for p in bb.link("hse", r[0], HSE_PROGRAMS)], ["hse-math"])

    def test_broken_words_in_names(self):
        r = self.rows("Направление подготовки 10.03.01 Информационная безопасность", "Информационная безопасность", self.ROW)
        self.assertEqual([p["program_id"] for p in bb.link("hse", r[0], HSE_PROGRAMS)], ["hse-ib"])

    def test_several_directions_in_header(self):
        r = bb.hse_rows([(1, [(["Направления подготовки 01.03.02 Прикладная математика и информатика, "
                                 "09.03.04 Программная инженерия, 38.03.05 Бизнес-информатика"], ("", "")),
                                (self.ROW, ("Компьютерные науки и технологии", self.ROW[0]))])], URL, "Нижний Новгород")
        self.assertEqual(sorted(p["program_id"] for p in bb.link("hse", r[0], HSE_PROGRAMS)), ["nn-cs", "nn-cs-bi"])

    def test_row_without_program_in_block_with_named_programs_dropped(self):
        # Стр. 110: ОП называется, только если в направлении их несколько. Строки
        # блока, где геометрия имени не нашла, — не «все ОП направления»: в A
        # у 40.03.01 одна ОП, и чужие льготы доставались ей.
        law = "Направление подготовки 40.03.01 Юриспруденция"
        r = bb.hse_rows([(1, [([law], ("", "")), (self.ROW, ("", self.ROW[0])),
                              (self.ROW, ("Юриспруденция", self.ROW[0]))])], URL, "Москва")
        self.assertEqual([x["match"]["program"] for x in r], ["Юриспруденция"])

    def test_olympiad_name_from_merged_cell_not_carried(self):
        row = ["", "информатика", "информатика", "математика", "75 и более", "Право на прием БВИ",
               "математика", "Победителям", "11 класс"]
        r = bb.hse_rows([(1, [(["Направление подготовки 01.03.01 Математика"], ("", "")),
                              (self.ROW, ("", self.ROW[0])),
                              (row, ("", "Всероссийская олимпиада школьников «Высшая проба»"))])], URL, "Москва")
        self.assertEqual([x["olympiad_name"] for x in r],
                         ["Олимпиада «Высшая проба»", "Всероссийская олимпиада школьников «Высшая проба»"])

    def test_cell_name_wins_over_geometry_unless_fragment(self):
        row2 = ["«Высшая проба»"] + self.ROW[1:]
        r = bb.hse_rows([(1, [(["Направление подготовки 01.03.01 Математика"], ("", "")),
                              (self.ROW, ("", "Олимпиада школьников «Ломоносов»")),
                              (row2, ("", "Всероссийская олимпиада школьников «Высшая проба»"))])], URL, "Москва")
        self.assertEqual([x["olympiad_name"] for x in r],
                         ["Олимпиада «Высшая проба»", "Всероссийская олимпиада школьников «Высшая проба»"])

    def test_typo_in_direction_code(self):
        r = bb.hse_rows([(1, [(["Направления подготовки 37.04.01 Психология"], ("", "")),
                                (self.ROW, ("Психология в бизнесе", self.ROW[0]))])], URL, "Нижний Новгород")
        self.assertEqual([p["program_id"] for p in bb.link("hse", r[0], HSE_PROGRAMS)], ["nn-psy"])

    TAIL = ["Количество баллов ЕГЭ или общеобразовательного вступительного испытания (не менее 75 баллов)**",
            "Вид особого права: право на прием без вступительных испытаний* или зачет максимального балла по предмету"]
    CONFIRM = ("Один или несколько предметов, по которым поступающим необходимы результаты ЕГЭ или "
               "общеобразовательных вступительных испытаний для подтверждения особого права")
    WHO = "Кому предоставляется особое право: победителям либо победителям и призерам олимпиады"
    GRADES = "В каких классах должны быть получены результаты победителя/призера олимпиады"
    HEAD_MSK = ["No п/п", "Наименование образовательной программы", "Полное наименование олимпиады из Перечня",
                "Профиль олимпиады", "Предмет олимпиады", CONFIRM, *TAIL, "Предмет зачета 100 баллов", WHO, GRADES]
    # В приложении Петербурга нет «Предмета зачета 100 баллов», зато есть «Предмет ЕГЭ, который подтверждает».
    HEAD_SPB = ["No п/п", "Наименование образовательной программы", "Полное наименование олимпиады из Перечня",
                "Профиль олимпиады", CONFIRM, "Предмет ЕГЭ, который подтверждает особое право", *TAIL, WHO, GRADES]

    def table(self, head, campus, *rows):
        return bb.hse_rows([(1, [(head, ("", "")), (["Направление подготовки 38.03.05 Бизнес-информатика"], ("", "")),
                                 *[(c, ("", name)) for name, c in rows]])], URL, campus)

    def test_petersburg_columns_by_header(self):
        row = ["Турнир городов", "математика", "математика", "математика", "75 и более", "Право на прием БВИ",
               "Победителям", "10, 11 класс"]
        r = self.table(self.HEAD_SPB, "Санкт-Петербург", ("Турнир городов", row))
        self.assertEqual((r[0]["statuses"], r[0]["grades"], r[0]["ege_subject"]), (["pobeditel"], [10, 11], "математика"))

    def test_merged_status_cell_carried_down(self):
        # Профиль даёт БВИ за 10–11 класс и 100 баллов за 9–11: «Кому» и предметы — одна ячейка на обе строки.
        bvi = ["", "информационная безопасность", "компьютерные и информационные науки", "информатика", "75 и более",
               "Право на прием БВИ", "информатика", "Победителям", "10, 11 класс"]
        cont = ["", "", "", "", "75 и более", "Право на 100 баллов", "", "", "9, 10, 11 класс"]
        r = self.table(self.HEAD_MSK, "Нижний Новгород", ("НТО", bvi), ("НТО", cont))
        self.assertEqual([(x["benefit"], x["statuses"], x["grades"], x["ege_subject"]) for x in r],
                         [("БВИ", ["pobeditel"], [10, 11], "информатика"),
                          ("100_ballov", ["pobeditel"], [9, 10, 11], "информатика")])

    def test_merged_score_cell_carried_down(self):
        # ПМИ, стр. 7: порог 90 — одна ячейка на строки БВИ и 100 баллов.
        bvi = ["", "информатика", "информатика", "информатика", "90 и более", "Право на прием БВИ", "информатика",
               "Победителям", "11 класс"]
        cont = ["", "", "", "", "", "Право на 100 баллов", "", "Победителям и призерам", "10, 11 класс"]
        r = self.table(self.HEAD_MSK, "Москва", ("Высшая проба", bvi), ("Высшая проба", cont))
        self.assertEqual([x["ege_score"] for x in r], [90, 90])

    def test_merged_status_not_carried_into_next_olympiad(self):
        first = ["", "информатика", "информатика", "информатика", "75 и более", "Право на прием БВИ", "информатика",
                 "Победителям", "11 класс"]
        other = ["", "математика", "математика", "математика", "75 и более", "Право на 100 баллов", "математика", "",
                 "11 класс"]
        r = self.table(self.HEAD_MSK, "Москва", ("НТО", first), ("Турнир городов", other))
        self.assertEqual([x["olympiad_name"] for x in r], ["НТО"])

    def test_ege_subject_is_the_confirming_one_not_olympiad_subject(self):
        row = ["", "интеллектуальные робототехнические системы", "компьютерные и информационные науки",
               "физика / информатика", "75 и более", "Право на 100 баллов", "физика / информатика",
               "Победителям и призерам", "11 класс"]
        r = self.table(self.HEAD_MSK, "Москва", ("НТО", row))
        self.assertEqual(r[0]["ege_subject"], "физика / информатика")


NSU_PROGRAMS = [
    prog("m-alg", "Алгебра и математическая логика", "Математика", "01.03.01"),
    prog("pmi-prog", "Программирование", "Прикладная математика и информатика", "01.03.02"),
    prog("mech", "Гидродинамика", "Механика и математическое моделирование", "01.03.03"),
    prog("mcs-prog", "Программирование", "Математика и компьютерные науки", "02.03.01"),
    prog("phys", "Физика", "Физика. Фундаментальные исследования", "03.03.02", "Физика"),
    prog("phys-inf", "Физическая информатика", "Физика. Фундаментальные исследования", "03.03.02", "Физика"),
]


class NsuTest(unittest.TestCase):
    """НГУ задаёт льготы по направлению (его профили в A — отдельные
    программы), а заголовок группы перечисляет её направления."""

    def targets(self, header):
        html = (f'<span class="name line">{header}</span>'
                + html_table(["", "Победители и призеры"], ["Математика", "Без экзаменов"]))
        return sorted(p["program_id"] for p in bb.link("nsu", bb.nsu_rows(html, URL)[0], NSU_PROGRAMS))

    def test_group_header_lists_its_directions(self):
        self.assertEqual(self.targets("Математика и механика (01.03.00, бакалавр): (Математика (01.03.01); Прикладная "
                                      "математика и информатика (01.03.02); Механика и математическое моделирование (01.03.03))"),
                         ["m-alg", "mech", "pmi-prog"])

    def test_direction_covers_all_its_profiles(self):
        self.assertEqual(self.targets("Физика. Фундаментальная и экспериментальная физика (03.03.02, бакалавр)"),
                         ["phys", "phys-inf"])

    def test_same_program_name_in_another_direction_is_not_matched(self):
        self.assertEqual(self.targets("Математика и компьютерные науки (02.03.01, бакалавр) (в том числе "
                                      "Системное программирование)"), ["mcs-prog"])


class NsuRowsTest(unittest.TestCase):
    """olimpiady-privilege: классы — из сноски под своей таблицей РСОШ;
    у биологии льгота — матрица «степень диплома × уровень олимпиады»."""
    HEAD3 = ["Предмет олимпиады/Профиль олимпиады", "Предмет вступительных испытаний", "Льгота"]
    HEADM = ["Предмет олимпиады/Профиль олимпиады", "Предмет вступительных испытаний",
             "I", "II", "III", "I", "II", "III", "I", "II", "III"]

    @staticmethod
    def foot(grades):
        return ("<p>* льгота предоставляется при наличии результатов ЕГЭ не ниже 75 баллов по предмету "
                f"олимпиады для абитуриентов, обучавшихся в период участия в олимпиаде в {grades}</p>")

    def rows(self, body, header="Мехатроника и робототехника (15.03.06, бакалавр)"):
        return [r for r in bb.nsu_rows(f'<span class="name line">{header}</span>' + body, URL) if not r.get("vsosh")]

    def test_grades_from_footnote(self):
        r = self.rows(html_table(self.HEAD3, ["Информатика", "Информатика и ИКТ", "Без экзаменов"]) + self.foot("9-11 класс"))
        self.assertEqual([x["grades"] for x in r], [[9, 10, 11]])

    def test_informatics_and_ict_is_the_informatics_profile(self):
        r = self.rows(html_table(self.HEAD3, ["Информатика и ИКТ", "Информатика и ИКТ", "Без экзаменов"]) + self.foot("10-11 классе"))
        self.assertTrue(bb.expand_profile(r[0]["profile"], None))

    def test_matrix_by_degree_level_and_grade(self):
        body = (html_table(self.HEADM, ["Биология", "Биология"] + ["Без экзам."] * 9,
                           ["Математика", "Математика"] + ["100"] * 8 + ["—"]) + self.foot("<b>11 классе</b>")
                + html_table(self.HEADM, ["Биология", "Биология"] + ["Без экзам."] * 7 + ["100", "—"])
                + self.foot("<b>10 классе </b>"))
        got = sorted((r["profile"], tuple(r["statuses"]), r["benefit"], tuple(r["levels"]), tuple(r["grades"]))
                     for r in self.rows(body, "Биология (06.03.01, бакалавр)"))
        self.assertEqual(got, [
            ("Биология", ("pobeditel",), "БВИ", ("I", "II", "III"), (10,)),
            ("Биология", ("pobeditel",), "БВИ", ("I", "II", "III"), (11,)),
            # призёр — худшее из II и III степени: у III степени за 10 класс
            # уровень II — 100 баллов, уровень III — ничего
            ("Биология", ("prizyor",), "100_ballov", ("II",), (10,)),
            ("Биология", ("prizyor",), "БВИ", ("I",), (10,)),
            ("Биология", ("prizyor",), "БВИ", ("I", "II", "III"), (11,)),
            ("Математика", ("pobeditel",), "100_ballov", ("I", "II", "III"), (11,)),
            ("Математика", ("prizyor",), "100_ballov", ("I", "II"), (11,)),
        ])


class SechenovTest(unittest.TestCase):
    ROW = ["1.", "Олимпиада школьников «Физтех»", "физика", "Физика", "физика", "Право на 100 баллов"]

    def test_grades_from_document_text(self):
        # приложение 5, стр. 1: «Результаты победителя (призера) должны быть получены за 10 или 11 класс»
        pages = [{"page": 1, "text": "Результаты победителя (призера) должны быть получены за 10 или 11 класс Особые права",
                  "tables": [[self.ROW]]}]
        self.assertEqual([(r["benefit"], r["grades"]) for r in bb.sechenov_rows(pages, URL)], [("100_ballov", [10, 11])])

    def test_no_grade_clause(self):
        pages = [{"page": 1, "text": "", "tables": [[self.ROW]]}]
        self.assertEqual([r["grades"] for r in bb.sechenov_rows(pages, URL)], [None])


class CanonSubjectTest(unittest.TestCase):
    """Предмет ЕГЭ для подтверждения — только из списка предметов ЕГЭ.
    Остальное — обрывки соседних колонок PDF, в примечание им нельзя."""

    def test_cases(self):
        for raw, want in {
            "Обществоз нанию": "Обществознание",
            "Информат ика и ИКТ": "Информатика",
            "физика / информатика": "Физика или Информатика",
            "Математике или … обществознанию - по выбору абитуриента": "Математика или Обществознание",
            "английский язык": "Иностранный язык",
            "Русский язык": "Русский язык",
            "—": None, "-": None, "": None, None: None,
            "технологии материалов, машиностроение, электроника, радиотехника и системы связи": None,
            "клиническая медицина, фармация, фундаментальная медицина": None,
        }.items():
            with self.subTest(raw=raw):
                self.assertEqual(bb.canon_subject(raw), want)


class NarrowSubjectTest(unittest.TestCase):
    """Документ даёт предметы профиля («математика, обществознание»), а
    подтверждать надо тот, что среди ВИ программы — если ВИ известны."""

    def test_only_subjects_among_program_exams(self):
        self.assertEqual(bb.narrow_subject("Математика или Обществознание", {"Обществознание", "История", "Русский язык"}),
                         "Обществознание")

    def test_exams_unknown(self):
        self.assertEqual(bb.narrow_subject("Математика или Обществознание", None), "Математика или Обществознание")

    def test_no_overlap_keeps_document(self):
        self.assertEqual(bb.narrow_subject("Физика", {"Математика", "Русский язык"}), "Физика")


def benefit(grades, score=75, status="pobeditel", kind="БВИ"):
    return {"olympiad_id": "p669-8-matematika", "diploma_status": status, "benefit_type": kind,
            "eligible_grades": grades, "ege_confirm_min_score": score}


class DedupBenefitsTest(unittest.TestCase):
    """Одна льгота из нескольких строк документа (БВИ за 10 класс и за 11)
    не должна терять классы и порог второй строки."""

    def test_grades_of_duplicate_rows_are_merged(self):
        got = bb.dedup_benefits([benefit([11]), benefit([10])])
        self.assertEqual([b["eligible_grades"] for b in got], [[10, 11]])

    def test_no_grade_restriction_wins(self):
        self.assertEqual([b["eligible_grades"] for b in bb.dedup_benefits([benefit([11]), benefit(None)])], [None])

    def test_stricter_score_kept(self):
        self.assertEqual([b["ege_confirm_min_score"] for b in bb.dedup_benefits([benefit([11], 75), benefit([11], 80)])], [80])

    def test_confirm_subjects_of_both_rows_kept(self):
        # СПбГУ, бизнес-информатика: одна олимпиада подходит и под ВИ
        # «Обществознание», и под ВИ «Информатика» — подтвердить можно любым.
        a, b = benefit([10, 11]), benefit([10, 11])
        a["ege_confirm_subject"], b["ege_confirm_subject"] = "Обществознание", "Информатика"
        got = bb.dedup_benefits([a, b, dict(b)])
        self.assertEqual([x["ege_confirm_subject"] for x in got], ["Обществознание или Информатика"])

    def test_different_benefits_stay_separate(self):
        got = bb.dedup_benefits([benefit([10, 11]), benefit([9, 10, 11], kind="100_ballov")])
        self.assertEqual([(b["benefit_type"], b["eligible_grades"]) for b in got],
                         [("БВИ", [10, 11]), ("100_ballov", [9, 10, 11])])


MSU_PROGRAMS = [
    prog("sev-pmi", "Прикладная математика и информатика", "Филиал МГУ в г. Севастополе"),
    prog("fhb", "Фундаментальная и прикладная биология — Физико-химическая биология. Общая биология)",
         "Биологический факультет", "06.05.02", "Биомед"),
    prog("culture", "Менеджмент — Менеджмент в культуре)", "Высшая школа культурной политики и управления",
         "38.03.02", "Экономика"),
    prog("audit", "Экономика (профиль Государственный и муниципальный аудит)", "Высшая школа государственного аудита",
         "38.03.01", "Экономика"),
]


class MsuLinkTest(unittest.TestCase):
    def targets(self, faculty, napr):
        row = {"match": {"faculty": faculty, "napravlenie": napr}}
        return [p["program_id"] for p in bb.link("msu", row, MSU_PROGRAMS)]

    def test_unknown_direction_gets_nothing_not_whole_faculty(self):
        self.assertEqual(self.targets("Филиал МГУ в г. Севастополе", "Психология"), [])
        self.assertEqual(self.targets("Филиал МГУ в г. Севастополе", "Прикладная математика и информатика"), ["sev-pmi"])

    def test_program_group_must_match(self):
        self.assertEqual(self.targets("Биологический факультет", "Фундаментальная и прикладная биология (группа программ "
                                      "«Биоинженерия и биотехнология. Биофизика»)"), [])
        self.assertEqual(self.targets("Биологический факультет", "Фундаментальная и прикладная биология (группа программ "
                                      "«Физико- химическая биология. Общая биология»)"), ["fhb"])

    def test_profile_in_brackets(self):
        f = "Высшая школа культурной политики и управления"
        self.assertEqual(self.targets(f, "Менеджмент (Менеджмент в спорте)"), [])
        self.assertEqual(self.targets(f, "Менеджмент (Менеджмент в культуре)"), ["culture"])

    def test_plain_direction_covers_profiled_program(self):
        self.assertEqual(self.targets("Высшая школа государственного аудита", "Экономика"), ["audit"])

    def test_school_section_with_faculty_suffix(self):
        # Секции высших школ: «ВЫСШАЯ ШКОЛА ГОСУДАРСТВЕННОГО АУДИТА (ФАКУЛЬТЕТ)».
        self.assertEqual(self.targets("ВЫСШАЯ ШКОЛА ГОСУДАРСТВЕННОГО АУДИТА (ФАКУЛЬТЕТ)", "Экономика"), ["audit"])

    KCP = [{"page": 2, "tables": [[
        ["Факультет, … образовательная программа", None, "Количество бюджетных мест", "…", "Вступительные испытания"],
        ["Факультет вычислительной математики и кибернетики", None, None, None, None],
        ["01.03.02", 'Направление подготовки "Прикладная математика и информатика" (очная форма, бюджет и договор)',
         "312", "120", "математика (ДВИ, письменно) (1) математика (ЕГЭ) (2) физика (ЕГЭ) (3) "
                       "информатика (ЕГЭ) (4) русский язык (ЕГЭ) (5)"],
        [None, "в том числе для поступающих по особой квоте", "32", "", None],
        ["Филиал МГУ в г. Севастополе", None, None, None, None],
        ["03.05.02", 'Специальность "Фундаментальная и прикладная физика" (очная форма, бюджет и договор)', "8", "5",
         "физика (ДВИ, письменно) (1) физика (ЕГЭ) (2) математика (ЕГЭ) или информатика (ЕГЭ), или химия (ЕГЭ) "
         "(один по выбору поступающего) (3) русский язык (ЕГЭ) (4)"],
    ]]}]

    def test_program_exams_from_plan(self):
        # ВИ по ЕГЭ — те, где 100 баллов по ВсОШ засчитываются (п. 26 Правил); ДВИ — нет.
        self.assertEqual(bb.msu_program_exams(self.KCP), {
            ("факультет вычислительной математики и кибернетики", "01.03.02", "Прикладная математика и информатика"):
                {"Математика", "Физика", "Информатика", "Русский язык"},
            ("филиал мгу в г. севастополе", "03.05.02", "Фундаментальная и прикладная физика"):
                {"Физика", "Математика", "Информатика", "Химия", "Русский язык"}})

    def test_vsosh_hundred_rows_link_to_program(self):
        rows = bb.msu_vsosh_hundred_rows(self.KCP, URL)
        pmi = [r for r in rows if r["match"]["msu_program"][1] == "01.03.02"]
        self.assertEqual(sorted(r["profile"] for r in pmi), ["информатика", "математика", "русский язык", "физика"])
        self.assertEqual({(r["benefit"], tuple(r["statuses"]), r["unless_bvi"]) for r in rows},
                         {(bb.HUNDRED, (bb.POB, bb.PRIZ), True)})
        progs = [prog("pmi", "Прикладная математика и информатика", "Факультет вычислительной математики и кибернетики"),
                 prog("pmi-sev", "Прикладная математика и информатика", "Филиал МГУ в г. Севастополе")]
        self.assertEqual([p["program_id"] for p in bb.link("msu", pmi[0], progs)], ["pmi"])

    def test_hundred_dropped_where_same_diploma_gives_bvi(self):
        recs = [{"olympiad_id": "vsosh-fizika", "diploma_status": "pobeditel", "benefit_type": "БВИ"},
                {"olympiad_id": "vsosh-fizika", "diploma_status": "pobeditel", "benefit_type": "100_ballov",
                 "_unless_bvi": True},
                {"olympiad_id": "vsosh-himiya", "diploma_status": "pobeditel", "benefit_type": "100_ballov",
                 "_unless_bvi": True}]
        self.assertEqual([(r["olympiad_id"], r["benefit_type"], "_unless_bvi" in r) for r in bb.drop_unless_bvi(recs)],
                         [("vsosh-fizika", "БВИ", False), ("vsosh-himiya", "100_ballov", False)])

    def test_sevastopol_physics(self):
        # Севастополь, стр. 60–61: «Физика» — это 03.05.02 «Фундаментальная и
        # прикладная физика» (kcp стр. 30), другой физики у филиала нет.
        progs = [prog("sev-fiz", "Фундаментальная и прикладная физика", "Филиал МГУ в г. Севастополе", "03.05.02")]
        row = {"match": {"faculty": "ФИЛИАЛ МГУ В Г. СЕВАСТОПОЛЕ", "napravlenie": "Физика"}}
        self.assertEqual([p["program_id"] for p in bb.link("msu", row, progs)], ["sev-fiz"])

    def test_vsosh_skips_international_olympiads(self):
        pages = [{"page": 6, "tables": [[
            ["Биологический факультет", None, None],
            ["Экология и природопользование", "Всероссийская олимпиада школьников", "Экология"],
            [None, None, "Биология"],
            [None, "Международная биологическая олимпиада", "Биология"],
        ]]}]
        got = [(r["profile"], r["match"]["napravlenie"]) for r in bb.msu_vsosh_rows(pages, URL)]
        self.assertEqual(got, [("экология", "Экология и природопользование"), ("биология", "Экология и природопользование")])


class KazanGmuTest(unittest.TestCase):
    def test_row_goes_only_to_its_section(self):
        pages = [{"page": 1, "tables": [[
            ["Специальность (направление подготовки) – Лечебное дело, Педиатрия", "", "", "", ""],
            ["11", "Всероссийская Сеченовская олимпиада школьников", "химия", "химия", "II"],
            ["Специальность (направление подготовки) –Медицинская биофизика", "", "", "", ""],
            ["8", "Всероссийская олимпиада школьников «Высшая проба»", "физика", "физика", "II"],
        ]]}]
        progs = [prog("led", "Лечебное дело", "", "31.05.01", "Биомед"),
                 prog("bioph", "Медицинская биофизика", "", "30.05.02", "Биомед")]
        got = [(r["profile"], [p["program_id"] for p in bb.link("kazan-gmu", r, progs)])
               for r in bb.kazan_gmu_rows(pages, URL)]
        self.assertEqual(got, [("химия", ["led"]), ("физика", ["bioph"])])


class SpbuTest(unittest.TestCase):
    """СПбГУ: «—» в перечне — не ВсОШ, а любая олимпиада уровня с профилем
    или предметом; ВсОШ — отдельным документом."""

    ROWS = [{"page": 1, "tables": [[
        ["01.03.01 Математика", "Математика (с дополнительной квалификацией)", "очная", "1", "Математика", "85",
         "I", "—", "—", "Математика", "Победитель, призѐр", "Без вступительных испытаний"],
        ["", "", "", "2", "Информатика", "75", "I–II", "—", "Информатика", "Информатика", "Призѐр",
         "100 баллов за ВИ по информатике"],
        ["01.05.01 Фундаментальные математика и механика", "Фундаментальная математика", "очная", "1", "Математика",
         "75", "I–III", "—", "—", "Математика", "Победитель, призѐр", "Без вступительных испытаний"],
    ]]}]

    def test_dash_rows_are_perechen_not_vsosh(self):
        rows = bb.spbu_rows(self.ROWS, URL)
        self.assertFalse(any(r.get("vsosh") for r in rows))
        self.assertEqual([(r.get("by_subject", False), r.get("by_profile", False), r["levels"]) for r in rows],
                         [(True, False, ["I"]), (False, True, ["I", "II"]), (True, False, ["I", "II", "III"])])

    def test_program_absent_from_a_gets_nothing(self):
        progs = [prog("mm", "Механика и математическое моделирование (с доп. квалификацией)", "", "01.05.01")]
        r = bb.spbu_rows(self.ROWS, URL)[2]
        self.assertEqual(bb.link("spbu", r, progs), [])

    def test_vsosh_document(self):
        pages = [{"page": 1, "tables": [[
            ["Предмет Всероссийской олимпиады школьников", "Наименование профильных направлений", "Уровень"],
            ["Астрономия", "01.03.01 Математика", "Бакалавриат"],
            ["Астрономия", "02.03.01 Математика и компьютерные науки (ОП «AI360: Математика машинного обучения»)", "Бакалавриат"],
        ]]}]
        got = [(r["profile"], r["match"]) for r in bb.spbu_vsosh_rows(pages, URL)]
        self.assertEqual(got, [("астрономия", {"spbu": None, "code": "01.03.01"}),
                               ("астрономия", {"spbu": "AI360: Математика машинного обучения", "code": "02.03.01"})])

    def test_subject_expansion_keeps_exact_levels(self):
        got = {lv for *_, lv in bb.expand_subject("Математика")}
        self.assertEqual(got, {"I", "II", "III"})

    def test_statuses_with_io_letter(self):
        # В PDF СПбГУ «Призѐр» набран через «ѐ» (U+0450), NFKC её не трогает.
        self.assertEqual(bb._statuses("Победитель, призѐр"), [bb.POB, bb.PRIZ])
        self.assertEqual(bb._statuses("Призѐр"), [bb.PRIZ])

    def test_subject_expansion_is_exact(self):
        # Колонка — предмет или УГН перечня: «Математика» не значит УГН
        # «математика и механика»; астрономию и УГН тоже надо находить.
        math = {oid for oid, *_ in bb.expand_subject("Математика")}
        self.assertIn("p669-50-matematika", math)
        self.assertNotIn("p669-50-mehanika-i-matematicheskoe-modelirovanie", math)
        self.assertNotIn("p669-8-inzhenernye-nauki", {oid for oid, *_ in bb.expand_subject("Информатика")})
        self.assertIn("p669-37-astronomiya", {oid for oid, *_ in bb.expand_subject("Астрономия")})
        self.assertIn("p669-5-yadernye-tehnologii",
                      {oid for oid, *_ in bb.expand_subject("Управление в технических системах")})

    VI_ROWS = [{"page": 5, "text": "Особые права предоставляются … (только при получении результатов за 10 или 11 "
                                   "класс) – в течение 4 лет", "tables": [[
        ["38.03.05 Бизнес-информатика", "Бизнес-информатика", "очная", "1", "Обществознание", "75",
         "I–II", "—", "—", "Экономика", "Победитель", "Без вступительных испытаний"],
        [None, None, None, None, None, None, "I–II", "—", "—", "Экономика", "Призѐр",
         "100 баллов за ВИ по обществознанию"],
        ["31.05.01 Лечебное дело", "Лечебное дело", "очная", "1", "Биология", "75",
         "I–II", "—", "Медицина", "—", "Победитель, призѐр", "Без вступительных испытаний"],
        [None, None, None, "2", "Химия", "80", "I", "—", "—", "Химия", "Победитель", "Без вступительных испытаний"],
        [None, None, None, None, None, None, "II", "—", "—", "Химия", "Призѐр", "100 баллов за ВИ по химии"],
    ]]}]

    def test_ege_is_row_vi_carried_down_merged_cells(self):
        # Предмет и порог — объединённые ячейки ВИ: действуют на все его строки.
        got = [(r["ege_subject"], r["ege_score"]) for r in bb.spbu_rows(self.VI_ROWS, URL)]
        self.assertEqual(got, [("Обществознание", 75), ("Обществознание", 75), ("Биология", 75),
                               ("Химия", 80), ("Химия", 80)])

    def test_grades_from_header_clause(self):
        self.assertEqual({tuple(r["grades"]) for r in bb.spbu_rows(self.VI_ROWS, URL)}, {(10, 11)})

    def test_vsosh_hundred_by_appendix_7(self):
        # Правила п. 7.5 и приложение 7: ВсОШ — ещё и 100 баллов за ВИ,
        # соответствующее предмету, на любой ОП с таким ВИ. Где по этой ВсОШ
        # уже БВИ (приложение 4), 100 баллов ничего не добавляют.
        rules = [{"page": 78, "text": "Приложение 7 к Правилам приема …", "tables": [[
            ["Предмет всероссийской олимпиады школьников", "Предмет вступительных испытаний"],
            ["Астрономия", "Физика"], ["Математика", "Математика"], ["Технология", "—"],
        ]]}]
        olymp = [{"page": 3, "tables": [[
            ["09.03.04 Программная инженерия", "Программная инженерия", "очная", "1", "Математика", "75",
             "I–II", "—", "—", "Математика", "Победитель", "Без вступительных испытаний"],
            [None, None, None, "2", "Информатика", "75", "I–II", "—", "—", "Информатика", "Победитель",
             "Без вступительных испытаний"],
            ["03.05.01 Астрономия", "Астрономия", "очная", "1", "Физика", "75",
             "I–III", "—", "—", "Физика", "Победитель, призѐр", "Без вступительных испытаний"],
        ]]}]
        bvi = [{"match": {"spbu": None, "code": "03.05.01"}, "profile": "астрономия"}]
        got = [(r["profile"], r["match"], r["benefit"], r["statuses"])
               for r in bb.spbu_vsosh_hundred_rows(rules, olymp, bvi, URL)]
        self.assertEqual(got, [("математика", {"spbu": "Программная инженерия", "code": "09.03.04"},
                                bb.HUNDRED, [bb.POB, bb.PRIZ])])


class CatalogScopeTest(unittest.TestCase):
    """Граница B — олимпиады каталога (C и missing_in_C): сид не знает других,
    а у тех, что в каталоге есть, льготы терять нельзя."""

    def test_subject_expansion_follows_catalog(self):
        got = {oid for oid, *_ in bb.expand_subject("Математика")}
        self.assertIn("p669-37-veroyatnost-i-statistika", got)   # в C, в профиле нет «математик»
        self.assertNotIn("p669-82-lingvistika", got)              # в каталоге нет

    def test_catalog_membership(self):
        self.assertTrue(bb.in_catalog("p669-54-nauchno-tehnicheskiy"))   # C
        self.assertTrue(bb.in_catalog("p669-59-medicina"))              # missing_in_C
        self.assertFalse(bb.in_catalog("p669-82-lingvistika"))
        self.assertFalse(bb.in_catalog("vsosh-russkiy-yazyk"))


class ConditionsTest(unittest.TestCase):
    """Условия записи: ВсОШ результатом ЕГЭ не подтверждается (ч. 4 ст. 71),
    поэтому ни предмета, ни порога, и отсутствие порога — не заглушка."""

    def test_vsosh_has_no_ege_confirmation(self):
        row = {"ege_subject": "биология", "ege_score": None, "score_is_demo": True}
        self.assertEqual(bb.conditions("vsosh-biologiya", row, "kfu", {}),
                         {"ege_confirm_subject": None, "ege_confirm_min_score": None, "is_demo": False})

    def test_vsosh_demo_benefit_stays_demo(self):
        row = {"ege_subject": None, "ege_score": None, "benefit_is_demo": True}
        self.assertTrue(bb.conditions("vsosh-himiya", row, "kazan-gmu", {})["is_demo"])

    def test_perechen_keeps_confirmation(self):
        row = {"ege_subject": "физика", "ege_score": 75}
        self.assertEqual(bb.conditions("p669-54-fizika", row, "itmo", {}),
                         {"ege_confirm_subject": "Физика", "ege_confirm_min_score": 75, "is_demo": False})

    def test_perechen_without_score_is_demo(self):
        row = {"ege_subject": "физика", "ege_score": None}
        self.assertTrue(bb.conditions("p669-54-fizika", row, "itmo", {})["is_demo"])


class VsoshCodeTableTest(unittest.TestCase):
    """Сеченов п. 5.1 и КГМУ п. 6.4: таблица «код — специальность — профили
    ВсОШ». Пустая ячейка профилей — объединённая: профили строки выше."""
    PAGES = [{"page": 13, "tables": [[
        ["", "профилю заключительного этапа всероссийской", "международных олимпиад школьников"],
        ["30.05.02", "Медицинская биофизика", "Физика, Математика, Биология"],
        ["31.05.01", "Лечебное дело", "Химия, Биология"],
        ["31.05.02", "Педиатрия", None],
    ]]}]

    def test_bvi_by_code_with_merged_cells(self):
        got = [(r["profile"], r["match"]) for r in bb.vsosh_code_rows(self.PAGES, URL)]
        self.assertEqual(got, [
            ("физика", {"codes": ["30.05.02"]}), ("математика", {"codes": ["30.05.02"]}),
            ("биология", {"codes": ["30.05.02"]}),
            ("химия", {"codes": ["31.05.01"]}), ("биология", {"codes": ["31.05.01"]}),
            ("химия", {"codes": ["31.05.02"]}), ("биология", {"codes": ["31.05.02"]}),
        ])

    def test_rows_are_vsosh_bvi_to_winners_and_prizers(self):
        r = bb.vsosh_code_rows(self.PAGES, URL)[0]
        self.assertEqual((r["vsosh"], r["level"], r["benefit"], r["statuses"], r["page"], r["grades"]),
                         (True, "ВсОШ", "БВИ", ["pobeditel", "prizyor"], 13, None))

    def test_links_by_code(self):
        progs = [{"program_name": "Педиатрия", "napravlenie_code": "31.05.02"},
                 {"program_name": "Лечебное дело", "napravlenie_code": "31.05.01"}]
        rows = bb.vsosh_code_rows(self.PAGES, URL)
        self.assertEqual([p["program_name"] for p in bb.link("kazan-gmu", rows[-1], progs)], ["Педиатрия"])


class SechenovVsoshHundredTest(unittest.TestCase):
    """Сеченов п. 5.3: особое преимущество ВсОШ — 100 баллов за ВИ,
    совпадающее с профилем; экономика засчитывается как обществознание."""
    PAGES = [{"page": 14, "tables": [[
        ["Профиль заключительного этапа всероссийской\nолимпиады школьников", "Предмет общеобразовательного вступительного\nиспытания"],
        ["физика", "физика"],
        ["экономика", "обществознание"],
    ]]}]

    def test_hundred_by_exam(self):
        got = [(r["profile"], r["match"], r["benefit"], r["statuses"])
               for r in bb.sechenov_vsosh_hundred_rows(self.PAGES, URL)]
        self.assertEqual(got, [("физика", {"exams": ["Физика"]}, "100_ballov", ["pobeditel", "prizyor"]),
                               ("экономика", {"exams": ["Обществознание"]}, "100_ballov", ["pobeditel", "prizyor"])])


class GradesClauseTest(unittest.TestCase):
    """Классы диплома — из текста документа, а не пусто по умолчанию."""

    def test_kfu_clause(self):
        pages = [{"page": 1, "text": "3) победители и призеры олимпиад школьников за 10 и 11 класс, проводимых"}]
        self.assertEqual(bb.kfu_grades(pages), [10, 11])

    def test_kfu_clause_missing_means_unknown(self):
        self.assertIsNone(bb.kfu_grades([{"page": 1, "text": "Право на прием"}]))


class KazanGmuRightsTest(unittest.TestCase):
    """КГМУ, таблица п. 6.5: профиль олимпиады -> предметы подтверждения и класс."""
    PAGES = [{"page": 3, "tables": [[
        ["Специальность\n(направление\nподготовки)", "Уровень\nолимпиады", "Профиль\nолимпиады",
         "Общеобра-\nзовательные\nпредметы", "Класс\nобучения", "Основание"],
        ["Лечебное дело\nПедиатрия", "Олимпиада\n1, 2, 3", "Химия", "Химия", "11", "Диплом"],
        ["Лечебное дело", "Олимпиада\n1, 2, 3", "Медицина", "Химия /\nБиология", "11", "Диплом"],
        ["Социальная работа", "Олимпиада\n1, 2, 3", "Обществозна\nние", "Обществозна\nние", "11", "Диплом"],
    ]]}]

    def test_rights_by_profile(self):
        self.assertEqual(bb.kgmu_rights(self.PAGES), {
            "химия": ("Химия", [11]),
            "медицина": ("Химия или Биология", [11]),
            "обществознание": ("Обществознание", [11]),
        })

    def test_rows_take_subject_and_grades_from_rights(self):
        rows = [{"profile": "медицина", "ege_subject": "клиническая медицина, фармация", "grades": None},
                {"profile": "физика", "ege_subject": "физика", "grades": None}]
        got = bb.with_kgmu_rights(rows, bb.kgmu_rights(self.PAGES))
        self.assertEqual([(r["ege_subject"], r["grades"]) for r in got],
                         [("Химия или Биология", [11]), ("физика", None)])


class MiptGradesTest(MiptPlanMixin, unittest.TestCase):
    """2026_olympiads, п. 4–6: РСОШ — только за 11 класс; победителям
    «Физтеха» (№ 54) — и за 10; победителям олимпиады по ИИ (№ 7) за 10
    класс — только на программах ВШПИ."""

    def grades(self, num, cells, hundred=""):
        html = html_table(MiptMatrixTest.HEAD, [num, "Олимпиада", "профиль", "1", hundred] + cells)
        return sorted({(r["benefit"], s, r["match"].get("school") or "", tuple(r["grades"] or ()))
                       for r in bb.mipt_rows(html, URL) for s in r["statuses"]})

    def cells(self, **by_school):
        out = [""] * 11
        for i, name in enumerate(SCHOOLS):
            out[i] = by_school.get(name.replace("/", "").replace(" ", ""), "")
        return out

    def test_perechen_only_for_11th_grade(self):
        got = self.grades("56", self.cells(ФРКТ="Все конкурсные группы ФРКТ Победителям и призерам при наличии "
                                                 "результата ЕГЭ или ВИ по информатике 80 баллов и выше"),
                          hundred="100 баллов по информатике при наличии результата ЕГЭ или ВИ по информатике 75 баллов и выше")
        self.assertEqual(got, [("100_ballov", "pobeditel", "", (11,)), ("100_ballov", "prizyor", "", (11,)),
                               ("БВИ", "pobeditel", "ФРКТ", (11,)), ("БВИ", "prizyor", "ФРКТ", (11,))])

    def test_phystech_winner_also_10th_grade(self):
        got = self.grades("54", self.cells(ЛФИ="Все конкурсные группы ЛФИ Победителям и призерам при наличии "
                                                "результата ЕГЭ или ВИ по физике 75 баллов и выше"),
                          hundred="100 баллов по физике при наличии результата ЕГЭ или ВИ по физике 75 баллов и выше")
        self.assertEqual(got, [("100_ballov", "pobeditel", "", (10, 11)), ("100_ballov", "prizyor", "", (11,)),
                               ("БВИ", "pobeditel", "ЛФИ", (10, 11)), ("БВИ", "prizyor", "ЛФИ", (11,))])

    def test_ai_olympiad_10th_grade_only_at_vshpi(self):
        cell = "Все конкурсные группы {} Победителям при наличии результата ЕГЭ или ВИ по математике 85 баллов и выше"
        got = self.grades("7", self.cells(ФПМИ=cell.format("ФПМИ"), ВШПИ=cell.format("ВШПИ")))
        self.assertEqual(got, [("БВИ", "pobeditel", "ВШПИ", (10, 11)), ("БВИ", "pobeditel", "ФПМИ", (11,))])


class MiptVsoshTest(unittest.TestCase):
    """Приложение 2: у ВсОШ по информатике строки подпрофилей (ИИ, ИБ,
    робототехника) дают БВИ шире базовой. В каталоге одна vsosh-informatika —
    её условия в базовой строке «Информатика; …профиль "Программирование"»."""

    def test_subprofile_rows_are_not_the_base_olympiad(self):
        empty = [""] * 10
        html = html_table(["Общеобразовательный предмет"] + SCHOOLS,
                          ['Информатика; Информатика, профиль "Программирование"',
                           '"Компьютерные технологии и вычислительная техника" Победителям и призерам'] + empty,
                          ['Информатика, профиль "Информационная безопасность"',
                           "Все конкурсные группы ФРКТ Победителям и призерам"] + empty)
        got = [r["match"] for r in bb.mipt_vsosh_rows(html, URL)]
        self.assertEqual(got, [{"school": "ФРКТ", "groups": ["Компьютерные технологии и вычислительная техника"]}])

    def test_typo_in_subject_still_recognized(self):
        # Прил. 1, № 50, ПИШ ФАЛТ: «…по тнформатике 80 баллов и выше»
        self.assertEqual(bb._subject_from("ЕГЭ или ВИ по тнформатике 80 баллов и выше"), "Информатика")


if __name__ == "__main__":
    unittest.main()
