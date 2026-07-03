package report

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/amt/tf-cost-risk/parser"
	"github.com/amt/tf-cost-risk/pricing"
	"github.com/amt/tf-cost-risk/rules"
)

// -- helpers --

func est(addr string, ct parser.ChangeType, cost float64) pricing.Estimate {
	return pricing.Estimate{ResourceAddress: addr, ChangeType: ct, MonthlyCostUSD: cost}
}

func unknownEst(addr string, ct parser.ChangeType) pricing.Estimate {
	return pricing.Estimate{ResourceAddress: addr, ChangeType: ct, Unknown: true}
}

func finding(sev rules.Severity, cat rules.Category, addr, explanation string) rules.Finding {
	return rules.Finding{Severity: sev, Category: cat, ResourceAddress: addr, Explanation: explanation}
}

// -- Build / Summary --

func TestBuild_NetDelta_AddOnly(t *testing.T) {
	r := Build([]pricing.Estimate{
		est("aws_instance.a", parser.ChangeCreate, 30),
		est("aws_instance.b", parser.ChangeCreate, 70),
	}, nil)
	if r.Summary.TotalAddedUSD != 100 {
		t.Errorf("expected TotalAddedUSD=100, got %.2f", r.Summary.TotalAddedUSD)
	}
	if r.Summary.TotalRemovedUSD != 0 {
		t.Errorf("expected TotalRemovedUSD=0, got %.2f", r.Summary.TotalRemovedUSD)
	}
	if r.Summary.NetDeltaUSD != 100 {
		t.Errorf("expected NetDelta=100, got %.2f", r.Summary.NetDeltaUSD)
	}
}

func TestBuild_NetDelta_MixedChanges(t *testing.T) {
	r := Build([]pricing.Estimate{
		est("aws_instance.new", parser.ChangeCreate, 120),
		est("aws_instance.old", parser.ChangeDelete, 30),
	}, nil)
	if r.Summary.TotalAddedUSD != 120 {
		t.Errorf("expected TotalAddedUSD=120, got %.2f", r.Summary.TotalAddedUSD)
	}
	if r.Summary.TotalRemovedUSD != 30 {
		t.Errorf("expected TotalRemovedUSD=30, got %.2f", r.Summary.TotalRemovedUSD)
	}
	if r.Summary.NetDeltaUSD != 90 {
		t.Errorf("expected NetDelta=90, got %.2f", r.Summary.NetDeltaUSD)
	}
}

func TestBuild_Unknown_Counted(t *testing.T) {
	r := Build([]pricing.Estimate{
		est("aws_instance.a", parser.ChangeCreate, 30),
		unknownEst("aws_s3_bucket.x", parser.ChangeCreate),
		unknownEst("aws_lambda_function.fn", parser.ChangeCreate),
	}, nil)
	if r.Summary.UnknownCount != 2 {
		t.Errorf("expected UnknownCount=2, got %d", r.Summary.UnknownCount)
	}
	if r.Summary.TotalAddedUSD != 30 {
		t.Errorf("unknown estimates should not contribute to cost total, got %.2f", r.Summary.TotalAddedUSD)
	}
}

func TestBuild_FindingCounts(t *testing.T) {
	r := Build(nil, []rules.Finding{
		finding(rules.SeverityCritical, rules.CategoryDestructive, "a", "x"),
		finding(rules.SeverityCritical, rules.CategorySecurity, "b", "y"),
		finding(rules.SeverityWarning, rules.CategorySecurity, "c", "z"),
		finding(rules.SeverityInfo, rules.CategoryCostRisk, "d", "w"),
	})
	if r.Summary.CountBySeverity[rules.SeverityCritical] != 2 {
		t.Errorf("expected 2 criticals, got %d", r.Summary.CountBySeverity[rules.SeverityCritical])
	}
	if r.Summary.CountBySeverity[rules.SeverityWarning] != 1 {
		t.Errorf("expected 1 warning, got %d", r.Summary.CountBySeverity[rules.SeverityWarning])
	}
	if r.Summary.CountBySeverity[rules.SeverityInfo] != 1 {
		t.Errorf("expected 1 info, got %d", r.Summary.CountBySeverity[rules.SeverityInfo])
	}
}

func TestBuild_EmptyInputs(t *testing.T) {
	r := Build(nil, nil)
	if r.Summary.NetDeltaUSD != 0 {
		t.Errorf("empty inputs should produce zero net delta")
	}
	if r.Summary.CountBySeverity == nil {
		t.Error("CountBySeverity should be initialized, not nil")
	}
}

// -- TextFormatter --

func TestText_CleanPlan_NoFindings(t *testing.T) {
	r := Build(nil, nil)
	out, err := (&TextFormatter{}).Format(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "no net cost change") {
		t.Errorf("clean plan should say no net cost change: %s", s)
	}
	if !strings.Contains(s, "no issues found") {
		t.Errorf("clean plan should say no issues found: %s", s)
	}
	if !strings.Contains(s, "Findings: none.") {
		t.Errorf("clean plan should show 'Findings: none.': %s", s)
	}
}

