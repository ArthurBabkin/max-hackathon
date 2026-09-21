#!/usr/bin/env python3
"""Скачивает документы из sources.yaml в snapshots/ и пишет метаданные рядом.

Каждый факт в датасетах должен ссылаться на конкретный документ и страницу,
поэтому снапшот с хешем — не кеш, а доказательство: по нему можно открыть
ровно ту редакцию, из которой факт взят.
"""
import hashlib, json, re, sys, time
from datetime import date, datetime, timezone
from pathlib import Path
from urllib.parse import quote, urlsplit, urlunsplit

import requests
import urllib3
import yaml

urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)

ROOT = Path(__file__).resolve().parent.parent
SNAP = ROOT / "snapshots"
UA = "Mozilla/5.0 (compatible; olymp-navigator/1.0; dataset build for MAX hackathon)"


def encode_url(url: str) -> str:
    """Кириллица и пробелы в путях (НГУ, Иннополис) ломают requests без этого."""
    p = urlsplit(url)
    return urlunsplit((p.scheme, p.netloc, quote(p.path, safe="/%"), quote(p.query, safe="=&%"), p.fragment))


def slug_for(url: str, role: str) -> str:
    tail = urlsplit(url).path.rstrip("/").split("/")[-1] or "index"
    tail = re.sub(r"[^A-Za-z0-9._-]+", "_", tail)[:60]
    return f"{role}__{tail}"


def itmo_build_id(session: requests.Session) -> str | None:
    """buildId протухает при каждом деплое фронта — читаем перед каждым прогоном."""
    try:
        html = session.get("https://abit.itmo.ru/bachelor", timeout=60).text
        m = re.search(r'"buildId":"([^"]+)"', html)
        return m.group(1) if m else None
    except requests.RequestException:
        return None


def fetch_one(session, url, verify, timeout=180):
    r = session.get(encode_url(url), timeout=timeout, allow_redirects=True, verify=verify)
    r.raise_for_status()
    return r


def main() -> int:
    cfg = yaml.safe_load((Path(__file__).parent / "sources.yaml").read_text(encoding="utf-8"))
    transport = cfg.get("transport", {})
    default_delay = cfg.get("defaults", {}).get("delay_sec", 2.0)

    session = requests.Session()
    session.headers["User-Agent"] = UA

    today = date.today().isoformat()
    by_hash: dict[str, str] = {}
    report = {"ok": 0, "dedup": 0, "failed": [], "needs_discovery": []}

    for vuz_id, vuz in cfg["vuzy"].items():
        delay = vuz.get("delay_sec", default_delay)
        out_dir = SNAP / vuz_id
        out_dir.mkdir(parents=True, exist_ok=True)

        for doc in vuz["docs"]:
            url, role = doc.get("url", ""), doc["role"]

            if url == "itmo-next-json":
                bid = itmo_build_id(session)
                if not bid:
                    report["needs_discovery"].append(f"{vuz_id}/{role}: не прочитался buildId")
                    continue
                url = f"https://abit.itmo.ru/_next/data/{bid}/programs/bachelor.json"
                merged, page_no = [], 1
                while True:
                    u = url if page_no == 1 else f"{url}?page={page_no}"
                    try:
                        chunk = fetch_one(session, u, verify=True).json()["pageProps"].get("initialPrograms") or []
                    except Exception:
                        break
                    if not chunk:
                        break
                    merged.extend(chunk)
                    page_no += 1
                    time.sleep(delay)
                if merged:
                    out_dir.mkdir(parents=True, exist_ok=True)
                    body = json.dumps({"pageProps": {"initialPrograms": merged}},
                                      ensure_ascii=False).encode("utf-8")
                    (out_dir / "programs__bachelor.json").write_bytes(body)
                    (out_dir / "programs__bachelor.json.meta.json").write_text(json.dumps({
                        "vuz_id": vuz_id, "role": role, "url": url, "final_url": url,
                        "sha256": hashlib.sha256(body).hexdigest(), "bytes": len(body),
                        "content_type": "application/json", "last_modified": None,
                        "fetched_at": today, "campus": None, "benefit": None,
                        "note": f"склеено {page_no - 1} страниц каталога, {len(merged)} программ",
                    }, ensure_ascii=False, indent=2), encoding="utf-8")
                    print(f"  {vuz_id:11s} {role:11s} {len(body):>9,} б  programs__bachelor.json "
                          f"({len(merged)} программ)")
                    report["ok"] += 1
                continue

            if not url or doc.get("discover") and not url.startswith("http"):
                report["needs_discovery"].append(f"{vuz_id}/{role}: {doc.get('note', 'URL ищется на landing')}")
                continue

            host = urlsplit(url).netloc
            rules = transport.get(host, {})
            if rules.get("skip"):
                report["failed"].append(f"{vuz_id}/{role}: хост {host} помечен skip")
                continue

            name = slug_for(url, role)
            path, meta_path = out_dir / name, out_dir / f"{name}.meta.json"

            if meta_path.exists() and path.exists():
                report["ok"] += 1
                continue

            try:
                r = fetch_one(session, url, verify=rules.get("verify_ssl", True))
            except Exception as e:
                report["failed"].append(f"{vuz_id}/{role}: {url[:80]} -> {type(e).__name__}: {e}")
                time.sleep(delay)
                continue

            body = r.content
            digest = hashlib.sha256(body).hexdigest()

            if digest in by_hash:
                report["dedup"] += 1
                meta_path.write_text(json.dumps({
                    "url": url, "sha256": digest, "duplicate_of": by_hash[digest],
                    "fetched_at": today, "note": "байт в байт совпадает с другим документом",
                }, ensure_ascii=False, indent=2), encoding="utf-8")
                time.sleep(delay)
                continue
            by_hash[digest] = f"{vuz_id}/{name}"

            path.write_bytes(body)
            meta_path.write_text(json.dumps({
                "vuz_id": vuz_id, "role": role, "url": url, "final_url": r.url,
                "sha256": digest, "bytes": len(body),
                "content_type": r.headers.get("Content-Type", ""),
                "last_modified": r.headers.get("Last-Modified"),
                "fetched_at": today,
                "fetched_at_utc": datetime.now(timezone.utc).isoformat(timespec="seconds"),
                "campus": doc.get("campus"), "benefit": doc.get("benefit"), "note": doc.get("note"),
            }, ensure_ascii=False, indent=2), encoding="utf-8")

            report["ok"] += 1
            print(f"  {vuz_id:11s} {role:11s} {len(body):>9,} б  {name}")
            time.sleep(delay)

    print(f"\nскачано/в кеше: {report['ok']}, дубликатов: {report['dedup']}, "
          f"ошибок: {len(report['failed'])}, требуют поиска на landing: {len(report['needs_discovery'])}")
    for line in report["failed"]:
        print(f"  ОШИБКА  {line}")
    for line in report["needs_discovery"]:
        print(f"  ИСКАТЬ  {line}")
    (SNAP / "_fetch_report.json").write_text(
        json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    return 0


if __name__ == "__main__":
    sys.exit(main())
