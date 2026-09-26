"""Инварианты генератора сида. Запуск: make test-db или
python3 -m unittest discover -s datasets/parser -p 'test_*.py'."""
import re
import unittest
from collections import Counter

import build_seed as bs


def rec(status, benefit, score=75, subject="Информатика", grades=None, demo=False,
        url="https://vuz.example/olymp.pdf", page=1):
    return {
        "diploma_status": status, "benefit_type": benefit, "ege_confirm_min_score": score,
        "ege_confirm_subject": subject, "eligible_grades": grades, "is_demo": demo,
        "source_url": url, "source_page": page, "source_date": "2026-09-21",
    }


class AggregateKeyTest(unittest.TestCase):
    """Все сочетания статуса диплома и вида льготы внутри одного ключа."""

    def test_bvi_for_winner_and_prize_winner(self):
        a = bs.aggregate_key([rec("pobeditel", "БВИ"), rec("prizyor", "БВИ")])
        self.assertEqual(a["benefit"], "bvi")
        self.assertEqual(a["note"], "Подтвердить ЕГЭ: Информатика")

    def test_bvi_for_winner_score100_for_prize_winner(self):
        a = bs.aggregate_key([rec("pobeditel", "БВИ"), rec("prizyor", "100_ballov")])
        self.assertEqual(a["benefit"], "bvi_winners")
        self.assertIn("Победителю — БВИ, призёру — 100 баллов", a["note"])

    def test_bvi_only_for_winner(self):
        a = bs.aggregate_key([rec("pobeditel", "БВИ")])
        self.assertEqual(a["benefit"], "bvi_winners")
        self.assertIn("БВИ только победителю", a["note"])

    def test_score100_for_both(self):
        a = bs.aggregate_key([rec("pobeditel", "100_ballov"), rec("prizyor", "100_ballov")])
        self.assertEqual(a["benefit"], "score100")
        self.assertNotIn("только победителю", a["note"] or "")

    def test_score100_only_for_winner(self):
        a = bs.aggregate_key([rec("pobeditel", "100_ballov")])
        self.assertEqual(a["benefit"], "score100")
        self.assertIn("100 баллов только победителю", a["note"])

    def test_bvi_only_for_prize_winner_is_not_downgraded(self):
        # Неполная выгрузка правил: победитель не может получить меньше призёра.
        a = bs.aggregate_key([rec("prizyor", "БВИ")])
        self.assertEqual(a["benefit"], "bvi")

    def test_ege_min_is_minimum_of_non_null_with_range_in_note(self):
        a = bs.aggregate_key([rec("pobeditel", "БВИ", score=80), rec("prizyor", "БВИ", score=None),
                              rec("prizyor", "БВИ", score=75)])
        self.assertEqual(a["ege_min"], 75)
        self.assertIn("75–80", a["note"])

    def test_ege_min_null_when_nobody_states_it(self):
        a = bs.aggregate_key([rec("pobeditel", "БВИ", score=None, subject=None),
                              rec("prizyor", "БВИ", score=None, subject=None)])
        self.assertIsNone(a["ege_min"])
        self.assertIsNone(a["note"])

    def test_ege_subjects_listed_once_across_combined_records(self):
        # КФУ: у программ предмет подтверждения — «Математика», «Обществознание»
        # или сразу «Математика или Обществознание».
        a = bs.aggregate_key([rec("pobeditel", "БВИ", subject="Математика"),
                              rec("prizyor", "БВИ", subject="Математика или Обществознание"),
                              rec("prizyor", "БВИ", subject="Обществознание")])
        self.assertEqual(a["note"], "Подтвердить ЕГЭ: Математика или Обществознание")

    def test_diploma_grades_null_when_any_record_is_unrestricted(self):
        self.assertIsNone(bs.aggregate_key([rec("pobeditel", "БВИ", grades=[11]),
                                            rec("prizyor", "БВИ", grades=None)])["diploma_grades"])
        self.assertEqual(bs.aggregate_key([rec("pobeditel", "БВИ", grades=[11]),
                                           rec("prizyor", "БВИ", grades=[10, 11])])["diploma_grades"],
                         [10, 11])

    def test_diploma_grades_of_best_benefit_other_grades_in_note(self):
        # ВШЭ, Нижний Новгород: БВИ за диплом 10–11 класса, 100 баллов — за 9–11.
        # Девятикласснику строка не должна обещать БВИ (#78).
        a = bs.aggregate_key([rec(s, b, grades=g) for s in ("pobeditel", "prizyor")
                              for b, g in (("БВИ", [10, 11]), ("100_ballov", [9, 10, 11]))])
        self.assertEqual(a["benefit"], "bvi")
        self.assertEqual(a["diploma_grades"], [10, 11])
        self.assertIn("За диплом 9 класса — 100 баллов", a["note"])

    def test_winner_bvi_grades_prize_winner_grades_in_note(self):
        a = bs.aggregate_key([rec("pobeditel", "БВИ", grades=[11]),
                              rec("prizyor", "100_ballov", grades=[10, 11])])
        self.assertEqual(a["benefit"], "bvi_winners")
        self.assertEqual(a["diploma_grades"], [11])
        self.assertIn("За диплом 10 класса — 100 баллов", a["note"])

    def test_no_grade_note_when_weaker_grades_are_covered_or_unknown(self):
        for weaker in ([10, 11], None):
            a = bs.aggregate_key([rec("pobeditel", "БВИ", grades=[10, 11]),
                                  rec("prizyor", "100_ballov", grades=weaker)])
            self.assertEqual(a["diploma_grades"], [10, 11])
            self.assertNotIn("класса", a["note"])

    def test_any_demo_record_marks_key_as_demo(self):
        self.assertTrue(bs.aggregate_key([rec("pobeditel", "БВИ"),
                                          rec("prizyor", "БВИ", demo=True)])["demo"])

    def test_most_common_source_wins(self):
        a = bs.aggregate_key([rec("pobeditel", "БВИ", url="https://a"), rec("prizyor", "БВИ", url="https://b"),
                              rec("prizyor", "100_ballov", url="https://b")])
        self.assertEqual(a["src_url"], "https://b")


