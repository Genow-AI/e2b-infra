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
