# tfx — Terraform Plan Cost & Risk Analyzer

`tfx` analyzes a `terraform plan` JSON file **before** `terraform apply` and gives you a single report covering cost delta, destructive-change warnings, and security misconfigs — including cost-risk hybrid findings that no other tool surfaces.

---

## Install

### Option 1 — Build from source (recommended)

Requires Go 1.21+. Check with `go version`.

```bash
git clone https://github.com/amtago/infra-cost-risk-adviser
cd infra-cost-risk-adviser
go build -o tfx ./cli/
```

Move the binary somewhere on your `PATH`:

```bash
# macOS / Linux
sudo mv tfx /usr/local/bin/tfx

# or without sudo, into your home bin
mkdir -p ~/.local/bin && mv tfx ~/.local/bin/tfx
# make sure ~/.local/bin is in your PATH
```

Verify:

```bash
tfx version
# tfx v0.1.0
```

### Option 2 — Run without installing

```bash
go run ./cli/ analyze plan.json
```

---

## Quick start

**Step 1 — generate a plan JSON file from your Terraform project:**

```bash
cd your-terraform-directory/
terraform init          # if not already initialized
terraform plan -out tfplan.binary
terraform show -json tfplan.binary > plan.json
```

**Step 2 — run tfx:**

```bash
tfx analyze plan.json
```

That's it. No API keys, no network access, no configuration required.

---

## Usage

```
tfx analyze <plan.json> [flags]
tfx cfn <changeset.json> [--template template.json] [flags]
tfx version
tfx --help

Flags:
  --format text|json|markdown Output format (default: text)
  --region <region>           AWS region for pricing lookups (default: us-east-1)
  --required-tags <file.txt>  Path to a file listing required cost-allocation tags (one per line).
                              Overrides the built-in Env/Team defaults. Empty file disables the rule.
```

**Use a different region:**

```bash
tfx analyze plan.json --region eu-west-1
```

**Get JSON output (for CI pipelines or scripting):**

```bash
tfx analyze plan.json --format json
tfx analyze plan.json --format json | jq '.summary'
```

**Get Markdown output (for GitHub Step Summaries, wikis, or Notion):**

```bash
tfx analyze plan.json --format markdown
tfx analyze plan.json --format markdown >> "$GITHUB_STEP_SUMMARY"
```

**Enforce custom cost-allocation tags:**

```bash
tfx analyze plan.json --required-tags required-tags.txt
tfx cfn changeset.json --template template.json --required-tags required-tags.txt
```

---

## CloudFormation support

`tfx` can analyze AWS CloudFormation change sets using the same pipeline — pricing, destructive-change detection, security rules, and cost-risk hybrid findings all work identically.

**Step 1 — create and describe a change set:**

```bash
aws cloudformation create-change-set \
  --stack-name my-stack \
  --change-set-name cs-1 \
  --template-body file://template.json \
  --capabilities CAPABILITY_NAMED_IAM

aws cloudformation describe-change-set \
  --stack-name my-stack \
  --change-set-name cs-1 > changeset.json
```

**Step 2 — run tfx:**

```bash
# Change set only (resource type + action, no attribute details)
tfx cfn changeset.json

# With the template for full property-level analysis (recommended)
tfx cfn changeset.json --template template.json
```

With `--template`, tfx reads resource properties directly from the template so security rules (open security groups, public S3 ACLs, wildcard IAM, missing encryption) and cost-risk rules (oversized instances, missing tags, unbounded autoscaling) fire on real attribute values.

Without `--template`, tfx can still report which resources are being created, replaced, or deleted and flag destructive changes.

---

## Example output

### Clean plan

```
$ tfx analyze plan.json

This plan has no net cost change and has no issues found.

Cost table: no resources with pricing data.

Findings: none.
```

### Cost increase with oversized resource warning

