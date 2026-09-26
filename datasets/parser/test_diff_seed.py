"""Миграция-разница сидов: что удалить, что вставить, и обратно."""
import unittest

import diff_seed as ds

OLD = """-- +goose Up
INSERT INTO sources (id, kind, title, url, verified_at) VALUES
  ('src-a', 'rules', 'А', 'https://a', '2026-09-21'::date),
  ('src-b', 'rules', 'Б', 'https://b', '2026-09-21'::date)
ON CONFLICT (id) DO UPDATE SET
  kind = EXCLUDED.kind;

INSERT INTO benefits (id, olympiad_profile_id, university_id, admission_year, benefit, extra_points, ege_min, diploma_grades, note, source_id) VALUES
  ('p1__msu__2026__bvi', 'p1', 'msu', 2026, 'bvi', NULL, 75, NULL, 'Подтвердить ЕГЭ: Физика', 'src-a'),
  ('p2__msu__2026__bvi', 'p2', 'msu', 2026, 'bvi', NULL, 75, ARRAY[10,11]::int[], 'Л''Эколь, «А, Б»', 'src-a'),
  ('p3__msu__2026__bvi', 'p3', 'msu', 2026, 'bvi', NULL, 75, NULL, NULL, 'src-b')
ON CONFLICT (id) DO UPDATE SET
  benefit = EXCLUDED.benefit;

-- +goose Down
DELETE FROM benefits;
"""

NEW = """-- +goose Up
INSERT INTO sources (id, kind, title, url, verified_at) VALUES
  ('src-a', 'rules', 'А', 'https://a', '2026-09-21'::date),
  ('src-c', 'rules', 'В', 'https://c', '2026-09-25'::date)
ON CONFLICT (id) DO UPDATE SET
  kind = EXCLUDED.kind;

INSERT INTO benefits (id, olympiad_profile_id, university_id, admission_year, benefit, extra_points, ege_min, diploma_grades, note, source_id) VALUES
  ('p1__msu__2026__bvi', 'p1', 'msu', 2026, 'bvi', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-a'),
  ('p2__msu__2026__bvi', 'p2', 'msu', 2026, 'bvi', NULL, 75, ARRAY[10,11]::int[], 'Л''Эколь, «А, Б»', 'src-a'),
  ('p4__msu__2026__score100', 'p4', 'msu', 2026, 'score100', NULL, 75, NULL, NULL, 'src-c')
ON CONFLICT (id) DO UPDATE SET
  benefit = EXCLUDED.benefit;
"""

DIR_OLD = """INSERT INTO direction_benefits (olympiad_profile_id, university_id, direction_id, admission_year, benefit, ege_min, diploma_grades, note, varies, source_id)
SELECT v.olympiad_profile_id, v.university_id
FROM (VALUES
  ('p1', 'msu', 'napr-01-03-02', 2026, 'bvi', 75, NULL, NULL, false, 'src-a'),
  ('p2', 'msu', 'napr-01-03-02', 2026, 'bvi', 75, NULL, NULL, false, 'src-a')
) AS v(olympiad_profile_id, university_id, direction_id, admission_year, benefit, ege_min, diploma_grades, note, varies, source_id)
ON CONFLICT (olympiad_profile_id, university_id, direction_id, admission_year) DO UPDATE SET
  benefit = EXCLUDED.benefit;
"""

DIR_NEW = """INSERT INTO direction_benefits (olympiad_profile_id, university_id, direction_id, admission_year, benefit, ege_min, diploma_grades, note, varies, source_id)
SELECT v.olympiad_profile_id, v.university_id
FROM (VALUES
  ('p1', 'msu', 'napr-01-03-02', 2026, 'bvi', 75, NULL, NULL, false, 'src-a')
) AS v(olympiad_profile_id, university_id, direction_id, admission_year, benefit, ege_min, diploma_grades, note, varies, source_id)
ON CONFLICT (olympiad_profile_id, university_id, direction_id, admission_year) DO UPDATE SET
  benefit = EXCLUDED.benefit;
"""


