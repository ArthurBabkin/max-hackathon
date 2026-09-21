#!/usr/bin/env python3
"""Название олимпиады на сайте вуза -> номер в перечне №669 -> olympiad_id.

Вузы пишут названия по-разному: с кавычками-ёлочками и без, с «Всероссийская
олимпиада школьников» в начале и без, с римскими номерами выпусков.
Транслитерировать название в id нельзя (п. 4 спеки) — только через номер перечня,
иначе id в B и C разойдутся.
"""
import json, re, sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))
from common import DATA, clean

_index = json.loads((DATA / "olympiad_index_669.json").read_text(encoding="utf-8"))["olympiads"]
_slugs = {s["profile"]: s["slug"] for s in
          json.loads((DATA / "subject_slugs.json").read_text(encoding="utf-8"))["slugs"]}

NOISE = re.compile(
    r"^(всероссийск\w+|открыт\w+|международн\w+|межрегиональн\w+|многопрофильн\w+|объединённ\w+|объединенн\w+)?\s*"
    r"(олимпиад\w*\s+школьников|олимпиад\w*|конкурс)?\s*", re.I)
ROMAN = re.compile(r"\b[IVXL]{1,6}\b")


def norm(name: str) -> str:
    s = clean(name).lower().replace("ё", "е")
    s = ROMAN.sub(" ", s)
    s = re.sub(r"[«»\"'`,.:;()\[\]–—\-]", " ", s)
    s = re.sub(r"\bим(ени)?\b", " ", s)
    s = re.sub(r"\b\d{1,3}\s*(я|ая|й|ый|е)?\b", " ", s)
    stripped = re.sub(r"\s+", " ", NOISE.sub("", s)).strip()
    # У родовых названий («Открытая олимпиада школьников») чистка съедает всё —
    # тогда нормализуем мягко, иначе олимпиада не находится вообще.
    return stripped if stripped else re.sub(r"\s+", " ", s).strip()


# Готовим индекс: ключ -> номер. Один номер может иметь несколько написаний.
_by_norm: dict[str, int] = {}
_names: dict[int, str] = {}
for row in _index:
    _names[row["perechen_number_669"]] = row["name"]
    _by_norm.setdefault(norm(row["name"]), row["perechen_number_669"])
_norm_items = sorted(_by_norm.items(), key=lambda kv: -len(kv[0]))

# Профили, объявленные вузом в перечне для каждой олимпиады — для валидации.
_profiles: dict[int, dict[str, str]] = {}
for row in _index:
    _profiles.setdefault(row["perechen_number_669"], {})[norm(row["profile"])] = row["level"]


def match_number(name: str) -> int | None:
    """-> номер олимпиады в перечне №669, либо None."""
    n = norm(name)
    if not n:
        return None
    if n in _by_norm:
        return _by_norm[n]
    for key, num in _norm_items:          # вхождение: вуз мог обрезать или дополнить название
        if len(key) >= 8 and (key in n or n in key):
            return num
    tokens = set(n.split())
    best, score = None, 0.0
    for key, num in _norm_items:          # запасной путь — перекрытие значимых слов
        kt = set(key.split())
        if not kt:
            continue
        overlap = len(tokens & kt) / len(kt)
        if overlap > score:
            best, score = num, overlap
    return best if score >= 0.75 else None


_slugs_nospace = {re.sub(r"\s+", "", k): v for k, v in _slugs.items()}


def profile_slug(profile: str) -> str | None:
    """PDF-экстракция рвёт слова на границе колонки («Математи ка»), поэтому
    при промахе сравниваем без пробелов вовсе."""
    key = clean(profile).lower()
    if key in _slugs:
        return _slugs[key]
    return _slugs_nospace.get(re.sub(r"\s+", "", key))


