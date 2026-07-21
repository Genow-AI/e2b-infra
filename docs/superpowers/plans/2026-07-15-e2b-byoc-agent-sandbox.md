# e2b BYOC Agent Sandbox on GCP — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Created:** 2026-07-15 · **Revised:** 2026-07-21 (fork code changes implemented; feasibility gate passed — see "Current status")

**Goal:** Stand up a self-hosted [e2b](https://github.com/e2b-dev/infra) sandbox in the existing central GCP project `gcp-infrastructure-genow-euw3`, using e2b's own Terraform, integrated with Genow's existing VPC / DNS / Cloud SQL, and prove it works by spawning one hello-world sandbox through the e2b SDK.

**Architecture:** Fork `e2b-dev/infra` → `Genow-AI/e2b-infra`, pinned to `a84fe31` (tag `2026.28`). Deploy its `iac/provider-gcp` stack (Nomad + Consul + Firecracker microVMs on Compute Engine MIGs, built from a Packer image) into the **existing shared** project. Genow-specific changes vs upstream, all in the fork:

1. **Cloudflare → GCP Cloud DNS**, written **cross-project directly into the existing `genow-cloud` zone in the `development-root` DNS project** (`project = development-root`, `managed_zone = genow-cloud`) — no sub-zone, no delegation, records resolve immediately. Certificate Manager TLS stays in the deploy project.
2. **Deploy into the existing genow VPC** (`gcp-infrastructure-genow-euw3-nw-0000`) via a dedicated Terraform-managed subnet `e2b-nodes`, so nodes reach the **private** Cloud SQL over the VPC. Every nodepool `network_interface` now sets `subnetwork` (required for this custom-mode VPC).
3. **Reuse the existing Cloud SQL** instance (`…-sql-0000`, POSTGRES_15, private IP) — Terraform creates only an `e2b` database + user (generated password); the connection string is seeded to Secret Manager from a Terraform output.
4. **PoC-minimal compute** via a gitignored `.terraform.gcp.tfvars` (`network-e2b.tf`/`database.tf` are tracked).

All GCP operations run as **your own gcloud user** (which must hold the deploy-project + `development-root` DNS permissions listed in Prerequisites) — no service-account impersonation. The `genow-infrastructure` repo gains **documentation only**.

**Tech Stack:** Terraform 1.7.5, Packer 1.13.1, Go 1.26.5, gcloud, Docker + Buildx, Python 3.13 (SDK verification), GCP (Compute Engine, Cloud DNS, Certificate Manager, Cloud SQL for PostgreSQL, GCS, Secret Manager, HTTPS LB), HashiCorp Nomad/Consul, Firecracker.

---

## Source spec

`docs/superpowers/specs/2026-07-15-e2b-byoc-agent-sandbox-design.md` (GP-4108). Two spec corrections found during code discovery of the pinned fork remain in force:

- **Spec §9 was wrong that "certs need no rework."** TLS uses Certificate Manager with **DNS-authorization** challenge records; those validation records also move to Cloud DNS. The Certificate Manager resources are unchanged.
- **DNS was 100% Cloudflare**, localized to `nomad-cluster/network/{main.tf,ingress.tf}` + `cloudflare_api_token` plumbing in `init/` + `main.tf`. Swapped in Task 3.

## Current status (as of 2026-07-21)

| Area | State |
|---|---|
| Fork + pin (`Genow-AI/e2b-infra`, branch `genow/gcp-cloud-dns`, `a84fe31`) | ✅ done |
| Toolchain (Terraform 1.7.5) + **gcloud user auth** (Task 2) | ✅ done; **Go still to install** for Task 14 |
| Cloudflare → Cloud DNS swap (Task 3) | ✅ committed `e485929cf` + pushed; lock file cleaned |
| Cross-project + flat `genow-cloud` DNS (Task 3b) | ✅ implemented (`dns_project_id`/`dns_zone_name`) |
| Private-VPC integration — `e2b-nodes` subnet + `subnetwork` plumbing (Task 3c) | ✅ implemented (`network-e2b.tf` + 5 nodepools) |
| IAM + Cloud SQL Admin APIs in Terraform (Task 4) | ✅ implemented (`init/main.tf`) |
| Reuse Cloud SQL — TF db/user + generated password (Task 9) | ✅ implemented (`database.tf`) |
| PoC compute sizing (Task 10) | ✅ written, gitignored |
| **Feasibility gate (Task 7)** | ✅ **PASSED** — euw3 GO for Firecracker; quota fits; no us-west1 fallback |
| Commit the latest TF batch (3b/3c/4/9 files) | ⬜ pending — one signed commit |
| Deploy + verify (Tasks 5, 6, 11–22) | ⬜ not started |

## Parameters (confirmed)

| Parameter | Value | Notes |
|---|---|---|
| Fork repo | `Genow-AI/e2b-infra` | origin; `upstream` → e2b-dev |
| Pinned upstream commit | `a84fe313661f1b3632acca50cf0f6aa2a4484665` (tag `2026.28`) | |
| GCP project ID | `gcp-infrastructure-genow-euw3` | **EXISTING central project** — e2b deploys into it, does not create it |
| Region / zone | `europe-west3` / `europe-west3-a` | Feasibility gate **passed** (Task 7); no fallback |
| `DOMAIN_NAME` | `e2b-sandbox.genow.cloud` | `sandbox.genow.cloud` is already assigned |
| DNS project (`dns_project_id`) | **`development-root`** | e2b's DNS records are created here, not in the deploy project |
| DNS zone (`dns_zone_name`) | existing **`genow-cloud`** zone in `development-root` | records written directly into it — no sub-zone/delegation; resolves immediately |
| Deploy identity | **your gcloud user** | Terraform + gcloud + Packer run as you (Task 2). Requires admin on the deploy project + `roles/dns.admin` on `development-root` |
| Network | existing VPC `gcp-infrastructure-genow-euw3-nw-0000` + TF-managed subnet `e2b-nodes` (`10.60.0.0/24`) | so nodes reach the private Cloud SQL over the VPC |
| Cloud SQL | **reuse existing** `gcp-infrastructure-genow-euw3-sql-0000` (POSTGRES_15, private IP `10.1.0.3`) | TF creates only the `e2b` db + user (generated password); migrations run in-cluster |
| Billing account | already linked (`012E95-8DF475-160F0D`) | no billing step |
| Compute sizing | **PoC-minimal** via `.terraform.gcp.tfvars` | 1 server, ClickHouse/Loki off, `n1-standard-4` Firecracker hosts |

## Shared-project decision & coexistence (MUST respect)

Deploy into the **existing central platform project**, trading BYOC isolation for reuse of project/billing/DNS/VPC/Cloud-SQL. Verified safe against the pinned fork:

| Aspect | Finding | Consequence |
|---|---|---|
| Project creation | e2b has **no** `google_project` resource | Reuses the existing project |
| API enablement | `init/main.tf` `google_project_service` with `disable_on_destroy = false` | `make destroy` won't disable APIs the platform needs |
| Project IAM | `nomad-cluster/main.tf` uses additive `google_project_iam_member` | Won't clobber Terragrunt-managed IAM |
| Terraform state | e2b bucket `…-terraform-state` vs Terragrunt `…-tfstate` | No state collision |
| Resource naming | `prefix = "e2b-"`; buckets `…-fc-*`; subnet `e2b-nodes` | Distinct from platform resources |
| Shared VPC / Cloud SQL | e2b adds a subnet + a db/user; the VPC + SQL instance are only referenced | `make destroy` removes only e2b's subnet/db/user, never the VPC or instance |

**Guardrails:**
1. **Never delete the project or the shared VPC / Cloud SQL instance.** Teardown = **scoped `make destroy` only** (Task 22); `gcloud projects delete` is forbidden.
2. **Quota is shared** — checked in Task 7 (passed).
3. **The `e2b-nodes` subnet CIDR must not overlap** platform ranges (`10.0.0.0/24`, `10.0.2.0/28`) or the Cloud SQL PSA range (`10.1.0.x`).
4. **Review every apply/destroy plan** for unrecognized resources — two IaC tools share this project.

## Prerequisites

- [ ] Git push rights to `Genow-AI`; the fork is cloned as `$FORK` (`~/code/e2b-infra`).
- [ ] **Deploy identity — your gcloud user** (no service-account impersonation). Your user needs:
  - Admin on `gcp-infrastructure-genow-euw3` (Owner, or scoped: compute / secretmanager / certificatemanager / artifactregistry / storage / **cloudsql** admin + `roles/resourcemanager.projectIamAdmin` + `roles/serviceusage.serviceUsageAdmin`).
  - `roles/dns.admin` on `development-root` (the DNS records live there).
- [ ] `asdf`/`mise` installed (for `.tool-versions`), Docker Desktop + Buildx running.
- [ ] Commands run in `$FORK/iac/provider-gcp` unless stated. Documentation commits (Task 21) go to the `genow-infrastructure` repo.

---

## Task 1: Verify the fork and pin (done)

**Files:** none.

- [ ] **Step 1: Confirm remotes + pin**

```bash
cd ~/code/e2b-infra
git remote -v            # origin -> Genow-AI/e2b-infra, upstream -> e2b-dev/infra
git rev-parse HEAD       # descends from a84fe313…
git branch --show-current # genow/gcp-cloud-dns
```

---

## Task 2: Install toolchain + authenticate (gcloud user)

**Files:** `$FORK/.tool-versions` (read-only).

- [ ] **Step 1: Install the pinned toolchain (Go is still missing)**

```bash
cd ~/code/e2b-infra
mise install   # or: cut -d' ' -f1 .tool-versions | grep -v '^go:' | xargs -n1 asdf plugin add 2>/dev/null; asdf install
terraform version; packer version; go version; gcloud version | head -1
```
Expected: `Terraform v1.7.5`, `Packer v1.13.1`, `go version go1.26.5`, gcloud present.

- [ ] **Step 2: Authenticate as your gcloud user**

```bash
gcloud auth login                          # gcloud CLI identity
gcloud auth application-default login      # ADC for Terraform + Packer
gcloud config set project gcp-infrastructure-genow-euw3
```
If you experimented with impersonation earlier, clear it: `gcloud config unset auth/impersonate_service_account` (commands should NOT print a `WARNING: ... executed as [...]` banner). Run as the plain user — Packer's googlecompute plugin cannot read an `impersonated_service_account` ADC file.

- [ ] **Step 3: Confirm your user can reach both projects**

```bash
gcloud dns managed-zones list --project=development-root                       # DNS write target — must succeed
gcloud storage ls --project=gcp-infrastructure-genow-euw3 >/dev/null && echo "deploy-project OK"
```
Expected: both succeed. A `PERMISSION_DENIED` on `development-root` = your user lacks `roles/dns.admin` there (Task 16 would fail on the DNS records) — get it granted before deploying.

---

## Task 3: Cloudflare → GCP Cloud DNS swap (done, committed `e485929cf`)

**Files (`$FORK/iac/provider-gcp/`):** `main.tf`, `nomad-cluster/network/{main.tf,ingress.tf,variables.tf}`, `nomad-cluster/{main.tf,variables.tf}`, `init/{secrets.tf,outputs.tf}` — removed the Cloudflare provider/records/token plumbing; added `data.google_dns_managed_zone.zone` + `google_dns_record_set.{dns_auth,a_star,ingress}`.

- [ ] **Step 1: Verify (already done on this branch)**

```bash
cd ~/code/e2b-infra/iac/provider-gcp
terraform fmt -recursive && terraform init -upgrade -backend=false && terraform validate
grep -rin cloudflare . | grep -v '\.terraform'    # -> no hits (incl. lock file)
```
Expected: `validate` = "Success!"; grep clean. (Committed as `e485929cf`; the nested duplicate clone was removed.)

---

## Task 3b: DNS records go cross-project into the existing genow-cloud zone (done)

**Files:** `nomad-cluster/network/{main.tf,ingress.tf,variables.tf}`, `nomad-cluster/variables.tf`, `main.tf`, `variables.tf`, `.terraform.gcp.tfvars`.

**Why:** all Genow DNS lives in `development-root`. So the zone data source + every record set target that project (`project = var.dns_project_id`) and the existing `genow-cloud` zone (`name = var.dns_zone_name`) — no dedicated sub-zone, no delegation. The provider default project stays the deploy project (compute/certs).

- [ ] **Step 1: `dns_project_id` + `dns_zone_name` variables** added to `network/variables.tf`, `nomad-cluster/variables.tf`, and root `variables.tf` (root defaults `development-root` / `genow-cloud`).
- [ ] **Step 2: data source + record sets** in `network/{main.tf,ingress.tf}` set `project = var.dns_project_id`; the data source uses `name = var.dns_zone_name`.
- [ ] **Step 3: plumbed** through `module "network"` (nomad-cluster/main.tf) and `module "cluster"` (main.tf).
- [ ] **Step 4: tfvars**
```hcl
dns_project_id = "development-root"
dns_zone_name  = "genow-cloud"
```

---

## Task 3c: Deploy into the existing genow VPC (private-VPC integration) (done)

**Files:** `network-e2b.tf` (new), `main.tf`, `variables.tf`, `nomad-cluster/{main.tf,variables.tf}`, `nomad-cluster/worker-cluster/{nodepool.tf,variables.tf}`, `nomad-cluster/nodepool-{api,control-server,clickhouse,loki}.tf`, `.terraform.gcp.tfvars`.

**Why:** the reused Cloud SQL is **private-IP-only** on the genow VPC. e2b's own VPC couldn't reach it, so e2b's nodes deploy **into the genow VPC** instead. That VPC is **custom-mode**, so `network_interface` must set `subnetwork` (upstream only sets `network`, which works only on auto-mode).

- [ ] **Step 1: TF-managed subnet** — `network-e2b.tf` defines `google_compute_subnetwork.e2b_nodes` (`e2b-nodes`, `var.e2b_subnet_cidr` default `10.60.0.0/24`, `private_ip_google_access = true`) in `var.network_name`.
- [ ] **Step 2: `subnetwork` variable** added to `nomad-cluster/variables.tf` + `worker-cluster/variables.tf`; each nodepool `network_interface` sets `subnetwork = var.subnetwork != "" ? var.subnetwork : null` (api, control-server, clickhouse, loki, worker-cluster) — the conditional keeps upstream auto-mode behavior when unset.
- [ ] **Step 3: wiring** — `module "cluster"` passes `subnetwork = google_compute_subnetwork.e2b_nodes.name` (so the subnet is created before the nodes); `nomad-cluster/main.tf` passes `subnetwork = var.subnetwork` to both worker-cluster calls.
- [ ] **Step 4: tfvars**
```hcl
network_name = "gcp-infrastructure-genow-euw3-nw-0000"
# e2b_subnet_cidr defaults to 10.60.0.0/24 — override if it collides
```

---

## Task 4: Enable IAM + Cloud SQL Admin APIs in Terraform (done)

**Files:** `init/main.tf`. e2b already enables 8 APIs via `google_project_service`; added `iam.googleapis.com` (service-account creation) and `sqladmin.googleapis.com` (Task 9 db/user). **Cloud DNS is NOT enabled here** — DNS lives in `development-root` (Task 5 Step 2b).

```hcl
resource "google_project_service" "iam_api" {
  service            = "iam.googleapis.com"
  disable_on_destroy = false
}
resource "google_project_service" "sqladmin_api" {
  service            = "sqladmin.googleapis.com"
  disable_on_destroy = false
}
```

- [ ] **Commit the Task 3b/3c/4/9 batch** (`.terraform.gcp.tfvars` is gitignored):
```bash
cd ~/code/e2b-infra && git add iac/provider-gcp && \
  git commit -m "feat(gcp): genow-VPC subnet, cross-project Cloud DNS, Cloud SQL db/user, sqladmin API" && git push
```

---

## Task 5: Enable the required APIs (gcloud)

**Files:** none. Project already exists + billing-linked — do NOT create it.

- [ ] **Step 1: Confirm the project** — `gcloud projects describe gcp-infrastructure-genow-euw3 --format="value(projectId,lifecycleState)"` → `… ACTIVE`.

- [ ] **Step 2: Enable deploy-project APIs**

Most are also enabled by `make init` (`init/main.tf`); this up-front enable avoids first-run races. The **bootstrap trio** (`cloudresourcemanager`, `serviceusage`, `storage`) must precede any Terraform and are already on.
```bash
gcloud services enable \
  compute.googleapis.com secretmanager.googleapis.com certificatemanager.googleapis.com \
  artifactregistry.googleapis.com sqladmin.googleapis.com \
  osconfig.googleapis.com monitoring.googleapis.com logging.googleapis.com \
  file.googleapis.com iam.googleapis.com cloudresourcemanager.googleapis.com serviceusage.googleapis.com storage.googleapis.com \
  --project=gcp-infrastructure-genow-euw3
```
(`dns` is deliberately absent — the zone lives in `development-root`, Step 2b. If this throws `AUTH_PERMISSION_DENIED`/`serviceusage`, your user needs `roles/serviceusage.serviceUsageAdmin` on the deploy project.)

- [ ] **Step 2b: Confirm the Cloud DNS API in `development-root`** — `gcloud services enable dns.googleapis.com --project=development-root` (no-op — DNS is already managed there).

- [ ] **Step 3: Verify**
```bash
gcloud services list --enabled --project=gcp-infrastructure-genow-euw3 | grep -E 'compute|certificatemanager|sqladmin|secretmanager|iam'
gcloud services list --enabled --project=development-root | grep dns
```

---

## Task 6: Confirm DNS access + the genow-cloud zone

**Files:** none. Records are written directly into `genow-cloud` by Terraform (Task 3b) — nothing to create, no propagation wait.

- [ ] **Step 1: Confirm your user can read/write DNS in `development-root`** (functional check — `get-iam-policy` needs broader perms than `dns.admin`, don't use it):
```bash
gcloud dns managed-zones list --project=development-root      # lists genow-cloud
```
If `PERMISSION_DENIED`, get `roles/dns.admin` on `development-root` granted to your user (the `google_dns_record_set`s are created there at Task 16).

- [ ] **Step 2: Confirm the zone exists** — `gcloud dns managed-zones describe genow-cloud --project=development-root --format="value(name,dnsName)"` → `genow-cloud  genow.cloud.`.

- [ ] **Step 3: (cleanup) Delete the unused sub-zone** if an earlier iteration created one:
```bash
gcloud dns managed-zones delete e2b-sandbox-genow-cloud --project=development-root
```
(not a managed resource — one-off cleanup; skip if it doesn't exist.)

---

## Task 7: Region feasibility gate — PASSED ✅

**Files:** none. Recorded here as done; re-run only if redeploying elsewhere.

- [x] **`n1-standard-4` + `e2-standard-2` exist in `europe-west3-a`; `local-ssd` available (375 GB).**
- [x] **Nested-virt probe** — an `n1-standard-4` + `Intel Skylake` VM created + deleted cleanly (attached to `…-sn-gke` with `--no-address`, since the VPC has no `default` network). euw3 is **GO** for Firecracker.
- [x] **Quota fits on top of the platform:** `CPUS` 3000/used 12 (N1 counts here — no `N1_CPUS` metric in euw3); `SSD_TOTAL_GB` 40960/259; `LOCAL_SSD_TOTAL_GB` effectively unlimited; `IN_USE_ADDRESSES` 575/0. **No increase needed (Task 8 skipped).**
- [x] **`e2b-nodes` CIDR `10.60.0.0/24` is disjoint** from `10.0.0.0/24`, `10.0.2.0/28`, and the Cloud SQL `10.1.0.x` range. The subnet itself is Terraform-managed (Task 3c) — created at `make apply`.

**Decision: proceed with europe-west3.** No us-west1 fallback.

---

## Task 8: Quota increases — not needed (Task 7 passed)

Skip. If a future region/size fails Task 7's quota check: raise `N1_CPUS`/`CPUS`, `SSD_TOTAL_GB`, `LOCAL_SSD_TOTAL_GB`, `IN_USE_ADDRESSES` via IAM & Admin → Quotas, then re-verify.

---

## Task 9: Reuse the existing Cloud SQL instance — TF db/user + generated password (done)

**Files:** `database.tf` (new) + `.terraform.gcp.tfvars`. Reuse `gcp-infrastructure-genow-euw3-sql-0000` (POSTGRES_15, **private IP `10.1.0.3`**). e2b's nodes reach it over the VPC (Task 3c). TF creates only a dedicated `e2b` database + user; the instance is referenced, never managed.

- [ ] **Step 1: `database.tf` (implemented)** — `google_sql_database.e2b` + `google_sql_user.e2b` on `var.sql_instance_name`, password from `var.sql_e2b_password`:
```hcl
resource "google_sql_user" "e2b" { name = "e2b"; instance = var.sql_instance_name; project = var.gcp_project_id; password = var.sql_e2b_password }
```
tfvars — generate a password once and set it (used for the DB user *and* the secret in Task 13):
```hcl
sql_instance_name = "gcp-infrastructure-genow-euw3-sql-0000"
sql_e2b_password  = "<openssl rand -base64 24>"
```
**Why a set password (not `random_password`):** the API reads the connection string via a Secret Manager **data source evaluated at plan time**, so the secret must be seeded *before* `make plan` — which means the password must be known up front. Created during `make apply` (Task 16); your user needs `cloudsql.admin` on the deploy project. `make destroy` drops only the e2b db + user.

- [ ] **Step 2: Seed the connection-string secret with the same password — see Task 13 (do it BEFORE `make plan`).**

- [ ] **Step 3: No operator migration** — private DB; `make migrate` is skipped (Task 15). The API's `db-migrator` runs migrations in-cluster (Task 18).

---

## Task 10: PoC compute override (done)

**Files:** `.terraform.gcp.tfvars` (gitignored). Passed via `-var-file`, overriding `.env.gcp`: `server_cluster_size=1` (`e2-standard-2`), `api_machine_type=e2-standard-2`, `clickhouse_cluster_size=0`, `loki_cluster_size=0`, and `client_clusters_config`/`build_clusters_config` at `n1-standard-4`, `autoscaler.size_max=1`, `cache_disks.count=1` (375 GB), `boot_disk=pd-balanced/100`, hugepages 50/40, map key `default`.

- [ ] **Step 1: Confirm it parses + is gitignored** — `terraform fmt .terraform.gcp.tfvars`; `git check-ignore iac/provider-gcp/.terraform.gcp.tfvars`.
- [ ] **Step 2: Accept the two risks** — `n1-standard-4` (15 GB) is the aggressive floor (raise to `n1-standard-8` if the base-template build / hello-world OOMs); `clickhouse_cluster_size=0` assumes the API boots with an empty ClickHouse string (set `=1` + `e2-standard-2` if not).

---

## Task 11: Configure `.env.gcp`

**Files:** Create `$FORK/.env.gcp` from `.env.gcp.template`.

- [ ] **Step 1: Copy + set the core values**

```bash
cd ~/code/e2b-infra && cp .env.gcp.template .env.gcp
```
```bash
GCP_PROJECT_ID=gcp-infrastructure-genow-euw3
GCP_REGION=europe-west3
GCP_ZONE=europe-west3-a
DOMAIN_NAME=e2b-sandbox.genow.cloud
PROVIDER=gcp
PREFIX=e2b-
TERRAFORM_ENVIRONMENT=dev
```
`POSTGRES_CONNECTION_STRING` is **not** needed here — Terraform generates it and it's seeded to Secret Manager (Task 16 Step 2b), which is what the cluster reads. Leave the compute-sizing vars (`SERVER_CLUSTER_SIZE`, `CLIENT_CLUSTERS_CONFIG`, …) as shipped — `.terraform.gcp.tfvars` overrides them.

- [ ] **Step 2: Select the env** — `make switch-env ENV=gcp` (writes `.last_used_env=gcp`).

---

## Task 12: Bootstrap — `make init`

**Files:** none.

- [ ] **Step 1: Log in for Terraform + Packer + Docker** — `make provider-login` is the right call now (runs `gcloud auth login --update-adc` + configures Docker):
```bash
cd ~/code/e2b-infra && make provider-login GCP_PROJECT_ID=gcp-infrastructure-genow-euw3 GCP_REGION=europe-west3
```

- [ ] **Step 2: Run init** — `make init`. Creates the versioned state bucket, `terraform init`, applies `module.init` (buckets, empty secret containers, service account, APIs incl. sqladmin), then Packer builds the `e2b-orch` image (~15–25 min). Runs as your user; if buckets/APIs/SA/Packer throw `AUTH_PERMISSION_DENIED`, that's a role missing on your user.

- [ ] **Step 3: Verify** — `gcloud compute images list --filter="family=e2b-orch"`; `gcloud storage ls | grep -E 'terraform-state|fc-|setup'`.

---

## Task 13: Seed Secret Manager values

**Files:** none. `module.init` created empty containers. (`e2b-cloudflare-api-token` no longer exists.)

- [ ] **Step 1: Seed the Postgres connection-string secret — BEFORE `make plan`.** The API reads it via a plan-time data source (`data.google_secret_manager_secret_version.postgres_connection_string`), so it must have a version first. Use the same password you set as `sql_e2b_password`:
```bash
printf '%s' 'postgresql://e2b:<PW>@10.1.0.3:5432/e2b?sslmode=disable' | \
  gcloud secrets versions add e2b-postgres-connection-string --project=gcp-infrastructure-genow-euw3 --data-file=-
```
- [ ] **Step 2: Confirm `routing-domains` empty** — `gcloud secrets versions access latest --secret=e2b-routing-domains --project=gcp-infrastructure-genow-euw3 || printf '[]' | gcloud secrets versions add e2b-routing-domains --project=gcp-infrastructure-genow-euw3 --data-file=-`.
- [ ] **Step 3: Confirm no cloudflare secret** — `gcloud secrets list --project=gcp-infrastructure-genow-euw3 | grep -i cloudflare || echo "none (expected)"`.

---

## Task 14: Build and upload service images + copy public builds

**Files:** none. Requires Go (Task 2).

- [ ] **Step 1:** `make build-and-upload` — builds + pushes api, client-proxy, dashboard-api, docker-reverse-proxy, orchestrator, template-manager, envd, clickhouse-migrator to Artifact Registry (long).
- [ ] **Step 2:** `make copy-public-builds` — copies kernels/firecrackers/busybox from `gs://e2b-prod-public-builds/*` into the project's `fc-*` buckets.
- [ ] **Step 3: Verify** — `gcloud artifacts docker images list …/e2b-orchestration | head`; `gcloud storage ls gs://gcp-infrastructure-genow-euw3-fc-kernels/ | head`.

---

## Task 15: Migrations run in-cluster (no operator step)

**Files:** none. The DB is private-IP-only, so `make migrate` from a laptop can't reach `10.1.0.3`. The API's `db-migrator` runs migrations in-cluster when the API deploys (Task 18).

- [ ] **Step 1: Skip `make migrate`** — proceed to Task 16.
- [ ] **Step 2: Verify tables (during/after Task 18)** from a VPC-attached host (SSH to an e2b node — see `self-host.md`), not your laptop:
```bash
psql "postgresql://e2b:<generated>@10.1.0.3:5432/e2b?sslmode=disable" -c '\dt'   # envs, teams, api_keys, …
```

---

## Task 16: Deploy the infrastructure (everything except Nomad jobs)

**Files:** none.

- [ ] **Step 1: Plan** — `make plan-without-jobs`. Expected: **creates** `google_compute_subnetwork.e2b_nodes`, the `google_dns_record_set`s (in `genow-cloud`), LB/cert/compute, and `google_sql_database.e2b` + `google_sql_user.e2b` — **no `cloudflare_*`**, PoC sizes. Review for unrecognized resources.

- [ ] **Step 2: Apply** — `make apply`. Long (MIGs + LB). Re-running `make plan-without-jobs && make apply` is idempotent on transient ordering errors.

- [ ] **Step 2b:** _(none — the Postgres secret is seeded in Task 13, before `make plan`, because the API reads it via a plan-time data source.)_

- [ ] **Step 3: Verify DNS records** — 
```bash
gcloud dns record-sets list --zone=genow-cloud --project=development-root --filter="name~e2b-sandbox"
dig +short A api.e2b-sandbox.genow.cloud @8.8.8.8      # main LB IP
dig +short A grpc-api.e2b-sandbox.genow.cloud @8.8.8.8 # ingress LB IP
```

---

## Task 17: Verify TLS certificates become ACTIVE

**Files:** none.

- [ ] **Step 1: Poll** — `gcloud certificate-manager certificates list --project=gcp-infrastructure-genow-euw3` until `e2b-root-cert` is `ACTIVE`. If stuck > ~30 min, check `dig CNAME _acme-challenge.e2b-sandbox.genow.cloud @8.8.8.8`.
- [ ] **Step 2: HTTPS terminates** — `curl -sS -o /dev/null -w "%{http_code} %{ssl_verify_result}\n" https://api.e2b-sandbox.genow.cloud/health || true` (expect `ssl_verify_result 0`).

---

## Task 18: Deploy the Nomad jobs

**Files:** none.

- [ ] **Step 1: Plan** — `make plan` (targets `module.nomad.*`: api, client-proxy, orchestrator, redis, otel, traefik, template-manager; ClickHouse/Loki absent at size 0).
- [ ] **Step 2: Apply** — `make apply`. The API's `db-migrator` runs migrations against the seeded connection string.
- [ ] **Step 3: Verify** — `curl -sS https://nomad.e2b-sandbox.genow.cloud/v1/jobs | head` → jobs `"Status":"running"`.

---

## Task 19: `prep-cluster` — initial user, team, API key, base template

**Files:** none.

- [ ] **Step 1:** `make prep-cluster` — creates a user + team, issues an **e2b API key** (capture it), builds the **`base`** template (exercises the `n1-standard-4` build node — watch for OOM per Task 10).
- [ ] **Step 2: Verify the base template** — from a VPC-attached host (the DB is private):
```bash
psql "postgresql://e2b:<generated>@10.1.0.3:5432/e2b?sslmode=disable" -c 'SELECT id, public FROM envs LIMIT 5;'
```

---

## Task 20: Hello-world verification via the e2b SDK (acceptance test)

**Files:** Create `$FORK/verify/hello_world.py`.

- [ ] **Step 1: Write the script**
```python
# verify/hello_world.py
import os
from e2b import Sandbox

assert os.environ.get("E2B_DOMAIN"), "set E2B_DOMAIN=e2b-sandbox.genow.cloud"
assert os.environ.get("E2B_API_KEY"), "set E2B_API_KEY=<key from prep-cluster>"

sbx = Sandbox("base")
try:
    result = sbx.commands.run("echo hello world")
    print("stdout:", result.stdout.strip())
    assert result.stdout.strip() == "hello world", f"unexpected: {result.stdout!r}"
    print("HELLO-WORLD VERIFICATION PASSED")
finally:
    sbx.kill()
```

- [ ] **Step 2: Run**
```bash
cd ~/code/e2b-infra && pip install e2b
E2B_DOMAIN=e2b-sandbox.genow.cloud E2B_API_KEY=<key from Task 19> python verify/hello_world.py
```
Expected: `HELLO-WORLD VERIFICATION PASSED`, exit 0. **Satisfies acceptance criteria 1–3.**

- [ ] **Step 3: Commit** — `git add verify/hello_world.py && git commit -m "test: e2b SDK hello-world verification" && git push`.

---

## Task 21: Document the deployment + interface contract in `genow-infrastructure`

**Files (in the `genow-infrastructure` repo):** Create `docs/e2b-sandbox.md`; ensure the spec lives at `docs/superpowers/specs/2026-07-15-e2b-byoc-agent-sandbox-design.md`.

- [ ] **Step 1: Write `docs/e2b-sandbox.md`**
```markdown
# e2b Agent Sandbox (BYOC on GCP)

Self-hosted e2b for the swappable agent-sandbox backend. `genow-infrastructure`
stays Terragrunt/GKE-pure; the e2b stack lives in a separate fork.

- **Fork:** https://github.com/Genow-AI/e2b-infra (branch `genow/gcp-cloud-dns`, pinned to `a84fe31` / tag `2026.28`)
- **GCP project:** `gcp-infrastructure-genow-euw3` (shared central project), region `europe-west3`
- **Network:** deployed into the existing VPC `…-nw-0000`, subnet `e2b-nodes` (10.60.0.0/24)
- **DNS:** records in the existing `genow-cloud` zone in `development-root` (no sub-zone)
- **Cloud SQL:** reuses `…-sql-0000` (private IP); TF-managed `e2b` db + user
- **Changes vs upstream:** Cloudflare→Cloud DNS (cross-project), genow-VPC subnet + `subnetwork` on nodepools, `.terraform.gcp.tfvars` PoC sizing

## Interface output contract (provider-agnostic)

| Output | Value | Source |
|--------|-------|--------|
| `sandbox_provider` | `"e2b"` | constant |
| `sandbox_domain` | `e2b-sandbox.genow.cloud` | Cloud DNS / shared endpoint |
| `sandbox_api_key` | per-customer e2b **team API key** | Secret Manager; issued via `make prep-cluster` / team creation |

Onboarding a customer = create a team + issue an API key (no new infra). Swapping the
backend later = new adapter + new infra deployment, zero change to consumers.
```

- [ ] **Step 2: Commit** — `git add docs/ && git commit -m "docs: record e2b BYOC sandbox deployment + interface output contract (GP-4108)"` (on a branch).

---

## Task 22: Cost-control / teardown decision

**Files:** none.

- [ ] **Step 1: Decide**
  - **Verification spike → scoped teardown:** `cd iac/provider-gcp && SKIP_NOMAD_JOB_COUNT=1 make destroy` (the `destroy` target is in the **provider** Makefile, not the root — `make destroy` from the repo root fails; `SKIP_NOMAD_JOB_COUNT=1` skips the `template_manager_count` data source, which otherwise aborts destroy because Nomad is unreachable during teardown). Removes only e2b's resources from e2b's state (incl. the `e2b-nodes` subnet, the `e2b` db + user, and the record sets in `genow-cloud`) — **not** the VPC, the Cloud SQL instance, or the `genow-cloud` zone (all data sources / referenced). **Never `gcloud projects delete`.** Review the plan lists only `e2b-*` / `…-fc-*` before approving. Optionally delete the `…-terraform-state` bucket. PoC total ~$5–30.
  - **Becomes the shared service → keep running** and apply spec §7 levers, but restore HA + `n1-standard-8` (don't run the PoC `n1-standard-4`/1-server settings long-term). Delete `.terraform.gcp.tfvars` to revert to full sizing.
- [ ] **Step 2: Record** the decision (running vs torn down + date) in `docs/e2b-sandbox.md`.

---

## Self-review — spec coverage

| Spec section / AC | Covered by |
|---|---|
| AC1 use e2b Terraform | Tasks 1–2, 12/16/18 (its make pipeline) |
| AC2 viable for GCP + DNS/TLS | Task 3 (Cloud DNS swap) + 3b (cross-project `genow-cloud`), 6/17 (Certificate Manager) |
| AC2 network/DB integration for GCP | Task 3c (genow VPC subnet), Task 9 (reuse private Cloud SQL) |
| AC3 infra setup + small verification | Tasks 12–20, teardown Task 22 |
| AC4 swappable interface / output contract | Task 21 |
| §3 shared-project decision + coexistence | Header + guardrails + Tasks 5/16/22 |
| §4 shared multi-tenant, one project | Task 5; contract hands out shared domain + per-customer key (Task 21) |
| §5.2 genow-infrastructure GKE-pure, docs only | Task 21 |
| §7 cost/sizing | Task 10 (PoC), Task 22 (teardown/levers) |
| §9 Cloudflare→Cloud DNS (+ cert DNS-auth) | Task 3 + 3b |
| §10 hello-world | Tasks 19–20 |
| §12 risks: euw3 feasibility, quota, private DB reachability, fork drift | Task 7 (passed), Task 3c (VPC), Task 1 (pin) |
| §13 non-goals | Respected: no HA tuning, no adapter impl, dev-only, no Terragrunt changes |

**Placeholder scan:** `<generated>` (TF-generated DB password, via `terraform output`) and `<key from Task 19>` are intentional. **Name consistency:** DNS zone `genow-cloud` (`dns_zone_name`), DNS project `development-root` (`dns_project_id`), VPC `gcp-infrastructure-genow-euw3-nw-0000` + subnet `e2b-nodes` (`network_name`/`subnetwork`), Cloud SQL `gcp-infrastructure-genow-euw3-sql-0000` (`sql_instance_name`), secret `e2b-postgres-connection-string`, and the `default` cluster-config map key are used consistently across Tasks 3b/3c/5/6/9/16.