class ParseTest(unittest.TestCase):
    def test_rows_by_key_in_values_form(self):
        rows = ds.rows(OLD, "benefits")
        self.assertEqual(list(rows), ["p1__msu__2026__bvi", "p2__msu__2026__bvi", "p3__msu__2026__bvi"])
        self.assertEqual(rows["p2__msu__2026__bvi"],
                         "('p2__msu__2026__bvi', 'p2', 'msu', 2026, 'bvi', NULL, 75, ARRAY[10,11]::int[], "
                         "'Л''Эколь, «А, Б»', 'src-a')")

    def test_rows_by_composite_key_in_select_form(self):
        self.assertEqual(list(ds.rows(DIR_OLD, "direction_benefits")),
                         [("p1", "msu", "napr-01-03-02", "2026"), ("p2", "msu", "napr-01-03-02", "2026")])

    def test_fields_respect_quotes_and_arrays(self):
        self.assertEqual(ds.fields("('a, b', ARRAY[1,2]::int[], 'it''s', NULL)"),
                         ["'a, b'", "ARRAY[1,2]::int[]", "'it''s'", "NULL"])

    def test_only_up_section(self):
        self.assertEqual(ds.up_section(OLD).count("DELETE"), 0)


class DiffTest(unittest.TestCase):
    def test_changed_added_removed(self):
        d = ds.diff(ds.rows(OLD, "benefits"), ds.rows(NEW, "benefits"))
        self.assertEqual(d.removed, ["p3__msu__2026__bvi"])
        self.assertEqual(list(d.upsert), ["p1__msu__2026__bvi", "p4__msu__2026__score100"])

    def test_touched_rows_always_brought_to_new_seed(self):
        # 0017/0018 на чистой базе перезаписывают строки новой 0003 — их надо
        # вернуть к сиду, даже если между master и веткой они не менялись.
        d = ds.diff(ds.rows(OLD, "benefits"), ds.rows(NEW, "benefits"),
                    touched={"p2__msu__2026__bvi", "p9__msu__2026__bvi"})
        self.assertEqual(list(d.upsert), ["p1__msu__2026__bvi", "p2__msu__2026__bvi", "p4__msu__2026__score100"])
        self.assertEqual(d.removed, ["p3__msu__2026__bvi", "p9__msu__2026__bvi"])


class RenderTest(unittest.TestCase):
    def setUp(self):
        self.sql = ds.render(old=[OLD, DIR_OLD], new=[NEW, DIR_NEW], note="-- Проба.", touched={})
        self.up, self.down = self.sql.split("-- +goose Down")

    def test_up(self):
        self.assertIn("DELETE FROM benefits WHERE id IN (\n  'p3__msu__2026__bvi'\n);", self.up)
        self.assertIn("('p4__msu__2026__score100',", self.up)
        self.assertNotIn("('p2__msu__2026__bvi',", self.up)
        self.assertIn("('src-c', 'rules'", self.up)
        self.assertIn("  ('p2', 'msu', 'napr-01-03-02', 2026)\n) AS v(olympiad_profile_id", self.up)

    def test_down_restores_old(self):
        self.assertIn("DELETE FROM benefits WHERE id IN (\n  'p4__msu__2026__score100'\n);", self.down)
        self.assertIn("('p3__msu__2026__bvi',", self.down)
        self.assertIn("('p1__msu__2026__bvi', 'p1', 'msu', 2026, 'bvi', NULL, 75, NULL,", self.down)
        self.assertIn("('p2', 'msu', 'napr-01-03-02', 2026, 'bvi'", self.down)
        # источник, которого в прежнем сиде не было, убирается, если на него не ссылаются
        self.assertIn("'src-c'", self.down.split("DELETE FROM sources")[1])

    def test_up_drops_sources_gone_from_seed(self):
        # src-b был у прежних льгот; в новом сиде его нет — уходит, если на
        # него больше ничто не ссылается. Down вернёт его вместе с льготами.
        gone = self.up.split("DELETE FROM sources s WHERE s.id IN (")[1].split(";")[0]
        self.assertIn("'src-b'", gone)
        self.assertNotIn("'src-a'", gone)
        self.assertIn("NOT EXISTS (SELECT 1 FROM direction_benefits d WHERE d.source_id = s.id)", gone)
        self.assertIn("('src-b', 'rules'", self.down)

    def test_counts_in_header(self):
        self.assertTrue(self.sql.startswith("-- Проба.\n"))
        self.assertIn("-- Льготы по вузам: удаляется 1, добавляется или меняется 2.", self.sql)
        self.assertIn("-- Льготы по направлениям: удаляется 1, добавляется или меняется 0.", self.sql)


