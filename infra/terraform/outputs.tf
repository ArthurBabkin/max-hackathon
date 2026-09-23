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
  value       = var.enable_database ? local.database_url : null
}

# --- Для прогона Ansible ------------------------------------------------------
# Читать по одному, через -raw: см. предупреждение ниже про stdout.

output "db_host" {
  description = "Адрес ВМ с базой — сюда ходит Ansible и сюда же резолвится db.<домен>"
  value       = try(yandex_vpc_address.db[0].external_ipv4_address[0].address, null)
}

output "db_backup_access_key" {
  description = "Идентификатор ключа для выгрузки бэкапов в бакет (переменная Ansible)"
  sensitive   = true
  value       = try(yandex_iam_service_account_static_access_key.db[0].access_key, null)
}

output "db_backup_secret_key" {
  description = "Секретный ключ для выгрузки бэкапов (переменная Ansible)"
  sensitive   = true
  value       = try(yandex_iam_service_account_static_access_key.db[0].secret_key, null)
}

# ---------------------------------------------------------------------------
# Значения для секретов GitHub Actions. Читать по одному:
#   tofu output -raw github_secret_yc_sa_key
# В терминал целиком не выводить: `tofu output` без -raw покажет все сразу.
# ---------------------------------------------------------------------------

output "github_secret_yc_sa_key" {
  description = "Секрет YC_SA_KEY — авторизованный ключ сервисного аккаунта CI"
  sensitive   = true
  value = jsonencode({
    id                 = yandex_iam_service_account_key.cicd.id
    service_account_id = yandex_iam_service_account.cicd.id
    created_at         = yandex_iam_service_account_key.cicd.created_at
    key_algorithm      = "RSA_2048"
    public_key         = yandex_iam_service_account_key.cicd.public_key
    private_key        = yandex_iam_service_account_key.cicd.private_key
  })
}

output "github_secret_yc_storage_access_key" {
  description = "Секрет YC_STORAGE_ACCESS_KEY"
  sensitive   = true
  value       = yandex_iam_service_account_static_access_key.cicd.access_key
}

output "github_secret_yc_storage_secret_key" {
  description = "Секрет YC_STORAGE_SECRET_KEY"
  sensitive   = true
  value       = yandex_iam_service_account_static_access_key.cicd.secret_key
}

output "github_secret_yc_infra_key" {
  description = "Секрет YC_INFRA_KEY — ключ сервисного аккаунта для применения инфраструктуры"
  sensitive   = true
  value = jsonencode({
    id                 = yandex_iam_service_account_key.infra.id
    service_account_id = yandex_iam_service_account.infra.id
    created_at         = yandex_iam_service_account_key.infra.created_at
    key_algorithm      = "RSA_2048"
    public_key         = yandex_iam_service_account_key.infra.public_key
    private_key        = yandex_iam_service_account_key.infra.private_key
  })
}
