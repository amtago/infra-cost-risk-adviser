package aws

import (
	"testing"

	"github.com/amt/tf-cost-risk/normalizer"
	"github.com/amt/tf-cost-risk/parser"
	"github.com/amt/tf-cost-risk/pricing"
	"github.com/amt/tf-cost-risk/rules"
)

func ctxWithEstimates(resources []normalizer.NormalizedResource, estimates []pricing.Estimate) rules.EvaluateContext {
	return rules.EvaluateContext{Resources: resources, Estimates: estimates}
}

func est(address string, cost float64) pricing.Estimate {
	return pricing.Estimate{ResourceAddress: address, MonthlyCostUSD: cost}
}

func unknownEst(address string) pricing.Estimate {
	return pricing.Estimate{ResourceAddress: address, Unknown: true}
}

// -- OversizedResourceRule --

func TestOversized_FlagsExpensiveOutlier(t *testing.T) {
	r := &OversizedResourceRule{OversizeMultiple: 5}
	// median of [10, 10, 10, 10] = 10; threshold = 50
	// big resource at 100 should be flagged
	resources := []normalizer.NormalizedResource{
		{Address: "aws_instance.a", Provider: "aws", ResourceType: "aws_instance", ChangeType: parser.ChangeCreate},
		{Address: "aws_instance.b", Provider: "aws", ResourceType: "aws_instance", ChangeType: parser.ChangeCreate},
		{Address: "aws_instance.c", Provider: "aws", ResourceType: "aws_instance", ChangeType: parser.ChangeCreate},
		{Address: "aws_instance.big", Provider: "aws", ResourceType: "aws_instance", ChangeType: parser.ChangeCreate},
	}
	estimates := []pricing.Estimate{
		est("aws_instance.a", 10),
		est("aws_instance.b", 10),
		est("aws_instance.c", 10),
		est("aws_instance.big", 100),
	}
	findings := r.Evaluate(ctxWithEstimates(resources, estimates))
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].ResourceAddress != "aws_instance.big" {
		t.Errorf("expected aws_instance.big to be flagged, got %s", findings[0].ResourceAddress)
	}
	if findings[0].Severity != rules.SeverityWarning {
		t.Errorf("expected warning, got %s", findings[0].Severity)
	}
	if findings[0].Category != rules.CategoryCostRisk {
		t.Errorf("expected cost-risk category")
	}
}

func TestOversized_AllSimilarCost_NoFinding(t *testing.T) {
	r := &OversizedResourceRule{OversizeMultiple: 5}
	resources := []normalizer.NormalizedResource{
		{Address: "aws_instance.a", Provider: "aws", ChangeType: parser.ChangeCreate},
		{Address: "aws_instance.b", Provider: "aws", ChangeType: parser.ChangeCreate},
		{Address: "aws_instance.c", Provider: "aws", ChangeType: parser.ChangeCreate},
	}
	estimates := []pricing.Estimate{
		est("aws_instance.a", 30),
		est("aws_instance.b", 35),
		est("aws_instance.c", 32),
	}
	findings := r.Evaluate(ctxWithEstimates(resources, estimates))
	if len(findings) != 0 {
		t.Errorf("similar-cost resources should not trigger oversized rule, got %d", len(findings))
	}
}

func TestOversized_UnknownCosts_Excluded(t *testing.T) {
	r := &OversizedResourceRule{OversizeMultiple: 5}
	resources := []normalizer.NormalizedResource{
		{Address: "aws_instance.a", Provider: "aws", ChangeType: parser.ChangeCreate},
		{Address: "aws_s3_bucket.x", Provider: "aws", ChangeType: parser.ChangeCreate},
		{Address: "aws_instance.big", Provider: "aws", ChangeType: parser.ChangeCreate},
	}
	estimates := []pricing.Estimate{
		est("aws_instance.a", 10),
		unknownEst("aws_s3_bucket.x"),
		est("aws_instance.big", 100),
	}
	findings := r.Evaluate(ctxWithEstimates(resources, estimates))
	// median of [10, 100] = 55; 100 < 55*5=275 — no finding
	if len(findings) != 0 {
		t.Errorf("with only 2 known prices and no outlier over threshold, expected no finding, got %d", len(findings))
	}
}

