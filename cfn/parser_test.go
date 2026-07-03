package cfn

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/amt/tf-cost-risk/parser"
)

func fixturePath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "fixtures", name)
}

// -- action mapping --

func TestParse_Add_IsCreate(t *testing.T) {
	changes, err := Parse(makeChangeSet("Add", "AWS::EC2::Instance", "False"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].ChangeType != parser.ChangeCreate {
		t.Errorf("Add should map to create, got %v", changes)
	}
}

func TestParse_Remove_IsDelete(t *testing.T) {
	changes, err := Parse(makeChangeSet("Remove", "AWS::EC2::Instance", "False"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if changes[0].ChangeType != parser.ChangeDelete {
		t.Errorf("Remove should map to delete, got %s", changes[0].ChangeType)
	}
}

func TestParse_Modify_NoReplacement_IsUpdate(t *testing.T) {
	changes, err := Parse(makeChangeSet("Modify", "AWS::EC2::Instance", "False"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if changes[0].ChangeType != parser.ChangeUpdate {
		t.Errorf("Modify+False should map to update, got %s", changes[0].ChangeType)
	}
}

func TestParse_Modify_Replacement_IsReplace(t *testing.T) {
	changes, err := Parse(makeChangeSet("Modify", "AWS::RDS::DBInstance", "True"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if changes[0].ChangeType != parser.ChangeReplace {
		t.Errorf("Modify+True should map to replace, got %s", changes[0].ChangeType)
	}
}

func TestParse_Modify_Conditional_IsReplace(t *testing.T) {
	changes, err := Parse(makeChangeSet("Modify", "AWS::EC2::Instance", "Conditional"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if changes[0].ChangeType != parser.ChangeReplace {
		t.Errorf("Modify+Conditional should map to replace, got %s", changes[0].ChangeType)
	}
}

// -- type mapping --

func TestParse_CFNType_MappedToInternal(t *testing.T) {
	cases := map[string]string{
		"AWS::EC2::Instance":                        "aws_instance",
		"AWS::RDS::DBInstance":                      "aws_db_instance",
		"AWS::RDS::DBCluster":                       "aws_rds_cluster",
		"AWS::EC2::Volume":                          "aws_ebs_volume",
		"AWS::S3::Bucket":                           "aws_s3_bucket",
		"AWS::ElasticLoadBalancingV2::LoadBalancer": "aws_lb",
		"AWS::Lambda::Function":                     "aws_lambda_function",
		"AWS::EC2::SecurityGroup":                   "aws_security_group",
		"AWS::IAM::Policy":                          "aws_iam_policy",
		"AWS::AutoScaling::AutoScalingGroup":        "aws_autoscaling_group",
	}
	for cfnType, want := range cases {
		changes, err := Parse(makeChangeSet("Add", cfnType, "False"), nil)
		if err != nil {
			t.Fatal(err)
		}
		if changes[0].Type != want {
			t.Errorf("%s: expected internal type %s, got %s", cfnType, want, changes[0].Type)
		}
	}
}

func TestParse_UnknownCFNType_PassedThrough(t *testing.T) {
	changes, err := Parse(makeChangeSet("Add", "AWS::CloudFront::Distribution", "False"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if changes[0].Type != "AWS::CloudFront::Distribution" {
		t.Errorf("unknown type should be passed through, got %s", changes[0].Type)
	}
}

// -- address --

func TestParse_LogicalID_IsAddress(t *testing.T) {
	data := singleChange("Add", "AppServer", "AWS::EC2::Instance", "False")
	changes, err := Parse(data, nil)
	if err != nil {
		t.Fatal(err)
	}
	if changes[0].Address != "AppServer" {
		t.Errorf("LogicalResourceId should become Address, got %s", changes[0].Address)
	}
}

// -- template property normalisation --

func TestParse_WithTemplate_EC2_InstanceType(t *testing.T) {
	cs := singleChange("Add", "AppServer", "AWS::EC2::Instance", "False")
	tmpl := template("AppServer", "AWS::EC2::Instance", map[string]interface{}{
		"InstanceType": "m5.4xlarge",
	})
	changes, err := Parse(cs, tmpl)
	if err != nil {
		t.Fatal(err)
	}
	if changes[0].After["instance_type"] != "m5.4xlarge" {
		t.Errorf("InstanceType not normalised, got %v", changes[0].After)
	}
}

func TestParse_WithTemplate_RDS_Properties(t *testing.T) {
	cs := singleChange("Add", "Database", "AWS::RDS::DBInstance", "False")
	tmpl := template("Database", "AWS::RDS::DBInstance", map[string]interface{}{
		"DBInstanceClass":    "db.t3.micro",
		"StorageEncrypted":   true,
		"DeletionProtection": true,
	})
	changes, err := Parse(cs, tmpl)
	if err != nil {
		t.Fatal(err)
	}
	after := changes[0].After
	if after["instance_class"] != "db.t3.micro" {
		t.Errorf("DBInstanceClass not normalised, got %v", after["instance_class"])
	}
	if after["storage_encrypted"] != true {
		t.Errorf("StorageEncrypted not normalised, got %v", after["storage_encrypted"])
	}
	if after["deletion_protection"] != true {
		t.Errorf("DeletionProtection not normalised, got %v", after["deletion_protection"])
	}
}

func TestParse_WithTemplate_EBS_Properties(t *testing.T) {
	cs := singleChange("Remove", "OldVol", "AWS::EC2::Volume", "False")
	tmpl := template("OldVol", "AWS::EC2::Volume", map[string]interface{}{
		"VolumeType": "gp3",
		"Size":       float64(200),
		"Encrypted":  true,
	})
	changes, err := Parse(cs, tmpl)
	if err != nil {
		t.Fatal(err)
	}
	after := changes[0].After
	if after["type"] != "gp3" {
		t.Errorf("VolumeType not normalised, got %v", after["type"])
	}
	if after["size"] != float64(200) {
		t.Errorf("Size not normalised, got %v", after["size"])
	}
}

func TestParse_WithTemplate_SG_IngressNormalised(t *testing.T) {
	cs := singleChange("Add", "BastionSG", "AWS::EC2::SecurityGroup", "False")
	tmpl := template("BastionSG", "AWS::EC2::SecurityGroup", map[string]interface{}{
		"SecurityGroupIngress": []interface{}{
			map[string]interface{}{
				"IpProtocol": "tcp",
				"FromPort":   float64(22),
				"ToPort":     float64(22),
				"CidrIp":     "0.0.0.0/0",
			},
		},
	})
	changes, err := Parse(cs, tmpl)
	if err != nil {
		t.Fatal(err)
	}
	ingress, ok := changes[0].After["ingress"].([]interface{})
	if !ok || len(ingress) != 1 {
		t.Fatalf("ingress not normalised, got %v", changes[0].After["ingress"])
	}
	rule := ingress[0].(map[string]interface{})
	if rule["from_port"] != float64(22) {
		t.Errorf("from_port not normalised, got %v", rule["from_port"])
	}
	cidrs, _ := rule["cidr_blocks"].([]interface{})
	if len(cidrs) == 0 || cidrs[0] != "0.0.0.0/0" {
		t.Errorf("cidr_blocks not normalised, got %v", cidrs)
	}
}

func TestParse_WithTemplate_IAMRole_PolicyDocument(t *testing.T) {
	cs := singleChange("Add", "LambdaExecRole", "AWS::IAM::Role", "False")
	tmpl := template("LambdaExecRole", "AWS::IAM::Role", map[string]interface{}{
		"Policies": []interface{}{
			map[string]interface{}{
				"PolicyName": "FullAccess",
				"PolicyDocument": map[string]interface{}{
					"Statement": []interface{}{
						map[string]interface{}{"Effect": "Allow", "Action": "*", "Resource": "*"},
					},
				},
			},
		},
	})
	changes, err := Parse(cs, tmpl)
	if err != nil {
		t.Fatal(err)
	}
	policy, ok := changes[0].After["policy"].(string)
	if !ok || policy == "" {
		t.Errorf("policy document not extracted, got %v", changes[0].After["policy"])
	}
}

func TestParse_WithTemplate_Tags_FlatMap(t *testing.T) {
	cs := singleChange("Add", "AppServer", "AWS::EC2::Instance", "False")
	tmpl := template("AppServer", "AWS::EC2::Instance", map[string]interface{}{
		"InstanceType": "t3.medium",
		"Tags": []interface{}{
			map[string]interface{}{"Key": "Env", "Value": "prod"},
			map[string]interface{}{"Key": "Team", "Value": "platform"},
		},
	})
	changes, err := Parse(cs, tmpl)
	if err != nil {
		t.Fatal(err)
	}
	tags, ok := changes[0].After["tags"].(map[string]interface{})
	if !ok {
		t.Fatalf("tags not normalised to map, got %T", changes[0].After["tags"])
	}
	if tags["Env"] != "prod" || tags["Team"] != "platform" {
		t.Errorf("tags values wrong, got %v", tags)
	}
}

// -- fixture files --

func TestParseFile_ChangeSetOnly(t *testing.T) {
	changes, err := ParseFile(fixturePath("cfn_changeset.json"), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 7 {
		t.Errorf("expected 7 changes from fixture, got %d", len(changes))
	}
}

func TestParseFile_WithTemplate(t *testing.T) {
	changes, err := ParseFile(fixturePath("cfn_changeset.json"), fixturePath("cfn_template.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 7 {
		t.Errorf("expected 7 changes, got %d", len(changes))
	}
	// AppServer should have instance_type from template
	for _, c := range changes {
		if c.Address == "AppServer" {
			if c.After["instance_type"] != "m5.4xlarge" {
				t.Errorf("AppServer instance_type not loaded from template, got %v", c.After["instance_type"])
			}
		}
	}
}

func TestParseFile_NotFound(t *testing.T) {
	_, err := ParseFile("/does/not/exist.json", "")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestParse_InvalidJSON(t *testing.T) {
	_, err := Parse([]byte("not json"), nil)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// -- helpers --

func makeChangeSet(action, cfnType, replacement string) []byte {
	return singleChange(action, "Resource1", cfnType, replacement)
}

func singleChange(action, logicalID, cfnType, replacement string) []byte {
	return []byte(`{
		"ChangeSetName": "test",
		"StackName": "test-stack",
		"Changes": [{
			"Type": "Resource",
			"ResourceChange": {
				"Action": "` + action + `",
				"LogicalResourceId": "` + logicalID + `",
				"ResourceType": "` + cfnType + `",
				"Replacement": "` + replacement + `"
			}
		}]
	}`)
}

func template(logicalID, cfnType string, props map[string]interface{}) []byte {
	tmpl := map[string]interface{}{
		"Resources": map[string]interface{}{
			logicalID: map[string]interface{}{
				"Type":       cfnType,
				"Properties": props,
			},
		},
	}
	b, _ := json.Marshal(tmpl)
	return b
}
