# Секреты приложения — в Lockbox. Функция получает их переменными окружения
# при старте, но в версии функции их не видно. Значения приходят из TF_VAR_*
# и, как любые значения ресурсов, лежат в terraform.tfstate — он в git не
# попадает.

locals {
  database_url = var.enable_database ? format(
    "postgres://%s:%s@%s:6432/%s?sslmode=require",
    yandex_mdb_postgresql_user.app[0].name,
    var.pg_password,
    try(yandex_mdb_postgresql_cluster.main[0].host[0].fqdn, ""),
    yandex_mdb_postgresql_database.main[0].name,
  ) : var.database_url

  # Пустые значения в Lockbox не кладём: пустая запись там недопустима, а
  # приложение трактует отсутствие переменной так же, как пустую. Пометку
  # «секрет» снимаем только с факта «задано/не задано», не со значения —
  # иначе список ключей стал бы секретным и не годился бы для for_each.
  secret_values = { for k, v in {
    MAX_BOT_TOKEN    = var.max_bot_token
    WEBHOOK_SECRET   = var.webhook_secret
    JWT_SECRET       = var.jwt_secret
    POLZA_AI_API_KEY = var.polza_ai_api_key
    DATABASE_URL     = local.database_url
  } : k => v if nonsensitive(v != "") }

  # Каждой функции — только нужные ей секреты.
  function_secrets = {
    bot       = ["MAX_BOT_TOKEN", "WEBHOOK_SECRET", "DATABASE_URL"]
    api       = ["MAX_BOT_TOKEN", "JWT_SECRET", "POLZA_AI_API_KEY", "DATABASE_URL"]
    reminders = ["MAX_BOT_TOKEN", "DATABASE_URL"]
    notifier  = ["MAX_BOT_TOKEN", "DATABASE_URL"]
  }
}

resource "yandex_lockbox_secret" "app" {
  name        = "traektoria-app"
  description = "Токены и строка подключения функций «Траектории»"
}

resource "yandex_lockbox_secret_version" "app" {
  # Пока не задан ни один секрет, версию не создаём. Lockbox требует минимум
  # одну запись, а local.secret_values отфильтровывает пустые значения — и
  # когда пусты все, блок entries не создаётся ни разу:
  #   Error: Insufficient entries blocks
  # Без этого условия план падает у всех, кто не передал TF_VAR_*, включая
  # PR, который правит только DNS и к секретам отношения не имеет.
  count = length(local.secret_values) > 0 ? 1 : 0

  secret_id = yandex_lockbox_secret.app.id

  dynamic "entries" {
    for_each = local.secret_values
    content {
      key        = entries.key
      text_value = entries.value
    }
  }
}

# От имени этого аккаунта функции читают секреты при старте.
resource "yandex_iam_service_account" "functions" {
  name        = "traektoria-functions"
  description = "Чтение секретов Lockbox функциями"
}

resource "yandex_resourcemanager_folder_iam_member" "functions_lockbox" {
  folder_id = var.folder_id
  role      = "lockbox.payloadViewer"
  member    = "serviceAccount:${yandex_iam_service_account.functions.id}"
}