```
$ tfx analyze plan.json

This plan adds $619.62/mo and has 2 issue(s) found.
Note: 1 resource(s) have unknown cost (usage-based pricing; see table below).

Cost breakdown ($/mo):
  Resource                                                Change     $/mo
  --------------------------------------------------------------------------------
  aws_db_instance.main                                    + create   $12.41
  aws_instance.app                                        + create   $560.64
  aws_instance.worker                                     + create   $30.37
  aws_lb.frontend                                         + create   $16.20
  aws_s3_bucket.uploads                                   + create   unknown
  --------------------------------------------------------------------------------
                                                                      $619.62/mo net

Findings (grouped by severity):

  [WARNING]
  • [cost-risk] aws_instance.app costs an estimated $560.64/mo, which is 24.1x the median
    resource cost in this plan ($23.29/mo). Verify this instance size is intentional.

  [INFO]
  • [cost-risk] aws_instance.app is missing cost-allocation tag(s): Team.
```

### Destructive changes (exits 2)

```
$ tfx analyze plan.json

This plan adds $70.38/mo and has 2 critical issue(s) require attention.

Cost breakdown ($/mo):
  Resource                                                Change     $/mo
  --------------------------------------------------------------------------------
  aws_db_instance.prod                                    ± replace  $49.64
  aws_ebs_volume.data                                     - delete   $40.00
  aws_instance.api                                        ~ update   $60.74
  --------------------------------------------------------------------------------
                                                                      $110.38 added
                                                                     -$40.00 removed
                                                                      $70.38/mo net

Findings (grouped by severity):

  [CRITICAL]
  • [destructive] aws_db_instance.prod will be destroyed and recreated (replace), causing downtime.
  • [destructive] aws_ebs_volume.data will be permanently deleted.

  [WARNING]
  • [destructive] aws_db_instance.prod does not have deletion protection enabled.
```

### GCP plan (exits 2)

```
$ tfx analyze fixtures/gcp_plan.json

This plan adds $363.83/mo and has 2 critical issue(s) require attention.
Note: 2 resource(s) have unknown cost (usage-based pricing; see table below).

Cost breakdown ($/mo):
  Resource                                                Change     $/mo
  --------------------------------------------------------------------------------
  google_compute_disk.data                                - delete   $17.00
  google_compute_firewall.allow_ssh                       + create   unknown
  google_compute_instance.web                             + create   $97.84
  google_container_cluster.primary                        + create   $73.00
  google_container_node_pool.workers                      + create   $116.80
  google_sql_database_instance.db                         + create   $93.19
  google_storage_bucket.assets                            + create   unknown
  --------------------------------------------------------------------------------
                                                                      $380.83 added
                                                                     -$17.00 removed
                                                                      $363.83/mo net

Findings (grouped by severity):

  [CRITICAL]
  • [destructive] google_compute_disk.data (google_compute_disk) will be permanently deleted.
  • [security] google_compute_firewall.allow_ssh allows inbound SSH (port 22) from 0.0.0.0/0.

  [WARNING]
  • [destructive] google_sql_database_instance.db does not have deletion_protection enabled.
  • [security] google_storage_bucket.assets has uniform_bucket_level_access disabled.
  • [security] google_sql_database_instance.db does not require SSL connections.
  • [security] google_sql_database_instance.db does not have automated backups enabled.
  • [cost-risk] google_container_node_pool.workers has autoscaling enabled with no max_node_count.

  [INFO]
  • [cost-risk] google_compute_instance.web is missing cost-allocation label(s): team.
  • [cost-risk] google_container_node_pool.workers is missing cost-allocation label(s): env, team.
```

### Security misconfig (exits 2)

```
$ tfx analyze plan.json

This plan adds $49.64/mo and has 4 critical issue(s) require attention.

Findings (grouped by severity):

  [CRITICAL]
  • [security] aws_security_group.bastion allows inbound SSH (port 22) from 0.0.0.0/0.
  • [security] aws_security_group.db allows inbound PostgreSQL (port 5432) from 0.0.0.0/0.
  • [security] aws_s3_bucket.reports has a public ACL ("public-read").
  • [security] aws_iam_role_policy.lambda_exec contains Action:"*" (full AWS permissions).
```

---

## CI / GitHub Actions

### Option 1 — Use the tfx GitHub Action (recommended)

The action supports both Terraform plan files and CloudFormation change sets. Provide one of `plan-file` or `cfn-changeset-file` — not both.

**Terraform:**

