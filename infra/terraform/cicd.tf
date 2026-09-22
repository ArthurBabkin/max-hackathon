# Сервисный аккаунт для GitHub Actions.
#
# Права выданы узко: выкладывать версии функций и заливать статику в бакеты.
# Создавать и удалять инфраструктуру пайплайн не может намеренно — `tofu apply`
# выполняет человек. В окне проверки 30.09-14.10 автоматическое изменение
# инфраструктуры от случайного пуша в master стоило бы слишком дорого.
resource "yandex_iam_service_account" "cicd" {
  name        = "traektoria-cicd"
  description = "Выкладка из GitHub Actions"
}

# Версии функций: создать новую версию и переключить на неё $latest.
resource "yandex_resourcemanager_folder_iam_member" "cicd_functions" {
  folder_id = var.folder_id
  role      = "functions.editor"
  member    = "serviceAccount:${yandex_iam_service_account.cicd.id}"
}

# Заливка собранного фронта в бакет.
resource "yandex_resourcemanager_folder_iam_member" "cicd_storage" {
  folder_id = var.folder_id
  role      = "storage.editor"
  member    = "serviceAccount:${yandex_iam_service_account.cicd.id}"
}

# Авторизованный ключ: им yc CLI в пайплайне логинится для выкладки функций.
resource "yandex_iam_service_account_key" "cicd" {
  service_account_id = yandex_iam_service_account.cicd.id
  description        = "GitHub Actions: секрет YC_SA_KEY"
}

# Статические ключи: ими aws cli заливает статику по S3-совместимому API.
resource "yandex_iam_service_account_static_access_key" "cicd" {
  service_account_id = yandex_iam_service_account.cicd.id
  description        = "GitHub Actions: секреты YC_STORAGE_*"
}
