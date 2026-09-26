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
            for r in bb.innopolis_rows(pages, URL)]


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
        html = html_table(self.HEAD, ["54", "Олимпиада школьников \"Физтех\"", "биология", "2", hundred]
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


if __name__ == "__main__":
    unittest.main()
