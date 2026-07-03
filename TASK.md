# TASK.md — Terraform Cost & Risk Preview Tool

Status: **Planning only. Do not start implementation until DUL explicitly says to begin building.**

Read `CLAUDE.md` in full before starting any phase below.

---

## Phase 0 — Setup & scaffolding
- [ ] Decide tech stack (Go vs TS/Node) — see CLAUDE.md tradeoffs, confirm with DUL before locking in
- [ ] Init repo structure with the pipeline module boundaries pre-created:
  - `parser/`
  - `normalizer/providers/aws/` (+ `gcp/`, `azure/` stub folders with README stub notice)
  - `pricing/providers/aws/` (+ `gcp/`, `azure/` stubs)
  - `rules/providers/aws/` (+ `gcp/`, `azure/` stubs)
  - `report/`
  - `cli/`
- [ ] Set up basic test harness / sample `terraform plan` JSON fixtures (small hand-written examples covering create/update/delete/replace)

## Phase 1 — Plan Parser
- [ ] Parse `terraform show -json` output into an internal resource-change representation
- [ ] Support all four change types: create, update, delete, replace (create+delete)
- [ ] Unit tests against fixture plan JSONs

## Phase 2 — Resource Normalizer (AWS only)
- [ ] Define normalized internal schema (type category, size/tier, region, tags, stateful flag)
- [ ] Implement AWS mapping for MVP resource types: EC2, RDS, EBS, S3, ALB/NLB, Lambda
- [ ] Stub `gcp/` and `azure/` normalizers with clear "not implemented" markers

## Phase 3 — Pricing Lookup (AWS only, static snapshot)
- [ ] Build/curate static pricing snapshot for the MVP resource type list (~20-30 entries)
- [ ] Implement lookup logic: normalized resource → matched price entry → $/mo estimate
- [ ] Handle "no match found" gracefully (explicit "cost unknown" output, never silently $0)

## Phase 4 — Rule Engine (build in tier order — this is the differentiator, don't rush it)
- [ ] **Tier 1: Destructive/data-loss rules**
  - [ ] Flag all delete/replace operations
  - [ ] Escalate severity for stateful resource types
  - [ ] Flag missing deletion protection where applicable
- [ ] **Tier 2: Security misconfig rules**
  - [ ] Open security group / firewall rules (0.0.0.0/0 on sensitive ports)
  - [ ] Unencrypted or public storage resources
  - [ ] Wildcard IAM policies
- [ ] **Tier 3: Cost-risk hybrid rules**
  - [ ] Oversized-resource heuristic (relative to plan median)
  - [ ] Missing/removed cost tags on new resources
  - [ ] Unbounded autoscaling config
- [ ] Each rule outputs: severity, category, resource address, plain-language explanation

## Phase 5 — Report Formatter
- [ ] Shared internal data model for findings + cost data (used by all output formats)
- [ ] Terminal/human-readable output: summary line → cost table → findings grouped by severity
- [ ] `--format json` output for machine consumption
- [ ] Confirm output reads correctly with zero findings (clean plans should look clean, not empty/broken)

## Phase 6 — CLI wrapper
- [ ] `tool analyze <planfile.json>` command
- [ ] Flags: `--format json|text`, `--region` (if not inferable from plan)
- [ ] Basic `--help` and versioning

## Phase 7 — Demo & validation
- [ ] Build 3-4 realistic sample Terraform plans (clean, cost-increase, destructive-change, security-misconfig) as demo fixtures
- [ ] Record a short demo (GIF or terminal recording) showing each fixture's output
- [ ] Write README with install instructions, example output, and the "why this differs from Infracost" positioning from CLAUDE.md

## Phase 8 — GitHub Action wrapper (stretch, after CLI is solid)
- [ ] Wrap CLI in a GitHub Action that runs on PR, posts findings as a PR comment
- [ ] Test against a real (throwaway) repo with a real PR

## Explicitly deferred (do not build without a separate decision to proceed)
- GCP/Azure rule + pricing implementations
- Live pricing sync
- Slack notifications
- Any diagram/visual rendering (Floci integration)
- Any connection to the parked "IDP Self-Service Provisioning Module" / Temporal approval workflow idea

---

## Notes for whoever (or whichever Claude Code session) picks this up
- This is a **portfolio piece, not a revenue product.** Prioritize a clean, demoable, well-documented MVP over feature completeness.
- The rule engine's Tier 3 (cost-risk hybrid) findings are the actual point of this project. Don't let Tier 1/2 (which are table-stakes, done by every competitor) eat all the build time.
- If scope pressure hits, cut Phase 8 (GitHub Action) before cutting Tier 3 rules — the differentiation lives in the rules, not the distribution surface.