class LevelFilterTest(unittest.TestCase):
    def test_profile_must_be_at_least_as_strong_as_required(self):
        self.assertTrue(bs.level_ok("I", "II", False))
        self.assertTrue(bs.level_ok("II", "II", False))
        self.assertFalse(bs.level_ok("III", "II", False))
        self.assertFalse(bs.level_ok("I", "ВсОШ", False))
        self.assertTrue(bs.level_ok(None, "ВсОШ", True))


class StagesTest(unittest.TestCase):
    def test_fnv32_matches_reference(self):
        # Эталон FNV-1a 32: пустая строка и «a».
        self.assertEqual(bs.fnv32(""), 0x811C9DC5)
        self.assertEqual(bs.fnv32("a"), 0xE40C292C)

    def test_generated_stages_are_deterministic(self):
        first = bs.generated_stages("p669-2-matematika")
        self.assertEqual(first, bs.generated_stages("p669-2-matematika"))
        self.assertEqual([s["kind"] for s in first], ["registration", "qualifying", "final"])
        self.assertTrue(all(s["is_demo"] and s["source_id"] is None for s in first))
        reg = first[0]
        self.assertGreaterEqual(reg["starts_at"], "2026-10-01")
        self.assertLessEqual(reg["starts_at"], "2026-11-14 99")
        # Для регистрации важен последний день.
        self.assertEqual(reg["deadline_at"], reg["ends_at"])
        self.assertEqual(first[2]["deadline_at"], first[2]["starts_at"])

    def test_deadline_only_stage_has_no_start(self):
        # «Регистрация до 30 ноября», «финал не позднее 31 марта»: начала нет.
        reg = bs.stage("p", "registration", 1, None, "2026-11-30", True, False, "src")
        self.assertIsNone(reg["starts_at"])
        self.assertEqual(reg["deadline_at"], "2026-11-30 23:59:59+03")
        final = bs.stage("p", "final", 1, None, "2027-03-31", False, False, "src")
        self.assertIsNone(final["starts_at"])
        self.assertEqual(final["deadline_at"], final["ends_at"])

    def test_partial_calendar_gets_demo_final_a_month_after_last_stage(self):
        seed = bs.Seed()
        seed.olympiads["p669-11"] = {"name": "Сеченовская олимпиада"}
        p = {"id": "p669-11-biologiya", "olympiad_id": "p669-11", "_c": {
            "etapy_status": "partial_2026_27", "etapy_source_url": "https://olymp.example/",
            "etapy_source_page": None, "etapy_checked_at": "2026-09-23",
            "etapy": [
                {"stage_name": "Регистрация", "type": "registration",
                 "start_date": "2026-10-01", "end_date": "2026-10-29", "format": "online"},
                {"stage_name": "Отборочный этап", "type": "qualifying",
                 "start_date": "2026-11-10", "end_date": "2026-11-30", "format": "online"},
            ]}}
        st = bs.published_stages(seed, p)
        self.assertEqual([s["kind"] for s in st], ["registration", "qualifying", "final"])
        for s in st[:2]:
            self.assertFalse(s["is_demo"])
            self.assertEqual(seed.sources[s["source_id"]]["verified_at"], "2026-09-23")
        final = st[2]
        self.assertTrue(final["is_demo"])
        self.assertIsNone(final["source_id"])
        self.assertEqual(final["starts_at"], "2026-12-30 00:00:00+03")
        self.assertEqual(final["ends_at"], "2027-01-01 23:59:59+03")

    def test_vsosh_school_stage_from_sirius_schedule(self):
        # «Сириус.Курсы», график школьного этапа 2026/27: математика 7–11 класс —
        # 13–16 октября в зависимости от группы регионов, онлайн.
        seed = bs.Seed()
        st = bs.vsosh_stages(seed, "vsosh-matematika")
        self.assertEqual([s["kind"] for s in st], ["school", "municipal", "regional", "final"])
        school = st[0]
        self.assertEqual((school["starts_at"], school["ends_at"], school["is_online"], school["is_demo"]),
                         ("2026-10-13 00:00:00+03", "2026-10-16 23:59:59+03", True, False))
        self.assertEqual(seed.sources[school["source_id"]]["url"], "https://siriusolymp.ru/school2026/about")

    def test_vsosh_other_stages_are_deadlines_of_the_order(self):
        # Порядок проведения ВсОШ (vserosolimp.edsoo.ru): школьный этап — не
        # позднее 1 ноября, муниципальный — 25 декабря, региональный — 1 марта,
        # заключительный — до конца апреля. Экологии на «Сириусе» нет.
        seed = bs.Seed()
        st = {s["kind"]: s for s in bs.vsosh_stages(seed, "vsosh-ekologiya")}
        self.assertEqual({k: (s["starts_at"], s["deadline_at"]) for k, s in st.items()}, {
            "school": (None, "2026-11-01 23:59:59+03"),
            "municipal": (None, "2026-12-25 23:59:59+03"),
            "regional": (None, "2027-03-01 23:59:59+03"),
            "final": (None, "2027-04-30 23:59:59+03"),
        })
        self.assertTrue(all(not s["is_demo"] and s["source_id"] for s in st.values()))


