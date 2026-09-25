"""Чек-лист спеки и эталонные льготы на закоммиченных датасетах — в CI.
Запуск: python3 -m unittest discover -s datasets/parser -p 'test_*.py'."""
import contextlib
import io
import unittest

import validate


class DatasetsTest(unittest.TestCase):
    def test_checklist_and_golden_benefits(self):
        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            code = validate.main()
        self.assertEqual(code, 0, out.getvalue())


if __name__ == "__main__":
    unittest.main()
