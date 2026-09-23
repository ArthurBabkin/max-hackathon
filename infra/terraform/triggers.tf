# От имени этого аккаунта таймеры вызывают функции.
resource "yandex_iam_service_account" "invoker" {
  count = var.enable_workers ? 1 : 0

  name        = "traektoria-invoker"
  description = "Вызов функций из Timer-триггеров"
}

resource "yandex_resourcemanager_folder_iam_member" "invoker" {
  count = var.enable_workers ? 1 : 0

  folder_id = var.folder_id
  role      = "serverless.functions.invoker"
  member    = "serviceAccount:${yandex_iam_service_account.invoker[0].id}"
}

# Cron в Yandex Cloud — ШЕСТЬ полей: Минуты Часы День-месяца Месяц День-недели Год.
# Время в UTC. `?` = «любое значение», ставится в поле дня месяца или дня недели.
# Не перепутать с REMINDER_HOUR в коде: там местное время ученика, здесь UTC.

resource "yandex_function_trigger" "reminders" {
  count = var.enable_workers ? 1 : 0

  name        = "traektoria-reminders"
  description = "Напоминания: раз в 15 минут, воркер сам решает, кому пора"

  timer {
    cron_expression = "0/15 * ? * * *"
  }

  function {
    id                 = yandex_function.fn["reminders"].id
    service_account_id = yandex_iam_service_account.invoker[0].id
    retry_attempts     = 3
    retry_interval     = 30
  }

  depends_on = [yandex_resourcemanager_folder_iam_member.invoker]
}

resource "yandex_function_trigger" "notifier" {
  count = var.enable_workers ? 1 : 0

  name        = "traektoria-content-notifier"
  description = "Уведомления об изменениях контента: раз в сутки в 10:00 МСК"

  # 07:00 UTC = 10:00 МСК
  timer {
    cron_expression = "0 7 ? * * *"
  }

  function {
    id                 = yandex_function.fn["notifier"].id
    service_account_id = yandex_iam_service_account.invoker[0].id
    # Без повторов один сбой базы в 10:00 стоил бы суток тишины: следующий
    # запуск только завтра. Повтор безопасен — воркер забирает события
    # транзакцией, и заново их уже не выдадут. Повтор делается после
    # неуспешного завершения вызова, так что запуски не наложатся.
    retry_attempts = 3
    retry_interval = 30
  }

  depends_on = [yandex_resourcemanager_folder_iam_member.invoker]
}
