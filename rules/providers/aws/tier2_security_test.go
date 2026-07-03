package aws

import (
	"testing"

	"github.com/amt/tf-cost-risk/normalizer"
	"github.com/amt/tf-cost-risk/parser"
	"github.com/amt/tf-cost-risk/rules"
)

// -- OpenSecurityGroupRule --

func sgNR(address string, changeType parser.ChangeType, ingress []interface{}) normalizer.NormalizedResource {
	return normalizer.NormalizedResource{
		Address:      address,
		ResourceType: "aws_security_group",
		ChangeType:   changeType,
		Raw:          map[string]interface{}{"ingress": ingress},
	}
}

func ingressRule(fromPort, toPort int, cidr string) map[string]interface{} {
	return map[string]interface{}{
		"from_port":   float64(fromPort),
		"to_port":     float64(toPort),
		"protocol":    "tcp",
		"cidr_blocks": []interface{}{cidr},
	}
}

func TestSG_SSH_Open_Critical(t *testing.T) {
	r := &OpenSecurityGroupRule{}
	findings := r.Evaluate(ctx(sgNR("aws_security_group.web", parser.ChangeCreate,
		[]interface{}{ingressRule(22, 22, "0.0.0.0/0")})))
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for SSH open, got %d", len(findings))
	}
	if findings[0].Severity != rules.SeverityCritical {
		t.Errorf("expected critical, got %s", findings[0].Severity)
	}
}

func TestSG_RDP_Open_Critical(t *testing.T) {
	r := &OpenSecurityGroupRule{}
	findings := r.Evaluate(ctx(sgNR("aws_security_group.rdp", parser.ChangeCreate,
		[]interface{}{ingressRule(3389, 3389, "0.0.0.0/0")})))
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for RDP open, got %d", len(findings))
	}
}

func TestSG_DBPort_Open_Critical(t *testing.T) {
	r := &OpenSecurityGroupRule{}
	findings := r.Evaluate(ctx(sgNR("aws_security_group.db", parser.ChangeCreate,
		[]interface{}{ingressRule(5432, 5432, "0.0.0.0/0")})))
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for Postgres open, got %d", len(findings))
	}
}

func TestSG_WideRange_CatchesSensitivePort(t *testing.T) {
	// A wide port range 0-65535 should catch all sensitive ports.
	r := &OpenSecurityGroupRule{}
	findings := r.Evaluate(ctx(sgNR("aws_security_group.wide", parser.ChangeCreate,
		[]interface{}{ingressRule(0, 65535, "0.0.0.0/0")})))
	if len(findings) == 0 {
		t.Error("wide open range should catch sensitive ports")
	}
}

func TestSG_PrivateCIDR_NoFinding(t *testing.T) {
	r := &OpenSecurityGroupRule{}
	findings := r.Evaluate(ctx(sgNR("aws_security_group.internal", parser.ChangeCreate,
		[]interface{}{ingressRule(22, 22, "10.0.0.0/8")})))
	if len(findings) != 0 {
		t.Errorf("private CIDR should not trigger finding, got %d", len(findings))
	}
}

func TestSG_NonSensitivePort_NoFinding(t *testing.T) {
	r := &OpenSecurityGroupRule{}
	findings := r.Evaluate(ctx(sgNR("aws_security_group.web", parser.ChangeCreate,
		[]interface{}{ingressRule(80, 80, "0.0.0.0/0")})))
	if len(findings) != 0 {
		t.Errorf("port 80 open is not flagged as sensitive, got %d findings", len(findings))
	}
}

func TestSG_Delete_NoFinding(t *testing.T) {
	r := &OpenSecurityGroupRule{}
	findings := r.Evaluate(ctx(sgNR("aws_security_group.old", parser.ChangeDelete,
		[]interface{}{ingressRule(22, 22, "0.0.0.0/0")})))
	if len(findings) != 0 {
		t.Errorf("deletes should not trigger security group rule")
	}
}

