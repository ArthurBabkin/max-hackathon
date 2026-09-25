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
]


def html_table(*rows):
    tr = "".join("<tr>" + "".join(f"<td>{c}</td>" for c in r) + "</tr>" for r in rows)
    return f"<table>{tr}</table>"


def mipt_targets(rows):
    """(олимпиада, льгота, статусы, предмет ЕГЭ, порог) -> программы."""
    return {(r.get("olympiad_name") or r["profile"], r["benefit"], tuple(r["statuses"]),
             r["ege_subject"], r["ege_score"]):
            sorted(p["program_id"] for p in bb.link("mipt", r, MIPT_PROGRAMS))
            for r in rows}


class MiptMatrixTest(unittest.TestCase):
    """Матрица МФТИ «олимпиада × физтех-школа»: БВИ дают не вузу, а школе,
    а внутри школы — иногда только названным конкурсным группам."""

    HEAD = ["№ в перечне", "Полное наименование олимпиады", "Профиль олимпиады", "Уровень",
            "Особое право получения 100 баллов"] + SCHOOLS

    def rows(self, *cells):
        html = html_table(self.HEAD, ["54", "Олимпиада школьников \"Физтех\"", "биология", "2",
                                      "100 баллов по биологии при наличии результата ЕГЭ или ВИ по биологии 75 баллов и выше"]
                          + list(cells))
        return [r for r in bb.mipt_rows(html, URL) if r["benefit"] == "БВИ"]

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

    def test_only_on_direction(self):
        rows = bb.itmo_rows([(2, ["09.03.01 ИВТ, 10.03.01 ИБ", "Все из перечня олимпиад школьников",
                                  "Информационная безопасность (только на направление 10.03.01)",
                                  "Информатика", "1, 2 или 3", "Победитель или призер"])], URL, "100_ballov")
        self.assertEqual([(r["match"]["codes"], r["profile"]) for r in rows],
                         [(["10.03.01"], "Информационная безопасность")])


class ExpandProfileTest(unittest.TestCase):
    def test_part_of_composite_nto_profile(self):
        got = [oid for oid, *_ in bb.expand_profile("Аэрокосмические системы", "II")]
        self.assertEqual(len(got), 1)
        self.assertTrue(got[0].startswith("p669-5-bespilotnyy-transport-aerokosmicheskie-sistemy"))

    def test_hyphen_split_by_line_break(self):
        self.assertEqual([o for o, *_ in bb.expand_profile("Автоматизация бизнес- процессов", "II")],
                         ["p669-5-avtomatizaciya-biznes-processov"])

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

    def test_listed_code_without_profile_needs_subject_among_exams(self):
        self.assertEqual(self.targets("44.03.05 Педагогическое образование (с двумя профилями подготовки)", "биология"),
                         ["ped-bio"])


if __name__ == "__main__":
    unittest.main()
