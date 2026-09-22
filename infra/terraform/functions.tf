# Все четыре функции живут в одном Go-модуле, поэтому zip у них общий:
# отличаются только точкой входа. Лишнее вырезаем — в архив должны попасть
# только исходники, иначе туда уедут датасеты, экраны и история git.
data "archive_file" "backend" {
  type        = "zip"
  source_dir  = "${path.module}/../.."
  output_path = "${path.module}/.build/backend.zip"

  excludes = [
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
      timeout     = 10
      public      = true
    }
    api = {
      name        = "traektoria-api"
      description = "REST мини-приложения"
      entrypoint  = "apps/api/cmd/function/main.Handler"
      memory      = 256
      timeout     = 30
      public      = true
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

  environment = {
    MAX_API_BASE = var.max_api_base
  }
}

# Снаружи и без IAM-авторизации ходят только бот (вебхук MAX) и api (браузер).
# Воркеров дёргает таймер, публичный доступ им не нужен.
# Требует роли admin: с одним editor падает с PermissionDenied.
resource "yandex_function_iam_binding" "public" {
  for_each = { for k, v in local.functions : k => v if v.public }

  function_id = yandex_function.fn[each.key].id
  role        = "serverless.functions.invoker"
  members     = ["system:allUsers"]
}