func TestSG_NoIngress_NoFinding(t *testing.T) {
	r := &OpenSecurityGroupRule{}
	nr := normalizer.NormalizedResource{
		Address:      "aws_security_group.egress_only",
		ResourceType: "aws_security_group",
		ChangeType:   parser.ChangeCreate,
		Raw:          map[string]interface{}{},
	}
	findings := r.Evaluate(ctx(nr))
	if len(findings) != 0 {
		t.Errorf("no ingress key should produce no findings")
	}
}

// -- PublicS3BucketRule --

func s3NR(address string, acl string) normalizer.NormalizedResource {
	return normalizer.NormalizedResource{
		Address:      address,
		ResourceType: "aws_s3_bucket",
		ChangeType:   parser.ChangeCreate,
		Raw:          map[string]interface{}{"acl": acl},
	}
}

func TestS3_PublicRead_Critical(t *testing.T) {
	r := &PublicS3BucketRule{}
	findings := r.Evaluate(ctx(s3NR("aws_s3_bucket.assets", "public-read")))
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for public-read, got %d", len(findings))
	}
	if findings[0].Severity != rules.SeverityCritical {
		t.Errorf("expected critical, got %s", findings[0].Severity)
	}
}

func TestS3_PublicReadWrite_Critical(t *testing.T) {
	r := &PublicS3BucketRule{}
	findings := r.Evaluate(ctx(s3NR("aws_s3_bucket.rw", "public-read-write")))
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for public-read-write, got %d", len(findings))
	}
}

func TestS3_Private_NoFinding(t *testing.T) {
	r := &PublicS3BucketRule{}
	findings := r.Evaluate(ctx(s3NR("aws_s3_bucket.private", "private")))
	if len(findings) != 0 {
		t.Errorf("private ACL should not trigger finding")
	}
}

func TestS3_NoACL_NoFinding(t *testing.T) {
	r := &PublicS3BucketRule{}
	nr := normalizer.NormalizedResource{
		Address:      "aws_s3_bucket.noacl",
		ResourceType: "aws_s3_bucket",
		ChangeType:   parser.ChangeCreate,
		Raw:          map[string]interface{}{},
	}
	findings := r.Evaluate(ctx(nr))
	if len(findings) != 0 {
		t.Errorf("no ACL attr should not trigger finding")
	}
}

// -- UnencryptedStorageRule --

func TestEncryption_RDS_NoEncryption_Warning(t *testing.T) {
	r := &UnencryptedStorageRule{}
	findings := r.Evaluate(ctx(nrBasic("aws_db_instance", "aws_db_instance.main", parser.ChangeCreate, true,
		map[string]interface{}{"storage_encrypted": false})))
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for unencrypted RDS, got %d", len(findings))
	}
	if findings[0].Severity != rules.SeverityWarning {
		t.Errorf("expected warning, got %s", findings[0].Severity)
	}
}

func TestEncryption_RDS_Encrypted_NoFinding(t *testing.T) {
	r := &UnencryptedStorageRule{}
	findings := r.Evaluate(ctx(nrBasic("aws_db_instance", "aws_db_instance.main", parser.ChangeCreate, true,
		map[string]interface{}{"storage_encrypted": true})))
	if len(findings) != 0 {
		t.Errorf("encrypted RDS should not trigger finding")
	}
}

func TestEncryption_EBS_NoEncryption_Warning(t *testing.T) {
	r := &UnencryptedStorageRule{}
	findings := r.Evaluate(ctx(nrBasic("aws_ebs_volume", "aws_ebs_volume.data", parser.ChangeCreate, true,
		map[string]interface{}{"encrypted": false, "size": float64(100)})))
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for unencrypted EBS, got %d", len(findings))
	}
}

func TestEncryption_EBS_Encrypted_NoFinding(t *testing.T) {
	r := &UnencryptedStorageRule{}
	findings := r.Evaluate(ctx(nrBasic("aws_ebs_volume", "aws_ebs_volume.data", parser.ChangeCreate, true,
		map[string]interface{}{"encrypted": true, "size": float64(100)})))
	if len(findings) != 0 {
		t.Errorf("encrypted EBS should not trigger finding")
	}
}