class BuildTest(unittest.TestCase):
    """Прогон по настоящим датасетам."""

    @classmethod
    def setUpClass(cls):
        cls.seed = bs.build()
        cls.demo = bs.build_demo()

    def test_benefit_keys_match_dataset(self):
        # После исправления привязки льгот к программам (#68): ушли льготы,
        # приписанные чужим программам и взятые из перечней прошлых лет КФУ,
        # добавились ВсОШ СПбГУ, льготы ВШЭ в Перми и правила «любая олимпиада
        # уровня» СПбГУ. Уровень строже профиля — только Всесибирская по
        # информатике (II) на «Экономике» МГУ, где нужен I. КФУ даёт 100 баллов
        # там, где предмет олимпиады — ВИ, но не первое (приложение 3, стр. 1–2).
        # Затем сверка с документами по вузам: НГУ по кодам заголовка, ВсОШ
        # СПбГУ и МГУ со 100 баллами, строки Сеченова в несколько строк,
        # профили НТО, коды КФУ без профиля.
        self.assertEqual(self.seed.stats["benefit_keys"], 1462)
        self.assertEqual(self.seed.stats["benefit_level_filtered"], 2)
        self.assertEqual(self.seed.stats["benefit_unknown_profile"], 0)

    def test_programs_to_check_add_no_keys(self):
        B = bs.load("vuz_napravlenie_olimpiady.json", "vuz_napravlenie_olimpiady")
        offered, _ = bs.benefit_keys(B, self.seed.profiles)
        everything, _ = bs.benefit_keys(B, self.seed.profiles,
                                        statuses=("offered", "to_check", "not_offered"))
        self.assertEqual(set(offered), set(everything))

    def test_every_profile_is_seeded_including_missing_in_c(self):
        self.assertEqual(len(self.seed.profiles), 178 + 14)
        self.assertIn("p669-5-yadernye-tehnologii", self.seed.profiles)

    def test_subject_codes_are_the_frontend_ones(self):
        codes = {c for c, _ in bs.SUBJECTS}
        self.assertTrue({"inf", "math", "phys", "chem", "bio", "soc"} <= codes)
        self.assertTrue(all(p["subject_code"] in codes for p in self.seed.profiles.values()))
        for d in self.seed.directions:
            self.assertTrue(set(d["subject_codes"]) <= codes, d)

    def test_profile_slugs_unique_within_olympiad(self):
        pairs = Counter((p["olympiad_id"], p["profile_slug"]) for p in self.seed.profiles.values())
        self.assertEqual(pairs.most_common(1)[0][1], 1)

    def test_vsosh_profiles_have_four_stages_others_demo_or_sourced(self):
        by_profile = Counter(s["olympiad_profile_id"] for s in self.seed.stages)
        vsosh = [p for p in self.seed.profiles if p.startswith("vsosh-")]
        self.assertEqual(len(vsosh), 9)
        self.assertTrue(all(by_profile[p] == 4 for p in vsosh))
        for s in self.seed.stages:
            self.assertTrue(s["is_demo"] or s["source_id"], s["id"])
        # +1 к 23.09 — Московская олимпиада по информатике (26.09).
        self.assertEqual(self.seed.stats["stages_profiles_published"], 113)
        self.assertEqual(self.seed.stats["stages_profiles_none"], 12)

    def test_stage_ids_unique_and_windows_ordered(self):
        ids = [s["id"] for s in self.seed.stages]
        self.assertEqual(len(ids), len(set(ids)))
        for s in self.seed.stages:
            if s["starts_at"]:
                self.assertLessEqual(s["starts_at"], s["ends_at"], s["id"])

    def test_moscow_olympiad_informatics_tours_of_10_11_grades(self):
        # mos.olimpiada.ru/schedule, 26.09.2026: туры 10–11 классов — 14 и 28
        # февраля; финала в расписании нет — демо; регистрации в нём нет.
        by_id = {s["id"]: s for s in self.seed.stages}
        tours = [by_id[f"p669-37-informatika:qualifying:{n}"] for n in (1, 2)]
        self.assertEqual([t["starts_at"][:10] for t in tours], ["2027-02-14", "2027-02-28"])
        self.assertFalse(any(t["is_demo"] or t["is_online"] for t in tours))
        self.assertTrue(by_id["p669-37-informatika:final:1"]["is_demo"])
        self.assertNotIn("p669-37-informatika:registration:1", by_id)

    def test_calendars_found_on_2026_09_23(self):
        by_id = {s["id"]: s for s in self.seed.stages}
        # Финатлон: регистрация и отбор — до 30 ноября, финал объявлен.
        reg = by_id["p669-1-finansovaya-gramotnost:registration:1"]
        self.assertFalse(reg["is_demo"])
        self.assertIsNone(reg["starts_at"])
        self.assertEqual(reg["deadline_at"], "2026-11-30 23:59:59+03")
        self.assertFalse(by_id["p669-1-finansovaya-gramotnost:final:1"]["is_demo"])
        # Сеченовская: финал ещё не объявлен — он демо, остальное факт.
        bio = [(s["kind"], s["is_demo"]) for s in self.seed.stages
               if s["olympiad_profile_id"] == "p669-11-biologiya"]
        self.assertEqual(bio, [("registration", False), ("qualifying", False), ("final", True)])

    def test_demo_benefits_have_no_source(self):
        sourced = [b for b in self.seed.benefits if b["source_id"]]
        self.assertTrue(sourced)
        self.assertTrue(any(b["source_id"] is None for b in self.seed.benefits
                            if b["benefit"] != "extra_points"))
        for b in sourced:
            self.assertIn(b["source_id"], self.seed.sources)

    def test_other_olympiads_give_only_extra_points_within_ten(self):
        for seed in (self.seed, self.demo):
            for b in seed.benefits:
                kind = seed.olympiads[seed.profiles[b["olympiad_profile_id"]]["olympiad_id"]]["kind"]
                self.assertEqual(kind == "other", b["benefit"] == "extra_points", b["id"])
                if b["extra_points"] is not None:
                    self.assertLessEqual(b["extra_points"], 10)

    def test_fictional_olympiads_only_in_demo_seed(self):
        # В проде вымышленную олимпиаду приняли бы за настоящую (F16 — только локально).
        self.assertFalse([o for o in self.seed.olympiads.values() if o["kind"] == "other"])
        self.assertEqual({o["organizer"] for o in self.demo.olympiads.values()}, {bs.OTHER_ORGANIZER})
        self.assertTrue(self.demo.stages)

    def test_final_city_is_derived_only_from_single_university_organizers(self):
        o = self.seed.olympiads
        self.assertEqual((o["p669-8"]["final_city"], o["p669-8"]["final_region_code"]), ("Москва", "77"))
        self.assertEqual((o["p669-34"]["final_city"], o["p669-34"]["final_region_code"]), ("Казань", "16"))
        self.assertEqual(o["p669-22"]["final_city"], "Иннополис")
        # Оргкомитеты, министерство и консорциумы города не получают.
        for oid in ("vsosh-informatika", "p669-5", "p669-57", "p669-36"):
            self.assertIsNone(o[oid]["final_city"], oid)
        self.assertEqual(self.demo.olympiads["other-tyk"]["final_city"], "Казань")

    def test_organizer_cities_are_used_and_regions_exist(self):
        import json
        regions = {r["code"] for r in json.loads(
            (bs.REPO / "packages/core/refdata/regions.json").read_text(encoding="utf-8"))["regions"]}
        organizers = {o["organizer"] for o in self.seed.olympiads.values()}
        for org, (city, region) in bs.ORGANIZER_CITY.items():
            self.assertIn(org, organizers, "лишняя запись в ORGANIZER_CITY")
            self.assertIn(region, regions, org)
        for o in self.seed.olympiads.values():
            self.assertEqual(o["final_city"] is None, o["final_region_code"] is None, o["id"])

    def test_urls_are_ascii(self):
        urls = [s["url"] for s in self.seed.sources.values()]
        urls += [u["rules_url"] for u in self.seed.universities if u["rules_url"]]
        for u in urls:
            self.assertTrue(u.isascii(), u)

    def test_render_is_deterministic(self):
        self.assertEqual(bs.render(self.seed), bs.render(bs.build()))

    def test_committed_sql_is_up_to_date(self):
        self.assertEqual(bs.OUT.read_text(encoding="utf-8"), bs.render(self.seed),
                         "0003_seed_content.sql устарел — выполните make seed")


