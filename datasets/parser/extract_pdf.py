#!/usr/bin/env python3
"""Снапшоты -> постраничный текст и таблицы в extracted/.

Номер страницы обязан дожить до JSON: спека требует source_page у каждого
факта из PDF. Поэтому извлекаем не «текст документа», а список страниц.

Отдельно считаем качество извлечения: у части вузовских PDF шрифты с кастомной
кодировкой, и наивный разбор молча отдаёт текст без цифр — именно те цифры,
ради которых документ и берётся (баллы, КЦП, номера).
"""
import json, re, sys, zipfile
from pathlib import Path

import pdfplumber

ROOT = Path(__file__).resolve().parent.parent
SNAP, OUT = ROOT / "snapshots", ROOT / "extracted"

CYR = re.compile(r"[а-яёА-ЯЁ]")
DIGIT = re.compile(r"\d")


def quality(text: str) -> dict:
    letters = [c for c in text if c.isalpha()]
    cyr_ratio = len(CYR.findall(text)) / len(letters) if letters else 0.0
    return {
        "chars": len(text),
        "cyrillic_ratio": round(cyr_ratio, 3),
        "has_digits": bool(DIGIT.search(text)),
    }


def verdict(pages: list[dict]) -> tuple[str, str]:
    if not pages:
        return "empty", "страниц не извлечено"
    full = "".join(p["text"] for p in pages)
    q = quality(full)
    if q["chars"] < 200 * len(pages) * 0.1:
        return "scan", f"всего {q['chars']} символов на {len(pages)} стр. — похоже на скан"
    if q["cyrillic_ratio"] < 0.5:
        return "bad_encoding", f"доля кириллицы {q['cyrillic_ratio']} — кастомная кодировка шрифта"
    if not q["has_digits"]:
        return "no_digits", "в тексте нет ни одной цифры — числа потерялись при декодировании"
    return "ok", ""


def extract_pdf(path: Path) -> list[dict]:
    pages = []
    with pdfplumber.open(path) as pdf:
        for i, page in enumerate(pdf.pages, start=1):
            try:
                text = page.extract_text() or ""
            except Exception as e:
                text = ""
                print(f"      стр. {i}: текст не извлёкся ({type(e).__name__})")
            try:
                tables = page.extract_tables() or []
            except Exception:
                tables = []
            pages.append({"page": i, "text": text, "tables": tables})
    return pages


def extract_docx(path: Path) -> list[dict]:
    """docx — zip с xml; python-docx ради одного файла Иннополиса не нужен."""
    with zipfile.ZipFile(path) as z:
        xml = z.read("word/document.xml").decode("utf-8", errors="replace")
    xml = re.sub(r"</w:p>", "\n", xml)
    xml = re.sub(r"</w:tr>", "\n", xml)
    xml = re.sub(r"</w:tc>", "\t", xml)
    text = re.sub(r"<[^>]+>", "", xml)
    text = re.sub(r"\n{3,}", "\n\n", text)
    return [{"page": 1, "text": text, "tables": []}]


def extract_html(path: Path) -> list[dict]:
    html = path.read_bytes().decode("utf-8", errors="replace").replace("­", "")
    return [{"page": 1, "text": html, "tables": [], "_raw_html": True}]


def sniff(path: Path) -> str:
    head = path.read_bytes()[:4]
    if head[:4] == b"%PDF":
        return "pdf"
    if head[:2] == b"PK":
        return "docx"
    return "html"


def main() -> int:
    report = []
    for meta_path in sorted(SNAP.rglob("*.meta.json")):
        meta = json.loads(meta_path.read_text(encoding="utf-8"))
        if meta.get("duplicate_of"):
            continue
        src = meta_path.with_suffix("")
        src = src.with_name(src.name.removesuffix(".meta")) if src.name.endswith(".meta") else src
        src = meta_path.parent / meta_path.name.removesuffix(".meta.json")
        if not src.exists():
            continue

        kind = sniff(src)
        if kind == "html":
            continue  # HTML разбирается в build_*.py напрямую, постраничности у него нет

        try:
            pages = extract_pdf(src) if kind == "pdf" else extract_docx(src)
        except Exception as e:
            report.append({"file": str(src.relative_to(ROOT)), "status": "error",
                           "detail": f"{type(e).__name__}: {e}"})
            print(f"  ОШИБКА {src.name}: {type(e).__name__}: {e}")
            continue

        status, detail = verdict(pages)
        out_dir = OUT / meta["vuz_id"]
        out_dir.mkdir(parents=True, exist_ok=True)
        with (out_dir / f"{src.name}.pages.jsonl").open("w", encoding="utf-8") as f:
            for p in pages:
                f.write(json.dumps(p, ensure_ascii=False) + "\n")

        report.append({
            "file": str(src.relative_to(ROOT)), "vuz_id": meta["vuz_id"], "role": meta["role"],
            "pages": len(pages), "status": status, "detail": detail,
            "source_url": meta["url"],
        })
        flag = "  " if status == "ok" else "!!"
        print(f"{flag} {meta['vuz_id']:11s} {meta['role']:11s} {len(pages):>4} стр.  {status:13s} {src.name[:45]} {detail}")

    (OUT / "_extract_report.json").write_text(
        json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    bad = [r for r in report if r["status"] != "ok"]
    print(f"\nдокументов: {len(report)}, чистых: {len(report) - len(bad)}, проблемных: {len(bad)}")
    for r in bad:
        print(f"  ТРЕБУЕТ РУЧНОГО РАЗБОРА  {r['vuz_id']}/{r['role']}: {r.get('detail') or r['status']}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