func TestOversized_SingleResource_NoFinding(t *testing.T) {
	// Need at least 2 priced resources to compute median.
	r := &OversizedResourceRule{OversizeMultiple: 5}
	resources := []normalizer.NormalizedResource{
		{Address: "aws_instance.solo", Provider: "aws", ChangeType: parser.ChangeCreate},
	}
	findings := r.Evaluate(ctxWithEstimates(resources, []pricing.Estimate{est("aws_instance.solo", 1000)}))
	if len(findings) != 0 {
		t.Errorf("single resource cannot have a median comparison, expected no finding")
	}
}

func TestOversized_DeleteExcluded(t *testing.T) {
	r := &OversizedResourceRule{OversizeMultiple: 5}
	resources := []normalizer.NormalizedResource{
		{Address: "aws_instance.a", Provider: "aws", ChangeType: parser.ChangeCreate},
		{Address: "aws_instance.b", Provider: "aws", ChangeType: parser.ChangeCreate},
		{Address: "aws_instance.dying", Provider: "aws", ChangeType: parser.ChangeDelete},
	}
	estimates := []pricing.Estimate{
		est("aws_instance.a", 10),
		est("aws_instance.b", 10),
		est("aws_instance.dying", 1000), // delete — should be excluded from comparison
	}
	findings := r.Evaluate(ctxWithEstimates(resources, estimates))
	if len(findings) != 0 {
		t.Errorf("deleted resource should be excluded from oversized check, got %d findings", len(findings))
	}
}

// -- MissingCostTagsRule --

func taggedNR(address string, tags map[string]string) normalizer.NormalizedResource {
	raw := map[string]interface{}{"tags": map[string]interface{}{}}
	tagRaw := map[string]interface{}{}
	for k, v := range tags {
		tagRaw[k] = v
	}
	raw["tags"] = tagRaw
	return normalizer.NormalizedResource{
		Address:      address,
		Provider:     "aws",
		ResourceType: "aws_instance",
		ChangeType:   parser.ChangeCreate,
		Tags:         tags,
		Raw:          raw,
	}
}

func TestMissingTags_BothMissing_Info(t *testing.T) {
	r := &MissingCostTagsRule{RequiredTags: []string{"Env", "Team"}}
	findings := r.Evaluate(ctx(taggedNR("aws_instance.web", map[string]string{})))
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for missing tags, got %d", len(findings))
	}
	if findings[0].Severity != rules.SeverityInfo {
		t.Errorf("expected info, got %s", findings[0].Severity)
	}
	if findings[0].Category != rules.CategoryCostRisk {
		t.Errorf("expected cost-risk category")
	}
}

func TestMissingTags_AllPresent_NoFinding(t *testing.T) {
	r := &MissingCostTagsRule{RequiredTags: []string{"Env", "Team"}}
	findings := r.Evaluate(ctx(taggedNR("aws_instance.web", map[string]string{"Env": "prod", "Team": "platform"})))
	if len(findings) != 0 {
		t.Errorf("all required tags present should produce no finding")
	}
}

func TestMissingTags_PartiallyMissing(t *testing.T) {
	r := &MissingCostTagsRule{RequiredTags: []string{"Env", "Team"}}
	findings := r.Evaluate(ctx(taggedNR("aws_instance.web", map[string]string{"Env": "prod"})))
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for partially missing tags, got %d", len(findings))
	}
}

func TestMissingTags_Delete_Skipped(t *testing.T) {
	r := &MissingCostTagsRule{RequiredTags: []string{"Env", "Team"}}
	nr := taggedNR("aws_instance.old", map[string]string{})
	nr.ChangeType = parser.ChangeDelete
	findings := r.Evaluate(ctx(nr))
	if len(findings) != 0 {
		t.Errorf("delete should not trigger missing tags rule")
	}
}

func TestMissingTags_NoTagsAttr_Skipped(t *testing.T) {
	// Resource has no "tags" key in Raw — skip rather than false-positive.
	r := &MissingCostTagsRule{RequiredTags: []string{"Env", "Team"}}
	nr := normalizer.NormalizedResource{
		Address:      "aws_iam_role.worker",
		Provider:     "aws",
		ResourceType: "aws_iam_role",
		ChangeType:   parser.ChangeCreate,
		Tags:         map[string]string{},
		Raw:          map[string]interface{}{}, // no "tags" key
	}
	findings := r.Evaluate(ctx(nr))
	if len(findings) != 0 {
		t.Errorf("resource without tags attr should not trigger missing tags rule")
	}
}