func TestText_CostIncrease_SummaryLine(t *testing.T) {
	r := Build([]pricing.Estimate{
		est("aws_instance.big", parser.ChangeCreate, 560.64),
		est("aws_db_instance.main", parser.ChangeCreate, 12.41),
	}, nil)
	out, err := (&TextFormatter{}).Format(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "adds $573.05/mo") {
		t.Errorf("expected cost-increase summary, got: %s", s)
	}
}

func TestText_Destructive_CriticalHighlighted(t *testing.T) {
	r := Build(nil, []rules.Finding{
		finding(rules.SeverityCritical, rules.CategoryDestructive, "aws_db_instance.prod", "Production DB will be destroyed."),
		finding(rules.SeverityWarning, rules.CategorySecurity, "aws_instance.web", "Port 22 open."),
	})
	out, err := (&TextFormatter{}).Format(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "[CRITICAL]") {
		t.Errorf("expected CRITICAL section in output: %s", s)
	}
	if !strings.Contains(s, "[WARNING]") {
		t.Errorf("expected WARNING section in output: %s", s)
	}
	// CRITICAL must appear before WARNING in output.
	critIdx := strings.Index(s, "[CRITICAL]")
	warnIdx := strings.Index(s, "[WARNING]")
	if critIdx > warnIdx {
		t.Errorf("CRITICAL should appear before WARNING in output")
	}
}

func TestText_UnknownCost_Noted(t *testing.T) {
	r := Build([]pricing.Estimate{
		unknownEst("aws_s3_bucket.assets", parser.ChangeCreate),
	}, nil)
	out, err := (&TextFormatter{}).Format(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "unknown cost") {
		t.Errorf("unknown cost should be noted in output: %s", s)
	}
	if !strings.Contains(s, "unknown") {
		t.Errorf("cost table should show 'unknown' for unpriced resource: %s", s)
	}
}

func TestText_MixedCost_DeleteReflected(t *testing.T) {
	r := Build([]pricing.Estimate{
		est("aws_instance.new", parser.ChangeCreate, 120),
		est("aws_instance.old", parser.ChangeDelete, 30),
	}, nil)
	out, err := (&TextFormatter{}).Format(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "adds $90.00/mo") {
		t.Errorf("expected net delta in summary: %s", s)
	}
}

func TestText_FindingsGroupedBySeverity_OrderCorrect(t *testing.T) {
	r := Build(nil, []rules.Finding{
		finding(rules.SeverityInfo, rules.CategoryCostRisk, "a", "missing tags"),
		finding(rules.SeverityCritical, rules.CategorySecurity, "b", "port 22 open"),
		finding(rules.SeverityWarning, rules.CategoryDestructive, "c", "will replace"),
	})
	out, err := (&TextFormatter{}).Format(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	critIdx := strings.Index(s, "[CRITICAL]")
	warnIdx := strings.Index(s, "[WARNING]")
	infoIdx := strings.Index(s, "[INFO]")
	if !(critIdx < warnIdx && warnIdx < infoIdx) {
		t.Errorf("findings not ordered CRITICAL > WARNING > INFO: crit=%d warn=%d info=%d", critIdx, warnIdx, infoIdx)
	}
}

// -- JSONFormatter --

func TestJSON_ValidJSON(t *testing.T) {
	r := Build([]pricing.Estimate{
		est("aws_instance.web", parser.ChangeCreate, 30.37),
		unknownEst("aws_s3_bucket.assets", parser.ChangeCreate),
	}, []rules.Finding{
		finding(rules.SeverityCritical, rules.CategorySecurity, "aws_instance.web", "port 22 open"),
	})
	out, err := (&JSONFormatter{}).Format(r)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
}

func TestJSON_Structure(t *testing.T) {
	r := Build([]pricing.Estimate{
		est("aws_instance.web", parser.ChangeCreate, 30.37),
	}, []rules.Finding{
		finding(rules.SeverityWarning, rules.CategoryDestructive, "aws_db_instance.db", "will replace"),
	})
	out, _ := (&JSONFormatter{}).Format(r)

	var parsed struct {
		Summary  map[string]interface{} `json:"summary"`
		Costs    []interface{}          `json:"costs"`
		Findings []interface{}          `json:"findings"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Summary == nil {
		t.Error("JSON missing summary key")
	}
	if len(parsed.Costs) != 1 {
		t.Errorf("expected 1 cost entry, got %d", len(parsed.Costs))
	}
	if len(parsed.Findings) != 1 {
		t.Errorf("expected 1 finding entry, got %d", len(parsed.Findings))
	}
}

func TestJSON_EmptyPlan(t *testing.T) {
	r := Build(nil, nil)
	out, err := (&JSONFormatter{}).Format(r)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("empty plan output is not valid JSON: %v", err)
	}
	costs := parsed["costs"].([]interface{})
	if len(costs) != 0 {
		t.Errorf("expected empty costs array, got %d entries", len(costs))
	}
}

func TestJSON_SeverityStrings(t *testing.T) {
	r := Build(nil, []rules.Finding{
		finding(rules.SeverityCritical, rules.CategorySecurity, "x", "explanation"),
	})
	out, _ := (&JSONFormatter{}).Format(r)
	if !strings.Contains(string(out), `"severity": "critical"`) {
		t.Errorf("severity should be lowercase string in JSON: %s", out)
	}
}