func TestEncryption_Delete_Skipped(t *testing.T) {
	r := &UnencryptedStorageRule{}
	findings := r.Evaluate(ctx(nrBasic("aws_db_instance", "aws_db_instance.old", parser.ChangeDelete, true,
		map[string]interface{}{"storage_encrypted": false})))
	if len(findings) != 0 {
		t.Errorf("delete should not trigger encryption rule")
	}
}

// -- WildcardIAMRule --

func iamNR(address, resourceType, policy string) normalizer.NormalizedResource {
	return normalizer.NormalizedResource{
		Address:      address,
		ResourceType: resourceType,
		ChangeType:   parser.ChangeCreate,
		Raw:          map[string]interface{}{"policy": policy},
	}
}

const wildcardActionPolicy = `{"Statement":[{"Effect":"Allow","Action":"*","Resource":"arn:aws:s3:::my-bucket/*"}]}`
const wildcardResourcePolicy = `{"Statement":[{"Effect":"Allow","Action":["s3:GetObject"],"Resource":"*"}]}`
const wildcardBothPolicy = `{"Statement":[{"Effect":"Allow","Action":"*","Resource":"*"}]}`
const safePolicy = `{"Statement":[{"Effect":"Allow","Action":["s3:GetObject"],"Resource":"arn:aws:s3:::my-bucket/*"}]}`

func TestIAM_WildcardAction_Critical(t *testing.T) {
	r := &WildcardIAMRule{}
	findings := r.Evaluate(ctx(iamNR("aws_iam_policy.admin", "aws_iam_policy", wildcardActionPolicy)))
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for wildcard action, got %d", len(findings))
	}
	if findings[0].Severity != rules.SeverityCritical {
		t.Errorf("wildcard action should be critical, got %s", findings[0].Severity)
	}
}

func TestIAM_WildcardResource_Warning(t *testing.T) {
	r := &WildcardIAMRule{}
	findings := r.Evaluate(ctx(iamNR("aws_iam_policy.broad", "aws_iam_policy", wildcardResourcePolicy)))
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for wildcard resource, got %d", len(findings))
	}
	if findings[0].Severity != rules.SeverityWarning {
		t.Errorf("wildcard resource should be warning, got %s", findings[0].Severity)
	}
}

func TestIAM_WildcardBoth_TwoFindings(t *testing.T) {
	r := &WildcardIAMRule{}
	findings := r.Evaluate(ctx(iamNR("aws_iam_policy.superadmin", "aws_iam_policy", wildcardBothPolicy)))
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings for wildcard action+resource, got %d", len(findings))
	}
}

func TestIAM_ScopedPolicy_NoFinding(t *testing.T) {
	r := &WildcardIAMRule{}
	findings := r.Evaluate(ctx(iamNR("aws_iam_policy.safe", "aws_iam_policy", safePolicy)))
	if len(findings) != 0 {
		t.Errorf("scoped policy should not trigger finding, got %d", len(findings))
	}
}

func TestIAM_RolePolicy_Checked(t *testing.T) {
	r := &WildcardIAMRule{}
	findings := r.Evaluate(ctx(iamNR("aws_iam_role_policy.admin", "aws_iam_role_policy", wildcardActionPolicy)))
	if len(findings) != 1 {
		t.Fatalf("aws_iam_role_policy should also be checked, got %d findings", len(findings))
	}
}

func TestIAM_Delete_Skipped(t *testing.T) {
	r := &WildcardIAMRule{}
	nr := iamNR("aws_iam_policy.old", "aws_iam_policy", wildcardActionPolicy)
	nr.ChangeType = parser.ChangeDelete
	findings := r.Evaluate(ctx(nr))
	if len(findings) != 0 {
		t.Errorf("delete should not trigger IAM rule")
	}
}
