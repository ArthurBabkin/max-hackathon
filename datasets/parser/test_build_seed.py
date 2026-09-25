"""Инварианты генератора сида. Запуск: make test-db или
python3 -m unittest discover -s datasets/parser -p 'test_*.py'."""
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

    def test_diploma_grades_null_when_any_record_is_unrestricted(self):
        self.assertIsNone(bs.aggregate_key([rec("pobeditel", "БВИ", grades=[11]),
                                            rec("prizyor", "БВИ", grades=None)])["diploma_grades"])
        self.assertEqual(bs.aggregate_key([rec("pobeditel", "БВИ", grades=[11]),
                                           rec("prizyor", "БВИ", grades=[10, 11])])["diploma_grades"],
                         [10, 11])

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

    def test_vsosh_has_four_stages_in_order(self):
        st = bs.vsosh_stages("vsosh-fizika")
        self.assertEqual([s["kind"] for s in st], ["school", "municipal", "regional", "final"])
        self.assertEqual(sorted(s["starts_at"] for s in st), [s["starts_at"] for s in st])


class BuildTest(unittest.TestCase):
    """Прогон по настоящим датасетам."""

    @classmethod
    def setUpClass(cls):
        cls.seed = bs.build()
        cls.demo = bs.build_demo()

    def test_benefit_keys_match_dataset(self):
        # 1149 + 34 льготы Иннополиса со стр. 8, 9, 11, 16 его приказа.
        self.assertEqual(self.seed.stats["benefit_keys"], 1183)
        self.assertEqual(self.seed.stats["benefit_level_filtered"], 91)
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
        self.assertEqual(self.seed.stats["stages_profiles_published"], 112)
        self.assertEqual(self.seed.stats["stages_profiles_none"], 12)

    def test_stage_ids_unique_and_windows_ordered(self):
        ids = [s["id"] for s in self.seed.stages]
        self.assertEqual(len(ids), len(set(ids)))
        for s in self.seed.stages:
            if s["starts_at"]:
                self.assertLessEqual(s["starts_at"], s["ends_at"], s["id"])

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


class SqlLiteralTest(unittest.TestCase):
    def test_quotes_are_escaped(self):
        self.assertEqual(bs.lit("д'Артаньян"), "'д''Артаньян'")
        self.assertEqual(bs.lit(None), "NULL")
        self.assertEqual(bs.lit([10, 11]), "ARRAY[10,11]")
        self.assertEqual(bs.lit([]), "'{}'")
        self.assertEqual(bs.lit(True), "true")


if __name__ == "__main__":
    unittest.main()
