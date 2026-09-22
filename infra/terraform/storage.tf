# Object Storage управляется не через IAM-токен, а через S3-совместимый API,
# которому нужны статические ключи сервисного аккаунта. Поэтому аккаунт и ключ
# описаны здесь же: руками секрет заводить не нужно, он создаётся тем же apply.
resource "yandex_iam_service_account" "storage" {
  name        = "traektoria-storage"
  description = "Управление бакетами из Terraform"
}

resource "yandex_resourcemanager_folder_iam_member" "storage_admin" {
  folder_id = var.folder_id
  role      = "storage.admin"
  member    = "serviceAccount:${yandex_iam_service_account.storage.id}"
}

resource "yandex_iam_service_account_static_access_key" "storage" {
  service_account_id = yandex_iam_service_account.storage.id
  description        = "S3-ключ для Terraform"
}

# Бакет на адресе Yandex Cloud: https://traektoria.website.yandexcloud.net/
# HTTPS работает из коробки по wildcard-сертификату — при условии, что в имени
# бакета нет точек (wildcard покрывает только один уровень поддомена).
# Именно этот адрес отдан организаторам как WEBAPP_URL.
resource "yandex_storage_bucket" "fallback" {
  bucket = var.fallback_bucket

  access_key = yandex_iam_service_account_static_access_key.storage.access_key
  secret_key = yandex_iam_service_account_static_access_key.storage.secret_key

  anonymous_access_flags {
    read = true
    list = false
  }

  website {
    index_document = "index.html"

    # У мини-аппа клиентский роутинг, поэтому неизвестные пути отдаём в SPA.
    # Отдаётся тело index.html, но со статусом 404, а не 200.
    error_document = "index.html"
  }
}

# Бакет собственного домена. Имя ОБЯЗАНО совпадать с доменом — иначе привязать
# домен невозможно, а переименовать бакет нельзя.
resource "yandex_storage_bucket" "domain" {
  bucket = var.domain

  access_key = yandex_iam_service_account_static_access_key.storage.access_key
  secret_key = yandex_iam_service_account_static_access_key.storage.secret_key

  anonymous_access_flags {
    read = true
    list = false
  }

  website {
    index_document = "index.html"
    error_document = "index.html"
  }

  # Имя с точками не покрывается wildcard-сертификатом Yandex Cloud,
  # поэтому здесь HTTPS возможен только со своим сертификатом.
  dynamic "https" {
    for_each = var.enable_domain_https ? [1] : []
    content {
      certificate_id = yandex_cm_certificate.domain.id
    }
  }
}
