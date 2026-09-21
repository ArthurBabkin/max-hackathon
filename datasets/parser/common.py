#!/usr/bin/env python3
"""Общие помощники сборки датасетов A и B."""
import html as _html
import json, re, unicodedata
from datetime import date
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
SNAP, EXTRACTED, DATA = ROOT / "snapshots", ROOT / "extracted", ROOT / "data"
TODAY = date.today().isoformat()
ADMISSION_YEAR = 2026

CODE_RE = re.compile(r"\b(\d{2}\.\d{2}\.\d{2})\b")

TRANSLIT = {
    "а": "a", "б": "b", "в": "v", "г": "g", "д": "d", "е": "e", "ё": "e", "ж": "zh",
    "з": "z", "и": "i", "й": "y", "к": "k", "л": "l", "м": "m", "н": "n", "о": "o",
    "п": "p", "р": "r", "с": "s", "т": "t", "у": "u", "ф": "f", "х": "h", "ц": "c",
    "ч": "ch", "ш": "sh", "щ": "sch", "ъ": "", "ы": "y", "ь": "", "э": "e",
    "ю": "yu", "я": "ya",
}


def clean(s) -> str:
    if s is None:
        return ""
    s = unicodedata.normalize("NFKC", _html.unescape(str(s))).replace("­", "").replace("\xa0", " ")
    return re.sub(r"\s+", " ", s).strip()


def slugify(s: str, maxlen: int = 48) -> str:
    out = []
    for ch in clean(s).lower():
        if ch in TRANSLIT:
            out.append(TRANSLIT[ch])
        elif ch.isalnum() and ch.isascii():
            out.append(ch)
        else:
            out.append("-")
    return re.sub(r"-+", "-", "".join(out)).strip("-")[:maxlen].strip("-")


def to_int(s):
    s = clean(s).replace(" ", "")
    m = re.fullmatch(r"\d+", s)
    return int(m.group()) if m else None


def load_pages(vuz_id: str, filename: str) -> list[dict]:
    p = EXTRACTED / vuz_id / f"{filename}.pages.jsonl"
    return [json.loads(l) for l in p.read_text(encoding="utf-8").splitlines()]


def load_html(vuz_id: str, filename: str) -> str:
    return (SNAP / vuz_id / filename).read_bytes().decode("utf-8", errors="replace").replace("­", "")


def meta(vuz_id: str, filename: str) -> dict:
    return json.loads((SNAP / vuz_id / f"{filename}.meta.json").read_text(encoding="utf-8"))


def html_rows(html: str, table_index: int | None = None) -> list[list[str]]:
    """Строки HTML-таблиц с развёрнутым rowspan.

    Без развёртки rowspan теряются ячейки, заданные один раз на блок строк
    (перечень РСОШ, план приёма МФТИ) — это не косметика, а треть данных.
    """
    tables = re.findall(r"<table.*?</table>", html, re.S)
    if table_index is not None:
        tables = [tables[table_index]] if table_index < len(tables) else []
    out = []
    for table in tables:
        carry: dict[int, list] = {}
        width = 0
        for row_html in re.findall(r"<tr[^>]*>(.*?)</tr>", table, re.S):
            cells = re.findall(r"<t[dh]([^>]*)>(.*?)</t[dh]>", row_html, re.S)
            if not cells:
                continue
            width = max(width, len(cells) + len(carry))
            rec: list = [None] * 40
            for ci, held in list(carry.items()):
                if held[1] > 0:
                    rec[ci] = held[0]
                    held[1] -= 1
            i = 0
            for attrs, raw in cells:
                while i < 40 and rec[i] is not None:
                    i += 1
                if i >= 40:
                    break
                txt = clean(re.sub(r"<[^>]+>", " ", raw))
                rec[i] = txt
                span = re.search(r'rowspan="?(\d+)"?', attrs)
                if span:
                    carry[i] = [txt, int(span.group(1)) - 1]
                i += 1
            trimmed = [c for c in rec if c is not None]
            if trimmed:
                out.append(trimmed)
    return out


def write_json(path: Path, key: str, rows: list, note: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps({
        "_generated_at": TODAY,
        "_note": note,
        "_count": len(rows),
        key: rows,
    }, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"записано {len(rows)} объектов -> {path.relative_to(ROOT)}")
