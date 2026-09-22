output "webhook_url" {
  description = "Адрес вебхука для POST /subscriptions в MAX"
  value       = "https://functions.yandexcloud.net/${yandex_function.bot.id}"
}

output "webapp_url" {
  description = "Адрес мини-приложения, который передан организаторам"
  value       = "https://${yandex_storage_bucket.fallback.bucket}.website.yandexcloud.net/"
}

output "landing_url" {
  description = "Лендинг на собственном домене (работает после выпуска сертификата)"
  value       = "https://${var.domain}/"
}

output "certificate_status" {
  description = "Статус сертификата: ISSUED — можно включать enable_domain_https"
  value       = yandex_cm_certificate.domain.status
}
