#!/usr/bin/env python3
"""Города России для поиска места в боте -> packages/core/refdata/places.json.

Источник — https://github.com/hflabs/city (CC BY-SA 4.0), файл city.csv.
Сам CSV в репозиторий не кладём: скачайте его и передайте путь.

    curl -sSfLO https://raw.githubusercontent.com/hflabs/city/master/city.csv
    python3 datasets/parser/build_places.py city.csv

Код региона — первые две цифры КЛАДР, они совпадают с кодами в
packages/core/refdata/regions.json. Города федерального значения (Москва,
Петербург, Севастополь) не берём: это регионы, их находит поиск по региону.
Вывод детерминирован: тот же вход — байт в байт тот же файл.
"""
from __future__ import annotations

import csv
import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
REGIONS = ROOT / "packages/core/refdata/regions.json"
OUT = ROOT / "packages/core/refdata/places.json"


def city_name(row: dict[str, str]) -> str:
    """Название города: в city, у части городов Подмосковья — в area,
    у Урус-Мартана — в settlement."""
    for key, kind in (("city", "city_type"), ("area", "area_type"), ("settlement", "settlement_type")):
        if row[key] and (key == "city" or row[kind] == "г"):
            return row[key]
    return ""


def build(rows: list[dict[str, str]], codes: set[str]) -> list[dict]:
    out = []
    for row in rows:
        name = city_name(row)
        if not name:
            continue  # город федерального значения — это регион
        code = row["kladr_id"][:2]
        if code not in codes:
            raise SystemExit(f"неизвестный код региона {code}: {row['address']}")
        out.append({
            "name": name,
            "region_code": code,
            "population": int(row["population"] or 0),
            "lat": round(float(row["geo_lat"]), 4),
            "lon": round(float(row["geo_lon"]), 4),
        })
    out.sort(key=lambda c: (c["region_code"], c["name"]))
    return out


def main() -> None:
    if len(sys.argv) != 2:
        raise SystemExit("использование: build_places.py path/to/city.csv")
    codes = {r["code"] for r in json.loads(REGIONS.read_text())["regions"]}
    with open(sys.argv[1], newline="", encoding="utf-8") as f:
        places = build(list(csv.DictReader(f)), codes)
    body = ",\n".join(json.dumps(p, ensure_ascii=False) for p in places)
    OUT.write_text("[\n" + body + "\n]\n", encoding="utf-8")
    print(f"{OUT.relative_to(ROOT)}: {len(places)} городов")


if __name__ == "__main__":
    main()