STAGES_OLD = """INSERT INTO stages (id, olympiad_profile_id, kind, title, starts_at, ends_at, deadline_at, is_online, is_demo, source_id) VALUES
  ('v:school:1', 'v', 'school', 'Школьный этап', '2026-09-20 00:00:00+03'::timestamptz, '2026-10-25 23:59:59+03'::timestamptz, '2026-09-20 00:00:00+03'::timestamptz, false, true, NULL),
  ('p:final:1', 'p', 'final', 'Финал', NULL, '2027-03-01 23:59:59+03'::timestamptz, '2027-03-01 23:59:59+03'::timestamptz, false, false, 'src-a')
ON CONFLICT (id) DO UPDATE SET
  is_demo = EXCLUDED.is_demo,
  source_id = EXCLUDED.source_id;
"""

STAGES_NEW = STAGES_OLD.replace(
    "'2026-09-20 00:00:00+03'::timestamptz, '2026-10-25 23:59:59+03'::timestamptz, '2026-09-20 00:00:00+03'::timestamptz, false, true, NULL",
    "'2026-10-13 00:00:00+03'::timestamptz, '2026-10-16 23:59:59+03'::timestamptz, '2026-10-13 00:00:00+03'::timestamptz, true, false, 'src-d'")

SRC_D = "  ('src-d', 'site', 'Г', 'https://d', '2026-09-26'::date),\n"


class StagesTest(unittest.TestCase):
    """Сроки этапов — своей миграцией: этапы и источники, на которые они ссылаются."""

    def setUp(self):
        new_seed = NEW.replace("  ('src-c',", SRC_D + "  ('src-c',")
        old_seed = OLD.replace("-- +goose Down", STAGES_OLD + "\n-- +goose Down")
        self.sql = ds.render_stages(old=[old_seed], new=[new_seed + STAGES_NEW], note="-- Сроки.")
        self.up, self.down = self.sql.split("-- +goose Down")

    def test_up_changes_stage_and_adds_its_source(self):
        self.assertIn("('v:school:1', 'v', 'school', 'Школьный этап', '2026-10-13", self.up)
        self.assertNotIn("'p:final:1'", self.up)
        self.assertIn("('src-d', 'site'", self.up)
        # Источники льгот — дело миграции льгот.
        self.assertNotIn("'src-c'", self.up)

    def test_down_restores_stage_and_drops_new_source(self):
        self.assertIn("'2026-09-20 00:00:00+03'::timestamptz, '2026-10-25 23:59:59+03'::timestamptz", self.down)
        self.assertIn("'src-d'", self.down.split("DELETE FROM sources")[1])

    def test_counts_in_header(self):
        self.assertIn("-- Этапы: удаляется 0, добавляется или меняется 1. Источников новых 1.", self.sql)

    def test_benefit_migration_skips_sources_of_stages_only(self):
        new_seed = NEW.replace("  ('src-c',", SRC_D + "  ('src-c',")
        old_seed = OLD.replace("-- +goose Down", STAGES_OLD + "\n-- +goose Down")
        sql = ds.render(old=[old_seed, DIR_OLD], new=[new_seed + STAGES_NEW, DIR_NEW], note="-- Проба.", touched={})
        self.assertNotIn("'src-d'", sql)


if __name__ == "__main__":
    unittest.main()
