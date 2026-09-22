# Токен берётся из окружения: export YC_TOKEN=$(yc iam create-token).
# В конфигурацию он не попадает намеренно — иначе утечёт в git и в state.
provider "yandex" {
  cloud_id  = var.cloud_id
  folder_id = var.folder_id
  zone      = var.zone
}
