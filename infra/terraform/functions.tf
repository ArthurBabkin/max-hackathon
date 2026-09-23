# Все четыре функции живут в одном Go-модуле, поэтому zip у них общий:
# отличаются только точкой входа. Лишнее вырезаем — в архив должны попасть
# только исходники, иначе туда уедут датасеты, экраны и история git.
data "archive_file" "backend" {
  type        = "zip"
  source_dir  = "${path.module}/../.."
  output_path = "${path.module}/.build/backend.zip"

  # Архив собирается из корня репозитория, поэтому локальные секреты и
  # служебные каталоги исключаются явно: .env с токеном бота не должен
  # попасть в облако внутри исходников.
  excludes = [
    ".env",
    ".claude",
    ".DS_Store",
    ".git",
    ".github",
    "docs",
    "datasets",
    "dataset_c_src",
    "infra",
    "apps/web",
    "olimpiady_spravochnik.json",
    "DATASET_C_NOTES.md",
  ]
}

locals {
  # Бот нужен всегда: это зарегистрированный в MAX вебхук.
  # Остальные включаются флагом enable_workers.
  functions = var.enable_workers ? local.all_functions : {
    bot = local.all_functions.bot
  }
}

locals {
  # Точка входа обязана указывать на main-пакет: рантайм собирает её как
  # Go-плагин (-buildmode=plugin), а плагин строится только из main.
  # Формат — <путь к каталогу>/<имя пакета>.<Функция>.
  all_functions = {
    bot = {
      name        = "traektoria-bot"
      description = "MAX webhook (Траектория)"
      entrypoint  = "apps/bot/cmd/function/main.Handler"
      memory      = 128
      # MAX ждёт ответа вебхука до 30 секунд; обработчик сам укладывается в 25.
      timeout = 30
      public  = true
    }
    api = {
      name        = "traektoria-api"
      description = "REST мини-приложения"
      entrypoint  = "apps/api/cmd/function/main.Handler"
      memory      = 256
      timeout     = 30
      # Снаружи api доступен только через шлюз (apigateway.tf): он вызывает
      # функцию от имени traektoria-invoker, прямой вызов без путей бесполезен.
      public = false
    }
    reminders = {
      name        = "traektoria-reminders"
      description = "Воркер напоминаний, Timer-триггер"
      entrypoint  = "apps/reminders/cmd/function/main.Handler"
      memory      = 256
      timeout     = 60
      public      = false
    }
    notifier = {
      name        = "traektoria-content-notifier"
      description = "Уведомления об изменениях контента, Timer-триггер"
      entrypoint  = "apps/notifier/cmd/function/main.Handler"
      memory      = 256
      timeout     = 60
      public      = false
    }
  }
}

resource "yandex_function" "fn" {
  for_each = local.functions

  name        = each.value.name
  description = each.value.description

  runtime           = "golang123"
  entrypoint        = each.value.entrypoint
  memory            = each.value.memory
  execution_timeout = each.value.timeout

  user_hash = data.archive_file.backend.output_base64sha256

  content {
    zip_filename = data.archive_file.backend.output_path
  }

  # Прод: дев-обход подписи initData выключен и не включится — api с
  # APP_ENV=production и DEV_UNSIGNED_INITDATA=true не стартует.
  environment = merge({
    APP_ENV       = "production"
    MAX_API_BASE  = var.max_api_base
    MAX_BOT_NAME  = var.max_bot_name
    MAX_BOT_ID    = var.max_bot_id
    REMINDER_HOUR = var.reminder_hour
    DB_MAX_CONNS  = "2"
    TZ            = "Europe/Moscow"
    }, each.key == "api" ? {
    # Рантайм собирает функцию плагином к своему main-модулю, и дефолт GODEBUG
    # там — старый ServeMux: шаблоны «GET /health» не матчатся, любой запрос
    # получает 404. Настройка читается при старте процесса, из кода её не
    # поменять. api.CheckMux не даст стартовать без неё.
    GODEBUG              = "httpmuxgo121=0"
    CORS_ALLOWED_ORIGINS = var.cors_allowed_origins
    LLM_BASE_URL         = "https://polza.ai/api/v1"
    LLM_MODEL            = var.llm_model
  } : {})

  service_account_id = yandex_iam_service_account.functions.id

  dynamic "secrets" {
    for_each = [for k in local.function_secrets[each.key] : k if contains(keys(local.secret_values), k)]
    content {
      id                   = yandex_lockbox_secret.app.id
      version_id           = yandex_lockbox_secret_version.app[0].id
      key                  = secrets.value
      environment_variable = secrets.value
    }
  }

  # Код функции выкладывает GitHub Actions при слиянии в master, а не Terraform.
  # Без этого каждый `tofu apply` откатывал бы функцию на ту версию, что собрана
  # из рабочей копии человека, затирая выложенное пайплайном.
  # Terraform владеет существованием функции и её настройками, CI — кодом.
  lifecycle {
    ignore_changes = [user_hash, content]
  }

  depends_on = [yandex_resourcemanager_folder_iam_member.functions_lockbox]
}

# Снаружи и без IAM-авторизации ходит только бот (вебхук MAX). api вызывает
# API Gateway, воркеров — таймер; публичный доступ им не нужен.
# Требует роли admin: с одним editor падает с PermissionDenied.
resource "yandex_function_iam_binding" "public" {
  for_each = { for k, v in local.functions : k => v if v.public }

  function_id = yandex_function.fn[each.key].id
  role        = "serverless.functions.invoker"
  members     = ["system:allUsers"]
}
