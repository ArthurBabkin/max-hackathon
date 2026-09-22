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

    def test_vsosh_has_four_stages_in_order(self):
        st = bs.vsosh_stages("vsosh-fizika")
        self.assertEqual([s["kind"] for s in st], ["school", "municipal", "regional", "final"])
        self.assertEqual(sorted(s["starts_at"] for s in st), [s["starts_at"] for s in st])


class BuildTest(unittest.TestCase):
    """Прогон по настоящим датасетам."""

    @classmethod
    def setUpClass(cls):
        cls.seed = bs.build()

    def test_benefit_keys_match_dataset(self):
        self.assertEqual(self.seed.stats["benefit_keys"], 1149)
        self.assertEqual(self.seed.stats["benefit_level_filtered"], 91)
        self.assertEqual(self.seed.stats["benefit_unknown_profile"], 0)

    def test_programs_to_check_add_no_keys(self):
        B = bs.load("vuz_napravlenie_olimpiady.json", "vuz_napravlenie_olimpiady")
        offered, _ = bs.benefit_keys(B, self.seed.profiles)
        everything, _ = bs.benefit_keys(B, self.seed.profiles,
                                        statuses=("offered", "to_check", "not_offered"))
        self.assertEqual(set(offered), set(everything))

    def test_every_profile_is_seeded_including_missing_in_c(self):
        self.assertEqual(len(self.seed.profiles), 178 + 14 + len(bs.OTHER_OLYMPIADS))
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
        self.assertEqual(self.seed.stats["stages_profiles_published"], 50)
        self.assertEqual(self.seed.stats["stages_profiles_none"], 12)

    def test_stage_ids_unique_and_windows_ordered(self):
        ids = [s["id"] for s in self.seed.stages]
        self.assertEqual(len(ids), len(set(ids)))
        for s in self.seed.stages:
            self.assertLessEqual(s["starts_at"], s["ends_at"], s["id"])

    def test_demo_benefits_have_no_source(self):
        sourced = [b for b in self.seed.benefits if b["source_id"]]
        self.assertTrue(sourced)
        self.assertTrue(any(b["source_id"] is None for b in self.seed.benefits
                            if b["benefit"] != "extra_points"))
        for b in sourced:
            self.assertIn(b["source_id"], self.seed.sources)

    def test_other_olympiads_give_only_extra_points_within_ten(self):
        for b in self.seed.benefits:
            kind = self.seed.olympiads[self.seed.profiles[b["olympiad_profile_id"]]["olympiad_id"]]["kind"]
            self.assertEqual(kind == "other", b["benefit"] == "extra_points", b["id"])
            if b["extra_points"] is not None:
                self.assertLessEqual(b["extra_points"], 10)

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
