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

variable "pg_preset" {
  description = <<-EOT
    Класс хоста PostgreSQL. b2.medium — 2 ядра (burstable) и 4 ГБ, самый дешёвый
    вменяемый вариант. Выделенные ядра начинаются с c3-c2-m4 и стоят заметно дороже.
  EOT
  type        = string
  default     = "b2.medium"
}

variable "pg_disk_size" {
  description = "Размер диска в ГБ"
  type        = number
  default     = 20
}

variable "pg_host_count" {
  description = <<-EOT
    Число хостов. Один дешевле; два снимают риск техработ на окне проверки
    30.09-14.10, когда решение должно быть доступно непрерывно.
  EOT
  type        = number
  default     = 1
}

variable "pg_password" {
  description = "Пароль пользователя БД. Задаётся через TF_VAR_pg_password, в git не попадает."
  type        = string
  sensitive   = true
  default     = ""
}

variable "enable_workers" {
  description = <<-EOT
    Поднимать ли api, reminders, content-notifier и таймеры к ним.
    Выключено, пока в них нет логики: лишние ресурсы только мешают читать план.
    Сами функции в простое бесплатны, но таймеры дёргают их вхолостую каждые 15 минут.
  EOT
  type        = bool
  default     = false
}

variable "enable_database" {
  description = <<-EOT
    Поднимать ли кластер Managed PostgreSQL.
    ВЫКЛЮЧЕНО НАМЕРЕННО: это единственный ресурс, который тарифицируется
    круглосуточно независимо от нагрузки. Включать, когда дойдёт до работы с БД,
    и обязательно держать включённым всё окно проверки 30.09-14.10.
  EOT
  type        = bool
  default     = false
}

# --- Приложение ---------------------------------------------------------------
# Секреты задаются только через TF_VAR_* и уходят в Lockbox (secrets.tf), а не
# в переменные окружения версии функции: там их видно в консоли каждому, кто
# может смотреть функцию.

variable "max_bot_token" {
  description = "Токен бота MAX (TF_VAR_max_bot_token). Им же проверяется подпись initData."
  type        = string
  sensitive   = true
  default     = ""
}

variable "webhook_secret" {
  description = "Секрет вебхука MAX, [a-zA-Z0-9_-]{5,256} (TF_VAR_webhook_secret)"
  type        = string
  sensitive   = true
  default     = ""
}

variable "jwt_secret" {
  description = "Ключ подписи JWT сессий, не короче 32 байт: openssl rand -hex 32 (TF_VAR_jwt_secret)"
  type        = string
  sensitive   = true
  default     = ""
}

variable "polza_ai_api_key" {
  description = "Ключ polza.ai для помощника (TF_VAR_polza_ai_api_key). Пусто — помощник отвечает шаблоном «данных нет»."
  type        = string
  sensitive   = true
  default     = ""
}

variable "database_url" {
  description = <<-EOT
    Строка подключения, если база не из этого конфига (TF_VAR_database_url).
    При enable_database = true берётся из кластера в postgres.tf.
  EOT
  type        = string
  sensitive   = true
  default     = ""
}

variable "max_bot_name" {
  description = "Ник бота для ссылок https://max.ru/<ник>?start=..."
  type        = string
  default     = "t356_hakaton_max_bot"
}

variable "max_bot_id" {
  description = "user_id бота (GET /me) — contact_id кнопки open_app"
  type        = string
  default     = "426643746"
}

variable "reminder_hour" {
  description = "Час напоминаний по местному времени ученика (ТЗ §6.3)"
  type        = string
  default     = "10"
}

variable "cors_allowed_origins" {
  description = "Откуда мини-приложение ходит в api, через запятую"
  type        = string
  default     = "https://traektoria.website.yandexcloud.net"
}

variable "llm_model" {
  description = "Модель помощника в polza.ai"
  type        = string
  default     = "GigaChat/GigaChat-3-Pro"
}
