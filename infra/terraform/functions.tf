# Код функции упаковывается прямо здесь, поэтому `tofu apply` выкладывает и
# инфраструктуру, и версию функции — отдельный шаг деплоя не нужен.
data "archive_file" "bot" {
  type        = "zip"
  source_dir  = "${path.module}/../../apps/bot"
  output_path = "${path.module}/.build/bot.zip"
}

resource "yandex_function" "bot" {
  name        = "traektoria-bot"
  description = "MAX webhook (Траектория)"

  runtime = "golang123"

  # Формат entrypoint — <путь к каталогу>/<имя пакета>.<Функция>.
  # Вариант cmd/function.Handler не собирается: сборщик ищет пакет в ./cmd.
  entrypoint = "cmd/function/main.Handler"

  memory            = 128
  execution_timeout = 10

  user_hash = data.archive_file.bot.output_base64sha256

  content {
    zip_filename = data.archive_file.bot.output_path
  }

  environment = {
    MAX_API_BASE = var.max_api_base
  }
}

# MAX ходит на вебхук снаружи и без IAM-авторизации.
# Эквивалент `yc serverless function allow-unauthenticated-invoke`.
# Требует роли admin: с одним editor апдейт упадёт с PermissionDenied.
resource "yandex_function_iam_binding" "bot_public" {
  function_id = yandex_function.bot.id
  role        = "serverless.functions.invoker"
  members     = ["system:allUsers"]
}
