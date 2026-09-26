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


if __name__ == "__main__":
    unittest.main()
