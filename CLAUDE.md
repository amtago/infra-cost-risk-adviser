# CLAUDE.md — Terraform Cost & Risk Preview Tool

## Purpose
This file is the persistent context handoff for any Claude Code session working on this project. Read this before writing any code.

## What this is
A CLI tool (with a GitHub Action wrapper) that analyzes a `terraform plan` JSON output **before** `terraform apply` and reports:
- Cost delta (total + per-resource breakdown)
- Destructive-change warnings (esp. stateful resources)
- Security/risk misconfig flags
- Cost-risk hybrid findings (the differentiator — see below)

Output surfaces: terminal (CLI first), GitHub PR comment (GitHub Action, phase 2), Slack (optional, later).

## Why this project exists
**Portfolio-only. Not for monetization.** Goal is to demonstrate platform/DevOps engineering skill — specifically: static analysis tooling, rule-engine design, cloud pricing API integration, and CI/CD packaging. This is "Option A" in a two-option comparison; "Option B" (a Temporal-orchestrated self-service infra provisioning platform) stays parked and unrelated unless this project is later repurposed for monetization.

## Competitive context (why this isn't "just another Infracost")
- **Infracost** already does full multi-cloud cost estimation (1,100+ resource types, AWS/Azure/GCP) with PR comments, editor integration, and AI agent skills. It is cost-first; policy/risk is a separate concern bolted on via external tooling.
- **OpenInfraQuote** is a lighter, privacy-focused CLI (no API keys, local-only) — AWS now, GCP/Azure roadmapped. Also cost-first.
- **Neither treats cost and risk as the same signal.** Our differentiator: a single rule engine that produces findings which are *simultaneously* cost and risk relevant — not two separate reports bolted together. See "Rule Engine — Finding Categories" below.
- Do not attempt to out-cover Infracost's resource/pricing breadth. Depth of the risk-flag angle is the differentiator, not breadth of cloud support.

## Architecture (locked decision: Option C — "designed for it, build one")

```
Plan Parser → Resource Normalizer → Pricing Lookup → Rule Engine → Report Formatter
                     ↑                     ↑              ↑
              provider adapters     provider adapters  provider rule sets
              (aws/ IMPLEMENTED,    (aws/ IMPLEMENTED,  (aws/ IMPLEMENTED,
               gcp/ azure/ STUBBED)  gcp/ azure/ STUBBED) gcp/ azure/ STUBBED)
```

**Rule:** every module must have a clean interface boundary so `providers/gcp/` and `providers/azure/` can be implemented later without touching the core pipeline. Stub them with a clear `NotImplementedError` / TODO and a one-line docstring — visible intent, no dead-end silent failures. This abstraction is itself part of the portfolio story ("designed for scale, shipped AWS first") — don't skip it even though only AWS ships.

### Pipeline stages
1. **Plan Parser** — reads `terraform show -json <plan>` output, extracts resource changes (create/update/delete/replace) and their `before`/`after` attribute values.
2. **Resource Normalizer** — maps provider-specific resource types (e.g. `aws_instance`, `aws_db_instance`) into a normalized internal schema (type category, size/tier, region, tags, stateful flag) so downstream stages don't need to know provider quirks.
3. **Pricing Lookup** — matches a normalized resource against a pricing dataset, returns monthly cost estimate (see Pricing Data below).
4. **Rule Engine** — runs normalized resources + plan diff through rule set, emits structured findings (see Rule Engine below).
5. **Report Formatter** — takes findings + cost data, renders to target format (terminal text first; JSON output should also be supported for future machine consumption / GitHub Action).

## Pricing data (AWS-first)
- Primary reference: AWS Price List Query API / bulk offer files (free, public, notoriously messy schema).
- **MVP approach: do not build a live pricing sync.** Use a small, hand-curated static pricing snapshot covering ~20-30 common resource types:
  - EC2 (common instance families/sizes)
  - RDS (common instance classes)
  - EBS (gp3/io2 volumes)
  - S3 (storage tiers, simplified)
  - ALB/NLB
  - Lambda (simplified request/duration model)
- Store as a simple JSON or CSV lookup table, versioned in-repo, refreshed manually/via script later — not part of MVP scope.
- Study Infracost's open-source engine for their plan-to-SKU mapping approach as a reference (do not copy their hosted pricing data or proprietary datasets).

## Rule Engine — Finding Categories (the core differentiator, build in this priority order)

### Tier 1 — Destructive / data-loss risk (build first)
- Any resource marked `delete` or `replace` in the plan.
- Escalate severity if resource is stateful (RDS, EBS, ElastiCache, DynamoDB) — especially if deletion protection is not set.
- Flag "replace" operations that imply downtime (force-new attribute changes), not just deletes.

### Tier 2 — Security misconfig (build second)
- Security groups / firewall rules with ingress open to `0.0.0.0/0` on sensitive ports (22, 3389, DB ports).
- S3 buckets (or equivalent) without encryption, or with public read/write ACLs.
- IAM policies with wildcard `Action: "*"` or `Resource: "*"`.

### Tier 3 — Cost-risk hybrid (build third — this is the actual differentiator vs. Infracost/OpenInfraQuote)
- Resource significantly oversized relative to other resources in the same plan (simple heuristic first, e.g. "this instance type costs Nx the median in this plan" — no historical data needed for MVP).
- Cost-relevant tags being removed or missing on new resources (ties to the "tagging-free environments" thesis from the Costly product — do not build integration with Costly, just flag it).
- Auto-scaling configuration with no upper bound / max size set.

Each finding must include: severity (info/warning/critical), category (destructive/security/cost-risk), the resource address, and a plain-language explanation — not just a rule ID.

## Output format requirements
- **Plain-language summary line first** — one sentence a non-infra person could understand (e.g. "This plan adds $220/mo and will destroy 1 production database.")
- **Cost table** — per-resource line items with $/mo delta.
- **Findings list** — grouped by severity, not by category, so critical items surface first regardless of type.
- Support both human-readable terminal output and `--format json` for later CI integration.

## Explicitly out of scope for MVP
- No diagram/visual rendering (that's a separate, optional future integration with the Floci product — not a dependency, don't build toward it).
- No live pricing sync / API-key-based pricing service.
- No GCP/Azure rule implementations (stub only).
- No approval workflows, no Temporal, no provisioning — that's the separate "IDP Self-Service Provisioning Module" idea and is explicitly unrelated to this project.
- No policy-as-code framework integration (OPA/Sentinel) — rules are simple, hardcoded, readable Go/Python/TS functions for MVP.

## Suggested tech stack (not locked — revisit at build time)
- CLI: Go or TypeScript/Node — pick based on whichever makes packaging as a GitHub Action easiest and matches DUL's other tooling comfort. Go gives a single static binary (easier distribution, matches OpenInfraQuote's approach); TS/Node is faster to iterate and matches DUL's existing Next.js-heavy stack.
- Input: `terraform show -json <planfile>` — never parse the raw plan binary or HCL directly.
- Output rendering: keep formatter logic separate from rule engine so JSON/terminal/future-Slack outputs share one data model.

## Reference repos (for study, not for copying)
- `infracost/infracost` — open-source engine, reference for plan parsing and resource-to-SKU mapping patterns.
- `terrateamio/openinfraquote` — lightweight CLI approach, reference for the match/price two-step pipeline pattern and zero-dependency local-first design.
- `temporal-sa/temporal-infra-provisioning-demo` — not directly relevant to this project; only relevant if this tool's findings are later wired into the parked "IDP Self-Service" approval-gate idea.

## Status
Concept/planning stage only. No build started as of this document's creation.
