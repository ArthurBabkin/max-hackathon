terraform {
  required_version = ">= 1.10"

  # Состояние в Object Storage, а не на ноутбуке: применять должен уметь
  # любой в команде, а не один человек. Бакет создан отдельно и Terraform
  # не управляется — иначе он попытался бы удалить хранилище собственного
  # состояния. Команда создания — в README.
  backend "s3" {
    endpoints = {
      s3 = "https://storage.yandexcloud.net"
    }
    bucket = "traektoria-tfstate"
    key    = "traektoria/terraform.tfstate"
    region = "ru-central1"

    # Блокировка средствами самого S3: DynamoDB, на которую рассчитан
    # классический вариант, в Yandex Cloud нет.
    use_lockfile = true

    # Object Storage совместим с S3, но не с проверками, специфичными для AWS.
    skip_region_validation      = true
    skip_credentials_validation = true
    skip_requesting_account_id  = true
    skip_metadata_api_check     = true
    skip_s3_checksum            = true
  }

  required_providers {
    yandex = {
      source  = "yandex-cloud/yandex"
      version = "~> 0.127"
    }
    archive = {
      source  = "hashicorp/archive"
      version = "~> 2.4"
    }
  }
}