class ProgramGrantTest(unittest.TestCase):
    """Что программа даёт победителю и призёру: 2 — БВИ, 1 — 100 баллов, 0 — ничего."""

    def test_grant_per_diploma_status(self):
        self.assertEqual(bs.program_grant([rec("pobeditel", "БВИ"), rec("prizyor", "БВИ")]), (2, 2))
        self.assertEqual(bs.program_grant([rec("pobeditel", "БВИ"), rec("prizyor", "100_ballov")]), (2, 1))
        self.assertEqual(bs.program_grant([rec("pobeditel", "100_ballov")]), (1, 0))
        self.assertEqual(bs.program_grant([]), (0, 0))

    def test_winner_gets_at_least_what_prize_winner_gets(self):
        # Как в aggregate_key: неполная выгрузка не понижает победителя.
        self.assertEqual(bs.program_grant([rec("prizyor", "БВИ")]), (2, 2))


class ProgramLabelTest(unittest.TestCase):
    def test_title_drops_unbalanced_paren_and_extra_spaces(self):
        self.assertEqual(bs.program_title("Менеджмент — Менеджмент в культуре)"),
                         "Менеджмент — Менеджмент в культуре")
        self.assertEqual(bs.program_title("Управление инновациями (сетевая программа МШУ «СКОЛКОВО» )"),
                         "Управление инновациями (сетевая программа МШУ «СКОЛКОВО»)")
        self.assertEqual(bs.program_title("Биотехнология (ФБМФ)"), "Биотехнология (ФБМФ)")

    def test_title_joins_words_broken_by_pdf_line_wrap(self):
        # В таблицах ВШЭ слово переносится по слогам: дефис переноса уходит,
        # у «бизнес-» дефис свой; «микро- и нано…» — не перенос.
        for raw, want in [
            ("Информацион- ная безопасность", "Информационная безопасность"),
            ("Цифровые технологии и телеком- муникации", "Цифровые технологии и телекоммуникации"),
            ("Бизнес- информатика", "Бизнес-информатика"),
            ("Иностранные языки и межкультурная бизнес- коммуникация",
             "Иностранные языки и межкультурная бизнес-коммуникация"),
            ("Физика перспективных технологий: микро- и наноэлектроника",
             "Физика перспективных технологий: микро- и наноэлектроника"),
        ]:
            self.assertEqual(bs.program_title(raw), want)

    def test_title_joins_letter_split_off_a_word(self):
        self.assertEqual(bs.program_title("Дизайн и разработка информационны х продуктов"),
                         "Дизайн и разработка информационных продуктов")
        self.assertEqual(bs.program_title("Экономика и анализ данных"), "Экономика и анализ данных")
        self.assertEqual(bs.program_title("Физика с углублённой математикой"),
                         "Физика с углублённой математикой")

    def test_title_drops_trailing_semicolon(self):
        self.assertEqual(bs.program_title("Бухгалтерский учет, анализ и аудит; Финансы и кредит;"),
                         "Бухгалтерский учет, анализ и аудит; Финансы и кредит")

    def test_same_name_in_pair_is_told_apart_by_faculty(self):
        labels = bs.program_labels([
            {"program_id": "a", "program_name": "Физика", "faculty": "НИУ ВШЭ — Москва"},
            {"program_id": "b", "program_name": "Физика", "faculty": "НИУ ВШЭ — Санкт-Петербург"},
            {"program_id": "c", "program_name": "Прикладная физика", "faculty": "Физический факультет"},
        ])
        self.assertEqual(labels, {"a": ("Физика", "НИУ ВШЭ — Москва"),
                                  "b": ("Физика", "НИУ ВШЭ — Санкт-Петербург"),
                                  "c": ("Прикладная физика", None)})

    def test_faculty_made_of_codes_is_not_a_label(self):
        # У КФУ в поле факультета — перечень укрупнённых групп.
        labels = bs.program_labels([
            {"program_id": "a", "program_name": "ПМИ", "faculty": "01.02.02 Прикладная математика"},
            {"program_id": "b", "program_name": "ПМИ", "faculty": "09.00.00 Информатика"},
        ])
        self.assertEqual(labels, {"a": ("ПМИ", None), "b": ("ПМИ", None)})


