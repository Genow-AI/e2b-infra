locals {
  subdomains = ["grpc-api", "dashboard-api"]
}

resource "google_compute_health_check" "ingress" {
  name = "${var.prefix}ingress"

  timeout_sec         = 3
  check_interval_sec  = 5
  healthy_threshold   = 2
  unhealthy_threshold = 2

  http_health_check {
    port         = var.ingress_port.port
    request_path = var.ingress_port.health_path
  }
}

resource "google_compute_backend_service" "ingress" {
  name = "${var.prefix}ingress"

  protocol  = "HTTP"
  port_name = var.ingress_port.name

  session_affinity = null
  health_checks    = [google_compute_health_check.ingress.id]

  timeout_sec                     = 86400
  connection_draining_timeout_sec = 1

  load_balancing_scheme = "EXTERNAL_MANAGED"
  locality_lb_policy    = "ROUND_ROBIN"

  security_policy = google_compute_security_policy.ingress.id

  backend {
    group = var.api_instance_group
  }
}

resource "google_compute_backend_service" "h2c_ingress" {
  name = "${var.prefix}h2c-ingress"

  protocol  = "H2C"
  port_name = var.ingress_port.name

  session_affinity = null
  health_checks    = [google_compute_health_check.ingress.id]

  timeout_sec                     = 86400
  connection_draining_timeout_sec = 1

  load_balancing_scheme = "EXTERNAL_MANAGED"
  locality_lb_policy    = "ROUND_ROBIN"

  security_policy = google_compute_security_policy.ingress.id

  backend {
    group = var.api_instance_group
  }
}

resource "google_compute_security_policy" "ingress" {
  name = "${var.prefix}ingress"

  adaptive_protection_config {
    layer_7_ddos_defense_config {
      enable = true
    }
  }
}

resource "google_compute_url_map" "ingress" {
  name            = "${var.prefix}ingress"
  default_service = google_compute_backend_service.ingress.self_link

  host_rule {
    hosts        = concat(["grpc-api.${var.domain_name}"], [for d in var.additional_domains : "grpc-api.${d}"])
    path_matcher = "grpc-api-paths"
  }

  path_matcher {
    name            = "grpc-api-paths"
    default_service = google_compute_backend_service.h2c_ingress.self_link
  }
}

resource "google_compute_global_forwarding_rule" "ingress" {
  name                  = "${var.prefix}ingress-forward-http"
  ip_protocol           = "TCP"
  port_range            = "443"
  load_balancing_scheme = "EXTERNAL_MANAGED"
  ip_address            = google_compute_global_address.ingress_ipv4.address
  target                = google_compute_target_https_proxy.ingress.self_link
}

resource "google_compute_global_address" "ingress_ipv4" {
  name       = "${var.prefix}ingress-ipv4"
  ip_version = "IPV4"
}

resource "google_compute_ssl_policy" "ingress" {
  name            = "${var.prefix}ingress-ssl-policy"
  profile         = "MODERN"
  min_tls_version = "TLS_1_2"
}

resource "google_compute_target_https_proxy" "ingress" {
  name    = "${var.prefix}ingress-https"
  url_map = google_compute_url_map.ingress.self_link

  ssl_policy = google_compute_ssl_policy.ingress.self_link

  certificate_map = "//certificatemanager.googleapis.com/${google_certificate_manager_certificate_map.certificate_map.id}"
}

# grpc-api / dashboard-api A records -> ingress load balancer.
# More specific than the wildcard, so they take precedence for these hosts.
resource "google_dns_record_set" "ingress" {
  for_each     = toset(local.subdomains) # ["grpc-api", "dashboard-api"]
  managed_zone = data.google_dns_managed_zone.zone.name
  name         = "${each.value}.${var.domain_name}."
  type         = "A"
  ttl          = 300
  rrdatas      = [google_compute_global_forwarding_rule.ingress.ip_address]
}
