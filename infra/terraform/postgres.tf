# PostgreSQL на собственной ВМ вместо Managed PostgreSQL.
#
# Управляемый кластер тарифицируется круглосуточно и для нагрузки MVP избыточен:
# единицы запросов в секунду и база в считанные мегабайты. Взамен к нам
# переезжает то, что раньше делал Яндекс: доступность, бэкапы и настройка TLS.
# Настройку самой СУБД держит Ansible (infra/ansible) — здесь только железо,
# сеть и права.
#
# Чего мы сознательно лишились вместе с кластером:
#   * автоматического переключения при отказе — хост один, и это единственная
#     точка отказа на окне проверки 30.09-14.10;
#   * пулера соединений на порту 6432 — теперь подключаемся прямо в 5432,
#     поэтому пул в коде ограничен двумя соединениями на инстанс (packages/db);
#   * бэкапов из коробки — их делает systemd-таймер, см. infra/ansible.

data "yandex_vpc_network" "default" {
  name = "default"
}

data "yandex_vpc_subnet" "default" {
  name = "default-${var.zone}"
}

data "yandex_compute_image" "db" {
  # В Ubuntu 24.04 штатный PostgreSQL 16 — та же мажорная версия, что в
  # docker-compose, в CI и в миграциях. Отдельный репозиторий не нужен.
  family = "ubuntu-2404-lts"
}

# Адрес резервируем: он попадает в DATABASE_URL всех четырёх функций, и
# меняться при перезапуске ВМ не должен.
resource "yandex_vpc_address" "db" {
  count = var.enable_database ? 1 : 0

  name = "traektoria-db"

  external_ipv4_address {
    zone_id = var.zone
  }
}

# Подключаемся к базе по имени, а не по адресу: если адрес когда-нибудь
# придётся пересоздать, строка подключения в Lockbox не изменится.
resource "yandex_dns_recordset" "db" {
  count = var.enable_database ? 1 : 0

  zone_id = yandex_dns_zone.domain.id
  name    = "db.${var.domain}."
  type    = "A"
  ttl     = 300
  data    = [yandex_vpc_address.db[0].external_ipv4_address[0].address]
}

resource "yandex_vpc_security_group" "db" {
  count = var.enable_database ? 1 : 0

  name        = "traektoria-db"
  description = "Доступ к PostgreSQL на ВМ"
  network_id  = data.yandex_vpc_network.default.id

  # 5432 открыт всему интернету намеренно, и это осознанный компромисс.
  # Cloud Functions работают вне VPC и постоянных исходящих адресов не имеют —
  # сузить источник нечем. Защита не в сетевом фильтре, а в самой СУБД:
  # подключение только по TLS (hostssl), пароли scram-sha-256, у пользователя
  # приложения нет прав суперпользователя. См. infra/ansible/roles/postgres.
  ingress {
    protocol       = "TCP"
    port           = 5432
    v4_cidr_blocks = ["0.0.0.0/0"]
    description    = "PostgreSQL: функции ходят снаружи VPC, только TLS"
  }

  # SSH нужен Ansible. Диапазон задаётся явно: по умолчанию не открыт никому,
  # чтобы случайный apply не выставил порт в интернет. При пустом списке
  # правила нет вовсе: правило без адресов API отклоняет («target: One of the
  # options must be selected»).
  dynamic "ingress" {
    for_each = length(var.ssh_allowed_cidrs) > 0 ? [1] : []
    content {
      protocol       = "TCP"
      port           = 22
      v4_cidr_blocks = var.ssh_allowed_cidrs
      description    = "SSH для Ansible"
    }
  }

  egress {
    protocol       = "ANY"
    v4_cidr_blocks = ["0.0.0.0/0"]
    description    = "Обновления пакетов и выгрузка бэкапов"
  }
}

