variable "cloud_id" {
  description = "Идентификатор облака"
  type        = string
  default     = "b1g3r44i678j8024470s" # cloud-chinook-330
}

variable "folder_id" {
  description = "Идентификатор каталога"
  type        = string
  default     = "b1gsauq7gtvp76o9jil0" # default
}

variable "zone" {
  description = "Зона доступности по умолчанию"
  type        = string
  default     = "ru-central1-a"
}

variable "domain" {
  description = "Собственный домен лендинга"
  type        = string
  default     = "traektoriaedu.ru"
}

variable "fallback_bucket" {
  description = <<-EOT
    Бакет на адресе Yandex Cloud. Именно он отдан организаторам как WEBAPP_URL,
    потому что HTTPS на нём работает сразу, без домена и сертификата.
  EOT
  type        = string
  default     = "traektoria"
}

variable "enable_domain_https" {
  description = <<-EOT
    Привязать сертификат к бакету собственного домена.
    Включать только после того, как сертификат перешёл в статус ISSUED,
    иначе apply упадёт. Проверка:
    yc certificate-manager certificate get --id <id> --format json | grep status
  EOT
  type        = bool
  default     = false
}

variable "max_api_base" {
  description = "Базовый адрес Bot API MAX"
  type        = string
  default     = "https://platform-api2.max.ru"
}