def match_number_for_profile(name: str, profile: str) -> int | None:
    """Номер олимпиады, у которой в перечне есть ИМЕННО этот профиль.

    Без этой проверки похожие названия склеиваются и рождаются несуществующие
    пары «олимпиада №41 + профиль, которого у неё нет». Спека запрещает выдавать
    предположение за факт, поэтому при отсутствии пары лучше вернуть None.
    """
    want = norm(profile)
    if not want:
        return None
    candidates = [num for num, profs in _profiles.items() if want in profs]
    if not candidates:
        return None
    n = norm(name)
    if not n:
        return candidates[0] if len(candidates) == 1 else None
    for num in candidates:                       # точное совпадение названия
        if norm(_names[num]) == n:
            return num
    hits = [num for num in candidates
            if len(norm(_names[num])) >= 8 and (norm(_names[num]) in n or n in norm(_names[num]))]
    if len(hits) == 1:
        return hits[0]
    tokens = set(n.split())
    best, score = None, 0.0
    for num in candidates:
        kt = set(norm(_names[num]).split())
        if not kt:
            continue
        overlap = len(tokens & kt) / len(kt)
        if overlap > score:
            best, score = num, overlap
    if score >= 0.6:
        return best
    return candidates[0] if len(candidates) == 1 else None


def olympiad_id(name: str, profile: str) -> tuple[str | None, int | None, str | None]:
    """-> (olympiad_id, номер перечня, официальное название). ВсОШ обрабатывается отдельно."""
    num = match_number(name)
    slug = profile_slug(profile)
    if num is None or slug is None:
        return None, num, _names.get(num) if num else None
    return f"p669-{num}-{slug}", num, _names[num]


# ВсОШ проводится по закрытому списку общеобразовательных предметов.
# Профили перечневых олимпиад («робототехника», «медицина», «искусственный
# интеллект») предметами ВсОШ не являются: id вида vsosh-robototehnika
# указывает на несуществующую олимпиаду и пары в датасете C не найдёт.
VSOSH_SUBJECTS = {
    "математика", "русский язык", "иностранный язык", "английский язык",
    "немецкий язык", "французский язык", "испанский язык", "итальянский язык",
    "китайский язык", "информатика", "физика", "химия", "биология", "экология",
    "география", "астрономия", "литература", "история", "обществознание",
    "экономика", "право", "искусство", "физическая культура", "технология",
    "основы безопасности и защиты родины", "мировая художественная культура",
}


def vsosh_id(subject: str) -> str | None:
    key = clean(subject).lower()
    if key not in VSOSH_SUBJECTS:
        return None
    slug = profile_slug(key)
    return f"vsosh-{slug}" if slug else None


def level_in_perechen(num: int, profile: str) -> str | None:
    return _profiles.get(num, {}).get(norm(profile))


def demo() -> None:
    cases = [
        ('Всероссийская олимпиада школьников «Высшая проба»', 8),
        ('Высшая проба', 8),
        ('Олимпиада школьников "Ломоносов"', 50),
        ('Объединённая межвузовская математическая олимпиада школьников', None),
        ('Олимпиада школьников «Физтех»', None),
        ('"Формула Единства"/"Третье тысячелетие"', 2),
        ('Открытая олимпиада школьников', 64),
    ]
    for name, expected in cases:
        got = match_number(name)
        label = _names.get(got, "—") if got else "—"
        print(f"  {name[:52]:52s} -> {str(got):>4}  {label[:44]}")
        if expected is not None:
            assert got == expected, f"{name}: ожидали {expected}, получили {got}"
    assert olympiad_id('Всероссийская олимпиада школьников «Высшая проба»', 'информатика')[0] == "p669-8-informatika"
    assert vsosh_id("биология") == "vsosh-biologiya"
    assert vsosh_id("робототехника") is None, "ВсОШ по робототехнике не проводится"
    assert vsosh_id("медицина") is None
    assert level_in_perechen(8, "информатика") == "I"
    assert profile_slug("Математи ка") == "matematika"   # перенос внутри слова
    print("olymp_match: проверки прошли")


if __name__ == "__main__":
    demo()
