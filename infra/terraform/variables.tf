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

variable "enable_browser_demo" {
  description = <<-EOT
    ВРЕМЕННО: мини-приложение открывается в обычном браузере без MAX, от лица
    демо-семьи Артёма и Ольги. api принимает неподписанный вход только для id
    дев-диапазона 900000000–900000999 — настоящих пользователей MAX это не
    затрагивает, их вход по-прежнему только с подписью. Выключить до 30.09,
    к окну проверки. Демо-данные заливает и убирает Deploy functions.
  EOT
  type        = bool
  default     = false
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

variable "db_cores" {
  description = "Ядер у ВМ с базой. У standard-v3 при доле 20% допустимы 2 или 4."
  type        = number
  default     = 2
}

variable "db_core_fraction" {
  description = <<-EOT
    Доля гарантированного ядра в процентах. 20 — минимум для standard-v3 и самый
    дешёвый вариант: всплески обслуживаются сверх неё. Нагрузка MVP — единицы
    запросов в секунду, поэтому доли хватает.
  EOT
  type        = number
  default     = 20
}

variable "db_memory" {
  description = "Память ВМ в ГБ. При 2 ядрах допустимо от 1 до 8; 2 ГБ хватает, чтобы база целиком легла в страничный кеш."
  type        = number
  default     = 2
}

variable "db_data_disk_type" {
  description = <<-EOT
    Тип диска под данные. network-ssd дороже network-hdd, но у HDD задержка
    fsync такова, что каждая фиксация транзакции ощутима. Диск маленький,
    поэтому разница в деньгах невелика.
  EOT
  type        = string
  default     = "network-ssd"
}

variable "db_data_disk_size" {
  description = "Размер диска под данные в ГБ"
  type        = number
  default     = 10
}

variable "ssh_public_key" {
  description = <<-EOT
    Открытый SSH-ключ для пользователя ubuntu на ВМ с базой: им ходит Ansible.
    Задаётся через TF_VAR_ssh_public_key, например
    export TF_VAR_ssh_public_key="$(cat ~/.ssh/id_ed25519.pub)".
  EOT
  type        = string
  default     = ""
}

variable "ssh_allowed_cidrs" {
  description = <<-EOT
    Откуда разрешён SSH к ВМ с базой. По умолчанию пусто — порт не открыт
    никому, и случайный apply не выставит его в интернет. Для прогона Ansible
    указать свой адрес: -var 'ssh_allowed_cidrs=["1.2.3.4/32"]'.
  EOT
  type        = list(string)
  default     = []
}

variable "db_name" {
  description = <<-EOT
    Имя базы и роли приложения в PostgreSQL. Отсюда же его берёт Ansible —
    значение должно быть одно, иначе строка подключения разойдётся с тем,
    что реально заведено на ВМ.
  EOT
  type        = string
  default     = "traektoria"
}

variable "backup_bucket" {
  description = "Бакет для дампов базы. Имя без точек: иначе не работает HTTPS по wildcard-сертификату."
  type        = string
  default     = "traektoria-db-backups"
}

variable "backup_retention_days" {
  description = "Сколько дней хранить дампы. Окно проверки 30.09-14.10 покрывается с запасом."
  type        = number
  default     = 30
}

variable "pg_password" {
  description = <<-EOT
    Пароль пользователя БД (TF_VAR_pg_password). В git не попадает.
    Тот же пароль получает Ansible — он и заводит роль в PostgreSQL.
  EOT
  type        = string
  sensitive   = true
  default     = ""
}

variable "enable_workers" {
  description = <<-EOT
    Поднимать ли api с API Gateway, reminders, content-notifier и таймеры к ним.
    Без базы (enable_database) им не с чем работать, поэтому включаются вместе:
    -var enable_database=true -var enable_workers=true.
  EOT
  type        = bool
  default     = false
}

variable "enable_database" {
  description = <<-EOT
    Поднимать ли ВМ с PostgreSQL, диск под данные, адрес и бакет для бэкапов.
    ВЫКЛЮЧЕНО НАМЕРЕННО: ВМ тарифицируется круглосуточно независимо от нагрузки.
    Включать, когда дойдёт до работы с БД, и обязательно держать включённым всё
    окно проверки 30.09-14.10. После apply настройку СУБД катит Ansible:
    см. infra/ansible/README.md.
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
  description = "Секрет вебхука MAX, [a-zA-Z0-9_-]{5,256} (TF_VAR_webhook_secret). Без него функция бота не стартует"
  type        = string
  sensitive   = true
  default     = ""

  validation {
    condition     = var.webhook_secret == "" || can(regex("^[a-zA-Z0-9_-]{5,256}$", var.webhook_secret))
    error_message = "Секрет вебхука: 5–256 символов из [a-zA-Z0-9_-] — маска платформы MAX."
  }
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
    При enable_database = true собирается из ВМ в postgres.tf.
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
