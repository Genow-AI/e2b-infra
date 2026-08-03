locals {
  // Subdomains served by the ingress load balancer (a separate LB from the main
  // one behind the wildcard record), so each needs its own A record below.
  ingress_subdomains = ["grpc-api", "dashboard-api"]
}

# Existing Cloud DNS zone in the development-root project (all Genow DNS is
# managed there). Records for the e2b subdomain (e.g. *.e2b-sandbox.genow.cloud)
# are written directly into this parent zone — no dedicated sub-zone/delegation.
data "google_dns_managed_zone" "zone" {
  name    = var.dns_zone_name
  project = var.dns_project_id
}

# Certificate Manager DNS-authorization record: proves domain ownership so the
# managed wildcard cert can be issued. Name/type/data are provided by GCP.
resource "google_dns_record_set" "dns_auth" {
  project      = var.dns_project_id
  managed_zone = data.google_dns_managed_zone.zone.name
  name         = google_certificate_manager_dns_authorization.dns_auth.dns_resource_record[0].name
  type         = google_certificate_manager_dns_authorization.dns_auth.dns_resource_record[0].type
  ttl          = 3600
  rrdatas      = [google_certificate_manager_dns_authorization.dns_auth.dns_resource_record[0].data]
}

# Wildcard A record -> main HTTPS load balancer. Covers api.<domain>,
# docker.<domain>, nomad.<domain>, and every <sandbox>.<domain> host.
resource "google_dns_record_set" "a_star" {
  project      = var.dns_project_id
  managed_zone = data.google_dns_managed_zone.zone.name
  name         = "*.${var.domain_name}."
  type         = "A"
  ttl          = 300
  rrdatas      = [google_compute_global_forwarding_rule.https.ip_address]
}

# grpc-api / dashboard-api A records -> ingress load balancer. NOT covered by the
# wildcard above: that points at the main LB's IP, these must resolve to the
# ingress LB's own IP. Being more specific, they win over the wildcard.
resource "google_dns_record_set" "ingress" {
  for_each     = toset(local.ingress_subdomains)
  project      = var.dns_project_id
  managed_zone = data.google_dns_managed_zone.zone.name
  name         = "${each.value}.${var.domain_name}."
  type         = "A"
  ttl          = 300
  rrdatas      = [google_compute_global_forwarding_rule.ingress.ip_address]
}