```yaml
# .github/workflows/terraform.yml
permissions:
  contents: read
  pull-requests: write   # required to post PR comments

jobs:
  tfx:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Terraform plan
        run: |
          terraform init
          terraform plan -out tfplan.binary
          terraform show -json tfplan.binary > plan.json

      - name: Run tfx
        uses: amtago/infra-cost-risk-adviser@main
        with:
          plan-file:           plan.json
          region:              us-east-1
          github-token:        ${{ secrets.GITHUB_TOKEN }}
          comment-on-pr:       'true'
          fail-on-critical:    'true'
          # required-tags-file: .github/required-tags.txt  # optional
```

**CloudFormation:**

```yaml
# .github/workflows/cloudformation.yml
permissions:
  contents: read
  pull-requests: write
  id-token: write   # required for AWS OIDC auth

jobs:
  tfx-cfn:
    name: CloudFormation change set analysis
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Configure AWS credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: ${{ secrets.AWS_ROLE_ARN }}
          aws-region: us-east-1

      - name: Create and describe change set
        run: |
          aws cloudformation create-change-set \
            --stack-name my-stack \
            --change-set-name cs-${{ github.run_id }} \
            --template-body file://template.json \
            --capabilities CAPABILITY_NAMED_IAM
          aws cloudformation wait change-set-create-complete \
            --stack-name my-stack \
            --change-set-name cs-${{ github.run_id }}
          aws cloudformation describe-change-set \
            --stack-name my-stack \
            --change-set-name cs-${{ github.run_id }} > changeset.json

      - name: Run tfx
        uses: amtago/infra-cost-risk-adviser@main
        with:
          cfn-changeset-file:  changeset.json
          cfn-template-file:   template.json
          region:              us-east-1
          github-token:        ${{ secrets.GITHUB_TOKEN }}
          comment-on-pr:       'true'
          fail-on-critical:    'true'
          # required-tags-file: .github/required-tags.txt  # optional
```

**Action inputs:**

| Input | Required | Default | Description |
|---|---|---|---|
| `plan-file` | one of | — | Path to `terraform show -json` output |
| `cfn-changeset-file` | one of | — | Path to `aws cloudformation describe-change-set` output |
| `cfn-template-file` | | `''` | CloudFormation template JSON (optional; enables attribute-level rules) |
| `required-tags-file` | | `''` | Path to a file listing required cost-allocation tags (one per line). Overrides built-in `Env`/`Team` defaults. Empty file disables the rule. |
| `region` | | `us-east-1` | AWS region for pricing |
| `github-token` | | `github.token` | Token to post PR comments |
| `comment-on-pr` | | `true` | Post findings as a PR comment |
| `fail-on-critical` | | `true` | Exit non-zero on critical findings |

**Action outputs:** `net-delta-usd`, `critical-count`, `warning-count`, `report-json`

