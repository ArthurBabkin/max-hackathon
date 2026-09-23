#!/usr/bin/env python3
"""Выкладка кода в функцию Cloud Functions с настройками её текущей версии.

Terraform владеет настройками функции (окружение, секреты Lockbox, сервисный
аккаунт, память, таймаут), CI — её кодом. `yc serverless function version
create` от предыдущей версии не наследует ничего: версия без явных флагов
вышла бы без секретов, и бот упал бы на старте. Поэтому настройки берутся у
текущей версии с тегом $latest, а код — из собранного архива.

Если после выкладки проверка живости не прошла, тег $latest возвращается на
прежнюю версию: откат без правки кода.

Запуск: deploy_function.py --name traektoria-bot --require A,B --probe bot
"""

import argparse
import json
import os
import subprocess
import sys
import time
import urllib.error
import urllib.request

# Что переносится в новую версию или не является настройкой. Всё прочее,
# если оно задано, — повод обновить скрипт: предупреждаем, а не молчим.
KNOWN = {
    "id", "function_id", "description", "created_at", "runtime", "entrypoint",
    "resources", "execution_timeout", "service_account_id", "image_size",
    "status", "tags", "log_group_id", "environment", "secrets", "concurrency",
    "log_options",
}


def fail(msg):
    print(f"::error::{msg}")
    sys.exit(1)


def yc(*args, check=True):
    r = subprocess.run(["yc", *args, "--format", "json"], capture_output=True, text=True)
    if check and r.returncode != 0:
        fail(f"yc {' '.join(args[:4])}: {r.stderr.strip()}")
    return r


def env_flag(key, value):
    # pflag stringToString: пара с одним «=» берётся целиком, запятые в
    # значении не мешают; с несколькими «=» строка разбирается как CSV.
    pair = f"{key}={value}"
    if "=" not in value:
        return pair
    return '"' + pair.replace('"', '""') + '"'


def probe(url, method, headers, expect):
    req = urllib.request.Request(url, method=method, headers=headers,
                                 data=b"{}" if method == "POST" else None)
    for attempt in range(3):
        try:
            code = urllib.request.urlopen(req, timeout=30).status
        except urllib.error.HTTPError as e:
            code = e.code
        except OSError as e:
            code = str(e)
        print(f"{method} {url} -> {code} (ждём {expect})")
        if code == expect:
            return True
        time.sleep(10)
    return False


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--name", required=True)
    p.add_argument("--require", default="", help="переменные, без которых код не стартует")
    p.add_argument("--optional", action="store_true", help="функции может не быть (enable_workers=false)")
    p.add_argument("--source", default="build")
    p.add_argument("--probe", choices=["bot", "api", "none"], default="none")
    p.add_argument("--api-url", default="")
    a = p.parse_args()

    r = yc("serverless", "function", "get", "--name", a.name, check=False)
    if r.returncode != 0:
        if a.optional and "notfound" in r.stderr.replace(" ", "").lower():
            print(f"::notice::{a.name}: функции нет (enable_workers=false) — пропускаем")
            return
        fail(f"{a.name}: {r.stderr.strip()}")
    function_id = json.loads(r.stdout)["id"]

    prev = json.loads(yc("serverless", "function", "version", "get-by-tag",
                         "--function-name", a.name, "--tag", "$latest").stdout)
    env = prev.get("environment") or {}
    secrets = prev.get("secrets") or []
    have = set(env) | {s.get("environment_variable") for s in secrets}
    missing = [k for k in a.require.split(",") if k and k not in have]
    if missing:
        fail(f"{a.name}: у текущей версии нет {', '.join(missing)}. Настройки функции "
             "задаёт Terraform: сначала Actions → Infra apply, см. .github/workflows/README.md")
    for k, v in prev.items():
        if k not in KNOWN and v not in (None, "", "0", 0, False, [], {}):
            print(f"::warning::{a.name}: настройка {k} не переносится в новую версию: {v}")
    # Логи в группу каталога по умолчанию — это и есть поведение новой версии.
    if set(prev.get("log_options") or {}) - {"folder_id"}:
        print(f"::warning::{a.name}: log_options не переносятся: {prev['log_options']}")

    memory_mb = int(prev["resources"]["memory"]) // (1024 * 1024)
    cmd = ["serverless", "function", "version", "create",
           "--function-name", a.name,
           "--runtime", prev["runtime"],
           "--entrypoint", prev["entrypoint"],
           "--memory", f"{memory_mb}MB",
           "--execution-timeout", prev["execution_timeout"],
           "--source-path", a.source]
    if os.environ.get("GITHUB_SHA"):
        cmd += ["--description", f"commit {os.environ['GITHUB_SHA'][:7]}"]
    if prev.get("service_account_id"):
        cmd += ["--service-account-id", prev["service_account_id"]]
    if int(prev.get("concurrency") or 1) > 1:
        cmd += ["--concurrency", str(prev["concurrency"])]
    for k, v in env.items():
        cmd += ["--environment", env_flag(k, v)]
    for s in secrets:
        cmd += ["--secret", f"id={s['id']},version-id={s['version_id']},"
                            f"key={s['key']},environment-variable={s['environment_variable']}"]

    print(f"{a.name}: прежняя версия {prev['id']}, окружение {sorted(env)}, "
          f"секреты {sorted(s['environment_variable'] for s in secrets)}")
    new = json.loads(yc(*cmd).stdout)
    print(f"{a.name}: новая версия {new['id']}")

    if a.probe == "bot":
        # Чужой секрет вебхука — 403 от нашего обработчика. Значит, версия
        # поднялась: без токена, секрета или с битым конфигом init валит
        # инстанс, и вызов отвечает 502. В базу и в MAX проба не ходит.
        ok = probe(f"https://functions.yandexcloud.net/{function_id}", "POST",
                   {"Content-Type": "application/json", "X-Max-Bot-Api-Secret": "ci-probe-not-the-secret"}, 403)
    elif a.probe == "api" and a.api_url:
        ok = probe(a.api_url.rstrip("/") + "/health", "GET", {}, 200)
    else:
        if a.probe == "api":
            print("::warning::переменная YC_API_URL не задана — api не проверяется")
        ok = True

    if not ok:
        yc("serverless", "function", "version", "set-tag", "--id", prev["id"], "--tag", "$latest")
        fail(f"{a.name}: проверка не прошла, $latest возвращён на {prev['id']}")


if __name__ == "__main__":
    main()
