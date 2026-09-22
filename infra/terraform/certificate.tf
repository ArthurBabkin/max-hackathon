# Бесплатный сертификат Let's Encrypt. Проверка владения — через DNS,
# запись создаётся в dns.tf из вычисленного здесь challenge.
# Сертификат останется в статусе VALIDATING, пока домен не делегирован
# на ns1.yandexcloud.net / ns2.yandexcloud.net и делегирование не разошлось.
resource "yandex_cm_certificate" "domain" {
  name    = "traektoriaedu"
  domains = [var.domain]

  managed {
    challenge_type = "DNS_CNAME"
  }
}
