output "webhook_url" {
  description = "Адрес вебхука для POST /subscriptions в MAX"
  value       = "https://functions.yandexcloud.net/${yandex_function.fn["bot"].id}"
}

output "api_url" {
  description = "Базовый адрес REST для мини-приложения, VITE_API_BASE (при enable_workers)"
  value       = try("https://${yandex_api_gateway.api[0].domain}/api/v1", null)
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

output "database_url" {
  description = "Строка подключения к PostgreSQL. Кладётся в Lockbox как DATABASE_URL."
  sensitive   = true
  value = var.enable_database ? format(
    "postgres://%s:%s@%s:6432/%s?sslmode=require",
    yandex_mdb_postgresql_user.app[0].name,
    var.pg_password,
    try(yandex_mdb_postgresql_cluster.main[0].host[0].fqdn, ""),
    yandex_mdb_postgresql_database.main[0].name,
  ) : null
}