class VariesNoteTest(unittest.TestCase):
    """Пояснение «Зависит от программы» у льготы по направлению."""

    def test_same_grant_everywhere_is_not_varies(self):
        self.assertIsNone(bs.varies_note([(("A", None), (2, 2)), (("B", None), (2, 2))]))

    def test_program_without_benefit(self):
        self.assertEqual(bs.varies_note([(("A", None), (2, 2)), (("B", None), (0, 0))]),
                         "Зависит от программы: на «B» льготы нет")

    def test_few_programs_with_benefit_are_named_instead(self):
        note = bs.varies_note([(("A", None), (2, 2)), (("B", None), (0, 0)),
                               (("C", None), (0, 0))])
        self.assertEqual(note, "Зависит от программы: льгота только на «A»")

    def test_other_kind_of_benefit(self):
        self.assertEqual(bs.varies_note([(("A", None), (2, 2)), (("B", None), (1, 1))]),
                         "Зависит от программы: на «B» — 100 баллов")

    def test_all_programs_listed_when_none_matches_headline(self):
        # Сводная льгота — БВИ победителю и 100 баллов призёру — не совпадает
        # ни с одной программой.
        note = bs.varies_note([(("A", None), (2, 0)), (("B", None), (1, 1)), (("C", None), (0, 0))])
        self.assertEqual(note, "Зависит от программы: на «A» — БВИ только победителю, "
                               "на «B» — 100 баллов, на «C» льготы нет")

    def test_prize_winner_difference_counts(self):
        self.assertEqual(bs.varies_note([(("A", None), (1, 1)), (("B", None), (1, 0))]),
                         "Зависит от программы: на «B» — 100 баллов только победителю")

    def test_long_lists_are_cut(self):
        grants = [(("A", None), (2, 2)), (("B", None), (2, 2)), (("C", None), (2, 2)),
                  (("D", None), (2, 2)), (("E", None), (2, 2)), (("F", None), (0, 0)),
                  (("G", None), (0, 0)), (("H", None), (0, 0)), (("I", None), (0, 0))]
        self.assertEqual(bs.varies_note(grants),
                         "Зависит от программы: на «F», «G» и ещё 2 программах льготы нет")
        self.assertEqual(bs.varies_note(grants[:3] + grants[5:]),
                         "Зависит от программы: льгота только на «A», «B» и «C»")

    def test_faculty_and_nested_quotes(self):
        note = bs.varies_note([
            (("Физика", "НИУ ВШЭ — Москва"), (2, 2)),
            (("Физика (с дополнительной квалификацией «Программист»)", "НИУ ВШЭ — Пермь"), (0, 0)),
        ])
        self.assertEqual(note, "Зависит от программы: на «Физика (с дополнительной "
                               "квалификацией „Программист“)» (НИУ ВШЭ — Пермь) льготы нет")


