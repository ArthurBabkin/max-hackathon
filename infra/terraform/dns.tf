resource "yandex_dns_zone" "domain" {
  name   = "traektoriaedu"
  zone   = "${var.domain}."
  public = true
}

# На апексе нельзя использовать CNAME, а хардкодить IP Object Storage опасно —
# он не наш и может смениться. ANAME резолвит цель и отдаёт A-запись.
resource "yandex_dns_recordset" "apex" {
  zone_id = yandex_dns_zone.domain.id
  name    = "${var.domain}."
  type    = "ANAME"
  ttl     = 600
  data    = ["${var.domain}.website.yandexcloud.net."]
}

# Подтверждение владения доменом для Let's Encrypt.
# Значение берём из самого сертификата: при продлении оно не меняется,
# поэтому запись заводится один раз.
resource "yandex_dns_recordset" "acme" {
  zone_id = yandex_dns_zone.domain.id
  name    = yandex_cm_certificate.domain.challenges[0].dns_name
  type    = yandex_cm_certificate.domain.challenges[0].dns_type
  ttl     = 600
  data    = [yandex_cm_certificate.domain.challenges[0].dns_value]
}
