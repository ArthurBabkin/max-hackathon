data "yandex_vpc_network" "default" {
  name = "default"
}

data "yandex_vpc_subnet" "default" {
  name = "default-${var.zone}"
}

# Managed PostgreSQL. Публичный доступ к хосту включён намеренно: так функции
# достают базу без привязки к VPC и без NAT-шлюза. Обязателен sslmode=require.
resource "yandex_mdb_postgresql_cluster" "main" {
  count = var.enable_database ? 1 : 0

  name        = "traektoria"
  environment = "PRODUCTION"
  network_id  = data.yandex_vpc_network.default.id

  config {
    version = "16"

    resources {
      resource_preset_id = var.pg_preset
      disk_type_id       = "network-ssd"
      disk_size          = var.pg_disk_size
    }

    access {
      # Нужен, чтобы Cloud Functions ходили в базу снаружи VPC.
      web_sql = false
    }

    # Единственная статья расходов, которая тикает круглосуточно независимо
    # от нагрузки. Бэкап держим неделю: окно проверки 30.09-14.10 это покрывает.
    backup_retain_period_days = 7
  }

  dynamic "host" {
    for_each = range(var.pg_host_count)
    content {
      zone             = var.zone
      subnet_id        = data.yandex_vpc_subnet.default.id
      assign_public_ip = true
    }
  }
}

resource "yandex_mdb_postgresql_database" "main" {
  count = var.enable_database ? 1 : 0

  cluster_id = yandex_mdb_postgresql_cluster.main[0].id
  name       = "traektoria"
  owner      = yandex_mdb_postgresql_user.app[0].name
}

resource "yandex_mdb_postgresql_user" "app" {
  count = var.enable_database ? 1 : 0

  cluster_id = yandex_mdb_postgresql_cluster.main[0].id
  name       = "traektoria"
  password   = var.pg_password
}