class UniversityDirectionsTest(unittest.TestCase):
    """Сид 0021 по настоящим датасетам: направления вузов и льготы по ним."""

    @classmethod
    def setUpClass(cls):
        cls.seed = bs.build()
        cls.ud = bs.build_university_directions(cls.seed)
        cls.dirs = {d["id"]: d for d in cls.ud.directions}
        cls.pairs = {(p["university_id"], p["direction_id"]): p for p in cls.ud.pairs}

    def test_every_code_of_pairs_is_a_direction(self):
        self.assertEqual(len(self.ud.directions), 72)
        self.assertEqual({p["direction_id"] for p in self.ud.pairs}, set(self.dirs))
        self.assertEqual(sum(d["onboarding"] for d in self.ud.directions), 16)

    def test_onboarding_directions_keep_seed_name_and_subjects(self):
        for d in self.seed.directions:
            got = self.dirs[d["id"]]
            self.assertTrue(got["onboarding"], d["id"])
            self.assertEqual((got["name"], got["subject_codes"]), (d["name"], d["subject_codes"]))
        self.assertEqual(self.dirs["napr-09-03-04"]["groups"], ["Биомед", "ИТ", "Экономика"])

    def test_new_directions_get_group_subjects(self):
        d = self.dirs["napr-01-03-01"]
        self.assertEqual((d["name"], d["groups"], d["subject_codes"], d["onboarding"]),
                         ("Математика", ["ИТ", "Экономика"], ["inf", "math", "econ", "soc"], False))
        innopolis = self.dirs["napr-09-00-00"]
        self.assertEqual((innopolis["name"], innopolis["groups"]),
                         ("Информатика и вычислительная техника", ["ИТ"]))
        codes = {c for c, _ in bs.SUBJECTS}
        for d in self.ud.directions:
            self.assertTrue(d["groups"], d["id"])
            self.assertEqual(d["groups"], sorted(d["groups"]), d["id"])
            self.assertTrue(set(d["subject_codes"]) <= codes, d["id"])

    def test_pairs_and_statuses(self):
        self.assertEqual(len(self.ud.pairs), 173)
        to_check = sorted(k for k, p in self.pairs.items() if p["status"] == "to_check")
        # СПбГУ 06.03.01 получил свою ВсОШ (bac_spec_olymp_1), НГУ 01.03.01–03 —
        # льготы заголовка «Математика и механика». Пара ВШЭ — 40.03.01 держалась
        # на очно-заочной программе, её в A больше нет (#79).
        self.assertEqual(to_check, [])
        self.assertNotIn(("hse", "napr-40-03-01"), self.pairs)

    def test_program_in_two_groups_is_counted_once(self):
        # В A программа повторяется на каждую профильную группу.
        kfu = self.pairs[("kfu", "napr-38-03-05")]
        self.assertEqual((kfu["programs"], kfu["budget_places"], kfu["program_names"]),
                         (2, 23, ["Бизнес-информатика", "Цифровое предприятие"]))

    def test_places_of_competition_group_are_counted_once(self):
        # МФТИ публикует места на конкурсную группу: у пяти программ ФПМИ на
        # 01.03.02 одно число 180 — и это места всего направления (план
        # приёма 2026, строка «01.03.02 … 180»), а не 5 × 180.
        for code, places in (("01-03-02", 180), ("03-03-01", 517), ("09-03-01", 196), ("19-03-01", 66)):
            self.assertEqual(self.pairs[("mipt", "napr-" + code)]["budget_places"], places, code)

    def test_places_unknown_stay_null(self):
        hse = self.pairs[("hse", "napr-38-03-01")]
        self.assertEqual((hse["status"], hse["programs"], hse["budget_places"]), ("offered", 10, None))
        # «Экономика и бизнес» в Нижнем — очно-заочная, в A её нет (#79).
        self.assertNotIn("Экономика и бизнес", hse["program_names"])

    def test_branches_are_named_by_faculty(self):
        msu = self.pairs[("msu", "napr-01-03-02")]
        self.assertEqual((msu["programs"], msu["budget_places"]), (4, 367))
        self.assertIn("Прикладная математика и информатика (Филиал МГУ в г. Грозном)",
                      msu["program_names"])

    def test_direction_benefits_refine_university_benefits(self):
        self.assertEqual(len(self.ud.benefits), 12769)
        keys = {(b["olympiad_profile_id"], b["university_id"], b["admission_year"])
                for b in self.ud.benefits}
        self.assertEqual(keys, {(b["olympiad_profile_id"], b["university_id"], b["admission_year"])
                                for b in self.seed.benefits})
        for b in self.ud.benefits:
            self.assertIn(b["benefit"], ("bvi", "bvi_winners", "score100"))
            self.assertIn((b["university_id"], b["direction_id"]), self.pairs)
            if b["source_id"]:
                self.assertIn(b["source_id"], self.ud.sources)

    def test_varies_rows_explain_themselves(self):
        varies = [b for b in self.ud.benefits if b["varies"]]
        self.assertEqual(len(varies), 1631)
        for b in self.ud.benefits:
            self.assertEqual(b["varies"], (b["note"] or "").startswith("Зависит от программы: "),
                             (b["olympiad_profile_id"], b["university_id"], b["direction_id"]))
        # ВШЭ, 01.03.01: Всесибирская по физике — БВИ на «Фундаментальной и
        # прикладной математике» (Нижний Новгород, стр. 8) и 100 баллов
        # победителю на «Математике» (Москва, стр. 1).
        b = next(b for b in self.ud.benefits if (b["olympiad_profile_id"], b["university_id"],
                                                  b["direction_id"]) ==
                 ("p669-14-fizika", "hse", "napr-01-03-01"))
        self.assertEqual(b["benefit"], "bvi")
        self.assertTrue(b["note"].startswith(
            "Зависит от программы: на «Математика» — 100 баллов только победителю"), b["note"])

    def test_down_removes_only_sources_of_its_own(self):
        own = {k for k in self.ud.sources if k not in self.seed.sources}
        self.assertTrue(own)
        down = bs.render_university_directions(self.ud).split("-- +goose Down")[1]
        self.assertEqual(set(re.findall(r"src-[0-9a-f]{12}", down)), own)

    def test_render_is_deterministic(self):
        self.assertEqual(bs.render_university_directions(self.ud),
                         bs.render_university_directions(bs.build_university_directions(bs.build())))

    def test_committed_sql_is_up_to_date(self):
        self.assertEqual(bs.UD_OUT.read_text(encoding="utf-8"),
                         bs.render_university_directions(self.ud),
                         "0021_university_directions_content.sql устарел — выполните make seed")


class SqlLiteralTest(unittest.TestCase):
    def test_quotes_are_escaped(self):
        self.assertEqual(bs.lit("д'Артаньян"), "'д''Артаньян'")
        self.assertEqual(bs.lit(None), "NULL")
        self.assertEqual(bs.lit([10, 11]), "ARRAY[10,11]")
        self.assertEqual(bs.lit([]), "'{}'")
        self.assertEqual(bs.lit(True), "true")


if __name__ == "__main__":
    unittest.main()