# Данные лежат на отдельном диске, а не на загрузочном: ВМ можно пересоздать
# (сменить образ, размер, платформу), не потеряв базу.
resource "yandex_compute_disk" "db_data" {
  count = var.enable_database ? 1 : 0

  name = "traektoria-db-data"
  type = var.db_data_disk_type
  size = var.db_data_disk_size
  zone = var.zone

  # В базе персональные данные школьников. Случайный apply с
  # enable_database = false должен упасть на защите, а не стереть их —
  # ровно так же вёл себя deletion_protection у управляемого кластера.
  lifecycle {
    prevent_destroy = true
  }
}

resource "yandex_compute_instance" "db" {
  count = var.enable_database ? 1 : 0

  name        = "traektoria-db"
  description = "PostgreSQL 16 для «Траектории»"
  platform_id = "standard-v3"
  zone        = var.zone

  # Изменение ресурсов требует остановки; без флага apply просто упадёт.
  allow_stopping_for_update = true

  resources {
    cores = var.db_cores
    # Доля гарантированного ядра. 20% — минимум для standard-v3 и самый дешёвый
    # вариант; всплески обслуживаются сверх неё, а нагрузка у нас редкая.
    core_fraction = var.db_core_fraction
    memory        = var.db_memory
  }

  boot_disk {
    initialize_params {
      image_id = data.yandex_compute_image.db.id
      # На загрузочном диске только система: он дешёвый и одноразовый.
      type = "network-hdd"
      size = 20
    }
  }

  secondary_disk {
    disk_id     = yandex_compute_disk.db_data[0].id
    device_name = "pgdata"
    auto_delete = false
  }

  network_interface {
    subnet_id          = data.yandex_vpc_subnet.default.id
    nat                = true
    nat_ip_address     = yandex_vpc_address.db[0].external_ipv4_address[0].address
    security_group_ids = [yandex_vpc_security_group.db[0].id]
  }

  # Прерываемую ВМ здесь использовать нельзя: её останавливают раз в сутки,
  # а база должна быть доступна непрерывно всё окно проверки.
  scheduling_policy {
    preemptible = false
  }

  service_account_id = yandex_iam_service_account.db[0].id

  metadata = {
    ssh-keys = "ubuntu:${var.ssh_public_key}"
    # Пароли и ключи в метаданные не кладём: их видно любому, кто может
    # смотреть ВМ. Всё чувствительное приносит Ansible по SSH.
  }

  lifecycle {
    # Образ обновляется сам по себе при каждом новом релизе Ubuntu; пересоздавать
    # из-за этого ВМ с базой нельзя. Обновлять — осознанно, сняв игнор.
    ignore_changes = [boot_disk[0].initialize_params[0].image_id]
  }
}

# --- Бэкапы -------------------------------------------------------------------
# Отдельный сервисный аккаунт с правом только записи в бакет: ключ лежит на ВМ,
# и если её скомпрометируют, им нельзя ни прочитать чужое, ни что-то удалить.

resource "yandex_iam_service_account" "db" {
  count = var.enable_database ? 1 : 0

  name        = "traektoria-db"
  description = "Выгрузка бэкапов PostgreSQL в Object Storage"
}

resource "yandex_resourcemanager_folder_iam_member" "db_uploader" {
  count = var.enable_database ? 1 : 0

  folder_id = var.folder_id
  role      = "storage.uploader"
  member    = "serviceAccount:${yandex_iam_service_account.db[0].id}"
}

resource "yandex_iam_service_account_static_access_key" "db" {
  count = var.enable_database ? 1 : 0

  service_account_id = yandex_iam_service_account.db[0].id
  description        = "S3-ключ для выгрузки бэкапов с ВМ"
}

resource "yandex_storage_bucket" "backups" {
  count = var.enable_database ? 1 : 0

  bucket = var.backup_bucket

  access_key = yandex_iam_service_account_static_access_key.storage.access_key
  secret_key = yandex_iam_service_account_static_access_key.storage.secret_key

  # Бэкапы приватны по умолчанию; anonymous_access_flags не задаём вовсе.

  lifecycle_rule {
    id      = "expire"
    enabled = true

    expiration {
      days = var.backup_retention_days
    }
  }

  versioning {
    enabled = true
  }
}
