# Dedicated subnet for e2b cluster nodes inside the EXISTING genow VPC, so nodes
# can reach the private Cloud SQL (10.1.0.3) over the VPC. The genow VPC is
# custom-mode, so an explicit subnet is required; every nodepool network_interface
# references it (module.cluster.subnetwork -> var.subnetwork in the child modules).

variable "e2b_subnet_cidr" {
  type        = string
  default     = "10.60.0.0/24"
  description = "Primary CIDR for the e2b node subnet; must be disjoint from platform (10.0.0.0/24, 10.0.2.0/28) and Cloud SQL (10.1.0.x) ranges."
}

resource "google_compute_subnetwork" "e2b_nodes" {
  name                     = "e2b-nodes"
  project                  = var.gcp_project_id
  region                   = var.gcp_region
  network                  = var.network_name
  ip_cidr_range            = var.e2b_subnet_cidr
  private_ip_google_access = true
}

# BYOC gap fix: e2b's network module (nomad-cluster/network/main.tf) only opens
# LB->node and IAP->node ingress — it ASSUMES the VPC already permits node-to-node
# traffic (true in GCP's `default` network via the implicit allow-internal rule, and
# in e2b's own VPC modules, but NOT in this shared custom VPC). Without this rule the
# Consul/Nomad agents can't gossip (serf 8301/8302, RPC 8300/4647, HTTP 8500/4646,
# etc.), so they only ever "join" themselves -> no quorum -> no leader -> Nomad never
# starts -> the cluster never forms. Allow all traffic between e2b nodes in the subnet.
resource "google_compute_firewall" "e2b_orch_internal" {
  name          = "e2b-orch-internal-allow"
  project       = var.gcp_project_id
  network       = var.network_name
  direction     = "INGRESS"
  priority      = 900
  source_ranges = [var.e2b_subnet_cidr] # 10.60.0.0/24 — all e2b nodes
  target_tags   = ["orch"]              # e2b's cluster_tag_name

  allow { protocol = "tcp" }
  allow { protocol = "udp" }
  allow { protocol = "icmp" }
}