func TestMissingTags_EmptyRequiredList_NoFinding(t *testing.T) {
	r := &MissingCostTagsRule{RequiredTags: []string{}}
	findings := r.Evaluate(ctx(taggedNR("aws_instance.web", map[string]string{})))
	if len(findings) != 0 {
		t.Errorf("empty required tags list should produce no findings")
	}
}

// -- UnboundedAutoscalingRule --

func asgNR(address string, maxSize int) normalizer.NormalizedResource {
	return normalizer.NormalizedResource{
		Address:      address,
		Provider:     "aws",
		ResourceType: "aws_autoscaling_group",
		ChangeType:   parser.ChangeCreate,
		Raw:          map[string]interface{}{"max_size": float64(maxSize)},
	}
}

func TestAutoscaling_NoMaxSize_Warning(t *testing.T) {
	r := &UnboundedAutoscalingRule{}
	nr := normalizer.NormalizedResource{
		Address:      "aws_autoscaling_group.workers",
		Provider:     "aws",
		ResourceType: "aws_autoscaling_group",
		ChangeType:   parser.ChangeCreate,
		Raw:          map[string]interface{}{}, // no max_size key
	}
	findings := r.Evaluate(ctx(nr))
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for missing max_size, got %d", len(findings))
	}
	if findings[0].Severity != rules.SeverityWarning {
		t.Errorf("expected warning, got %s", findings[0].Severity)
	}
	if findings[0].Category != rules.CategoryCostRisk {
		t.Errorf("expected cost-risk category")
	}
}

func TestAutoscaling_ZeroMaxSize_Warning(t *testing.T) {
	r := &UnboundedAutoscalingRule{}
	findings := r.Evaluate(ctx(asgNR("aws_autoscaling_group.workers", 0)))
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for max_size=0, got %d", len(findings))
	}
}

func TestAutoscaling_BoundedMaxSize_NoFinding(t *testing.T) {
	r := &UnboundedAutoscalingRule{}
	findings := r.Evaluate(ctx(asgNR("aws_autoscaling_group.workers", 10)))
	if len(findings) != 0 {
		t.Errorf("bounded ASG should not trigger finding, got %d", len(findings))
	}
}

func TestAutoscaling_Delete_Skipped(t *testing.T) {
	r := &UnboundedAutoscalingRule{}
	nr := asgNR("aws_autoscaling_group.old", 0)
	nr.ChangeType = parser.ChangeDelete
	findings := r.Evaluate(ctx(nr))
	if len(findings) != 0 {
		t.Errorf("delete should not trigger autoscaling rule")
	}
}

func TestAutoscaling_NonASGResource_NoFinding(t *testing.T) {
	r := &UnboundedAutoscalingRule{}
	findings := r.Evaluate(ctx(nrBasic("aws_instance", "aws_instance.web", parser.ChangeCreate, false, nil)))
	if len(findings) != 0 {
		t.Errorf("non-ASG resource should not trigger autoscaling rule")
	}
}

// -- Run (integration) --

func TestRun_AllRules_DestructivePlan(t *testing.T) {
	resources := []normalizer.NormalizedResource{
		{
			Address:      "aws_db_instance.prod",
			Provider:     "aws",
			ResourceType: "aws_db_instance",
			ChangeType:   parser.ChangeReplace,
			Stateful:     true,
			Raw:          map[string]interface{}{"deletion_protection": false, "storage_encrypted": false},
		},
	}
	findings := Run(rules.EvaluateContext{Resources: resources}, AllRules())
	// Expect at least: destructive replace (critical), missing deletion protection, unencrypted storage
	if len(findings) < 3 {
		t.Errorf("destructive plan should produce at least 3 findings, got %d", len(findings))
	}
	hasCritical := false
	for _, f := range findings {
		if f.Severity == rules.SeverityCritical {
			hasCritical = true
		}
	}
	if !hasCritical {
		t.Error("expected at least one critical finding for stateful replace")
	}
}
