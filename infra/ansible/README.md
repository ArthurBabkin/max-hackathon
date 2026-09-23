# PostgreSQL на своей ВМ

Terraform поднимает железо и сеть (`infra/terraform/postgres.tf`), Ansible
настраивает саму СУБД. Разделение простое: всё, что можно описать ресурсом
облака, — в Terraform; всё, что живёт внутри машины, — здесь.

## Почему не Managed PostgreSQL

Управляемый кластер тарифицируется круглосуточно и для нагрузки MVP избыточен:
единицы запросов в секунду и база в считанные мегабайты. Взамен экономии к нам
переехало то, что Яндекс делал сам:

- **доступности нет.** Хост один, автоматического переключения нет. На окне
  проверки 30.09–14.10 это единственная точка отказа всего решения.
- **бэкапы наши.** Их делает systemd-таймер: ежедневный `pg_dump` в
  Object Storage. Срок хранения в бакете задаёт Terraform.
- **пулера соединений нет.** У управляемого кластера на порту 6432 стоял
  Odyssey; здесь подключаемся прямо в 5432. Поэтому пул в `packages/db`
  ограничен двумя соединениями на инстанс функции — менять с оглядкой на
  `max_connections`.

## Что делает роль

Монтирует отдельный диск под данные **до** установки пакета, чтобы кластер сразу
создался на нём: ВМ после этого можно пересоздать, не потеряв базу. Ставит
PostgreSQL 16 — ту же мажорную версию, что в `docker-compose`, в CI и в
`packages/db/migrations`. Включает TLS и `scram-sha-256`, заводит роль и базу
приложения без прав суперпользователя, разворачивает ежедневный бэкап.

Порт 5432 открыт всему интернету, и это осознанный компромисс: Cloud Functions
работают вне VPC и постоянных исходящих адресов не имеют, сузить источник
нечем. Поэтому защита не в сетевом фильтре, а в самой СУБД — в `pg_hba.conf`
нет ни одной строки `host` без `ssl`, так что нешифрованное подключение
отвергает сервер, а не клиент.

## Прогон

Сначала `apply` с включённой базой и открытым для вас SSH:

```bash
cd infra/terraform
export TF_VAR_ssh_public_key="$(cat ~/.ssh/id_ed25519.pub)"
tofu apply -var enable_database=true -var "ssh_allowed_cidrs=[\"$(curl -s ifconfig.me)/32\"]"
```

Затем подготовьте инвентарь и переменные:

```bash
cd ../ansible
ansible-galaxy collection install -r requirements.yml
cp inventory.ini.example inventory.ini
cp group_vars/all.yml.example group_vars/all.yml
chmod 600 group_vars/all.yml
```

Значения читать **по одному**, через `-raw`. `tofu output` без аргументов
покажет всё сразу, включая секреты:

```bash
tofu -chdir=../terraform output -raw db_host
tofu -chdir=../terraform output -raw db_backup_access_key
tofu -chdir=../terraform output -raw db_backup_secret_key
```

> Состояние лежит в Object Storage, поэтому `tofu output` требует `AWS_ACCESS_KEY_ID`
> и `AWS_SECRET_ACCESS_KEY` (см. `infra/terraform/README.md`). Без них команда
> пишет ошибку в stderr и **ничего** в stdout — а `$(...)` этого не заметит и
> подставит пустую строку. Проверяйте, что значение непустое.

`pg_password` — тот же, что в `TF_VAR_pg_password`: Terraform кладёт его в
строку подключения для Lockbox, Ansible заводит с ним роль. Разойдутся —
функции не подключатся.

Дальше:

```bash
ansible-playbook playbook.yml
```

Роль идемпотентна: повторный прогон ничего не ломает и ничего не пересоздаёт.

## После первого прогона

Миграции катит CI (`.github/workflows/deploy-functions.yml`), ему нужен секрет
`DATABASE_URL`:

```bash
tofu -chdir=../terraform output -raw database_url
```

Проверить, что база видна снаружи и требует TLS:

```bash
psql "$(tofu -chdir=../terraform output -raw database_url)" -c 'select version()'
```

Проверить бэкап, не дожидаясь ночи:

```bash
ssh ubuntu@$(tofu -chdir=../terraform output -raw db_host) \
  sudo systemctl start traektoria-backup
```

## Что в git не попадает

`inventory.ini` и `group_vars/all.yml` — в них адрес, пароль и ключи. В
репозитории лежат только файлы `.example`.
