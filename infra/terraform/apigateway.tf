# REST мини-приложения ходит через API Gateway, а не напрямую в функцию.
#
# Прямой вызов https://functions.yandexcloud.net/<id> пути не передаёт:
# «Cloud Functions не поддерживает пути в запросах. Для корректной работы
# http.ServeMux функцию нужно вызывать через API-шлюз»
# (yandex.cloud/ru/docs/functions/lang/golang/handler). У контракта 25 операций
# на двух десятках путей — без шлюза роутер api увидел бы только «/».
#
# Один жадный маршрут на всё под /api/v1: роутинг остаётся в Go, спецификация
# шлюза не дублирует контракт и не расходится с ним. OPTIONS тоже уходит в
# функцию — CORS она отрабатывает сама по CORS_ALLOWED_ORIGINS.
resource "yandex_api_gateway" "api" {
  count = var.enable_workers ? 1 : 0

  name        = "traektoria-api"
  description = "REST мини-приложения «Траектория»"

  spec = <<-EOT
    openapi: 3.0.0
    info:
      title: traektoria-api
      version: 1.0.0
    paths:
      /api/v1/{path+}:
        x-yc-apigateway-any-method:
          parameters:
            - name: path
              in: path
              required: true
              schema:
                type: string
          x-yc-apigateway-integration:
            type: cloud_functions
            function_id: ${yandex_function.fn["api"].id}
            tag: "$latest"
            service_account_id: ${yandex_iam_service_account.invoker[0].id}
  EOT

  depends_on = [yandex_resourcemanager_folder_iam_member.invoker]
}