The action automatically writes a formatted Markdown report to the [GitHub Actions Step Summary](https://docs.github.com/en/actions/writing-workflows/choosing-what-your-workflow-does/workflow-commands-for-github-actions#adding-a-job-summary) (`$GITHUB_STEP_SUMMARY`) on every run — visible in the Actions UI without needing a PR.

### Option 2 — Run the CLI directly

```yaml
- name: Analyze plan with tfx
  run: tfx analyze plan.json --format json | tee tfx-report.json
  continue-on-error: true

- name: Fail on critical findings
  run: |
    criticals=$(jq '.summary.finding_counts.critical // 0' tfx-report.json)
    [ "$criticals" -eq 0 ] || exit 1
```

### Exit codes

| Code | Meaning |
|---|---|
| `0` | Clean plan, or warnings/info only |
| `1` | Error (unreadable file, bad JSON, missing argument) |
| `2` | One or more **critical** findings |

---

## Try the demo fixtures

The repo ships fixtures for both Terraform and CloudFormation. Run `demo.sh` to see all finding types at once:

```bash
# Clone and build first
git clone https://github.com/amtago/infra-cost-risk-adviser && cd infra-cost-risk-adviser
go build -o tfx ./cli/

bash demo.sh ./tfx
```

Or run individual fixtures:

```bash
# Terraform
tfx analyze fixtures/clean_plan.json
tfx analyze fixtures/cost_increase_plan.json
tfx analyze fixtures/destructive_plan.json
tfx analyze fixtures/security_misconfig_plan.json

# Mixed AWS + GCP + Azure plan (auto-detected, single report)
tfx analyze fixtures/mixed_provider_plan.json

# Azure Terraform plan
tfx analyze fixtures/azure_plan.json

# GCP Terraform plan
tfx analyze fixtures/gcp_plan.json

# CloudFormation — change set only (destructive findings, costs unknown without template)
tfx cfn fixtures/cfn_changeset.json

# CloudFormation — with template (full pricing + all three rule tiers)
tfx cfn fixtures/cfn_changeset.json --template fixtures/cfn_template.json
```

The Azure fixture surfaces: open SSH NSG rule (critical), public blob storage (critical), stateful managed disk deletion (critical), insufficient backup retention (warning), missing MSSQL TDE encryption (warning), oversized VM at 8× plan median (warning), and missing cost-allocation tags (info).

The GCP fixture surfaces: open SSH firewall rule (critical), stateful Persistent Disk deletion (critical), Cloud SQL without SSL or backups (warning), Cloud Storage with legacy ACLs (warning), GKE node pool with unbounded autoscaling (warning), and missing cost-allocation labels (info).

The CFN fixture with template surfaces: open SSH security group (critical), RDS replace causing downtime (critical), EBS permanently deleted (critical), missing encryption (warning), oversized EC2 at 11× plan median (warning), and missing cost-allocation tags (info).

---

## Rule reference

Rules fire on the detected provider. AWS and GCP rules run in the same pipeline — a mixed plan produces findings from both.

### Tier 1 — Destructive / data-loss

| Rule | AWS | GCP | Azure | Severity |
|---|---|---|---|---|
| Resource will be deleted | ✓ | ✓ | ✓ | warning |
| Resource will be replaced (destroy + recreate) | ✓ | ✓ | ✓ | warning |
| Stateful resource deleted or replaced | ✓ | ✓ | ✓ | **critical** |
| Stateful resource missing `deletion_protection` | ✓ (RDS, DynamoDB) | ✓ (Cloud SQL) | — | warning |
| Database `backup_retention_days` too low | — | — | ✓ (PostgreSQL, MySQL) | warning |

### Tier 2 — Security misconfig

| Rule | AWS | GCP | Azure | Severity |
|---|---|---|---|---|
| Firewall open to internet on SSH/RDP/DB ports | ✓ (security groups) | ✓ (compute firewall) | ✓ (NSG) | **critical** |
| Public object storage | ✓ (S3 public ACL) | ✓ (uniform bucket-level access) | ✓ (allow_blob_public_access) | **critical** |
| Storage without HTTPS enforcement | — | — | ✓ (storage account) | warning |
| IAM policy with `Action: "*"` | ✓ | — | — | **critical** |
| IAM policy with `Resource: "*"` | ✓ | — | — | warning |
| Storage/DB without encryption | ✓ (RDS, EBS) | ✓ (Cloud SQL SSL + backups) | ✓ (MSSQL TDE, PostgreSQL SSL) | warning |

### Tier 3 — Cost-risk hybrid

| Rule | AWS | GCP | Azure | Severity |
|---|---|---|---|---|
| Resource cost > 5× median in plan | ✓ | ✓ | ✓ | warning |
| Autoscaling with no upper bound | ✓ (ASG max_size) | ✓ (GKE max_node_count) | ✓ (AKS max_count) | warning |
| New/updated resource missing required cost tags/labels | ✓ | ✓ | ✓ | info |

The missing-tags/labels rule checks for `env` and `team` by default. Override with `--required-tags`:

```
# required-tags.txt
# Lines starting with # are comments. Blank lines are ignored.
Env
Team
Owner
CostCenter
```

```bash
tfx analyze plan.json --required-tags required-tags.txt
```

Providing an empty file (comments only) disables the rule entirely.

---

## Supported resources

Prices are approximate on-demand rates stored as a static snapshot (no API keys or internet access required). Resources with usage-based pricing are reported as `unknown` rather than `$0`.

### AWS (`pricing/providers/aws/prices.go`, region: us-east-1)

| Resource | Pricing | Rules |
|---|---|---|
| `aws_instance` | T3 / M5 / C5 / R5 | Tier 1, 3 |
| `aws_db_instance` | T3 / R6G / M6G | Tier 1, 2, 3 |
| `aws_rds_cluster` | R6G / R5 / T3 | Tier 1, 2, 3 |
| `aws_ebs_volume` | per GB — gp2/gp3/io1/io2/st1/sc1 | Tier 1, 2 |
| `aws_elasticache_cluster` | T3 / R6G / M6G | Tier 1 |
| `aws_elasticache_replication_group` | T3 / R6G / M6G | Tier 1 |
| `aws_lb` / `aws_alb` | base rate | Tier 3 |
| `aws_s3_bucket` | usage-based → unknown | Tier 2, 3 |
| `aws_dynamodb_table` | usage-based → unknown | Tier 1, 3 |
| `aws_lambda_function` | usage-based → unknown | Tier 3 |
| `aws_security_group` | n/a | Tier 2 |
| `aws_iam_policy` / `aws_iam_role_policy` | n/a | Tier 2 |
| `aws_autoscaling_group` | n/a | Tier 3 |

### Azure (`pricing/providers/azure/prices.go`, region: East US)

| Resource | Pricing | Rules |
|---|---|---|
| `azurerm_linux_virtual_machine` / `azurerm_windows_virtual_machine` | B / D / E / F / NC series | Tier 1, 3 |
| `azurerm_kubernetes_cluster` | per default node pool VM size | Tier 1, 3 |
| `azurerm_mssql_database` / `azurerm_sql_database` | DTU (S0–S4) and vCore (GP/BC Gen5) | Tier 1, 2, 3 |
| `azurerm_postgresql_server` / `azurerm_postgresql_flexible_server` | GP_Gen5 / GP_Standard_Dsv3 / Burstable | Tier 1, 2, 3 |
| `azurerm_mysql_server` / `azurerm_mysql_flexible_server` | GP_Gen5 / Burstable | Tier 1, 2, 3 |
| `azurerm_managed_disk` | per GB — Standard / StandardSSD / Premium / UltraSSD LRS | Tier 1 |
| `azurerm_lb` / `azurerm_application_gateway` | base rate | Tier 3 |
| `azurerm_storage_account` | usage-based → unknown | Tier 2, 3 |
| `azurerm_cosmosdb_account` | usage-based → unknown | Tier 1, 3 |
| `azurerm_function_app` / `azurerm_linux_function_app` | usage-based → unknown | Tier 3 |
| `azurerm_network_security_group` | free | Tier 2 |

### GCP (`pricing/providers/gcp/prices.go`, region: us-central1)

| Resource | Pricing | Rules |
|---|---|---|
| `google_compute_instance` | E2 / N2 / N1 / C2 machine types | Tier 1, 3 |
| `google_sql_database_instance` | db-f1-micro / n1-standard / db-custom | Tier 1, 2, 3 |
| `google_compute_disk` | per GB — pd-standard / pd-ssd / pd-balanced / pd-extreme | Tier 1 |
| `google_container_cluster` | GKE management fee ($73/mo) | Tier 1, 3 |
| `google_container_node_pool` | via machine_type lookup | Tier 3 |
| `google_filestore_instance` | per GB — BASIC_HDD / BASIC_SSD | Tier 1 |
| `google_compute_forwarding_rule` | base rate | Tier 3 |
| `google_storage_bucket` | usage-based → unknown | Tier 2, 3 |
| `google_cloudfunctions_function` | usage-based → unknown | Tier 3 |
| `google_pubsub_topic` | usage-based → unknown | Tier 3 |
| `google_bigquery_dataset` | usage-based → unknown | Tier 3 |
| `google_compute_firewall` | n/a | Tier 2 |

Provider is auto-detected per resource type (`azurerm_` → Azure, `google_` → GCP, everything else → AWS). Mixed-provider plans work — all resources run through a single report.

---

## Development

```bash
go test ./...          # 259 tests
go build -o tfx ./cli/
```

**Adding a price entry:** edit `pricing/providers/aws/prices.go` — no build step, just update the map and rebuild.

**Adding a rule:** implement the `rules.Rule` interface and register it in `rules/providers/aws/aws.go:AllRules()`.
