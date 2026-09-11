package tofu_test

import (
	"testing"

	tfjson "github.com/hashicorp/terraform-json"

	"github.com/iagram/iagram/internal/tofu"
)

func TestSummarizeAggregatesBySeverity(t *testing.T) {
	p := &tfjson.Plan{ResourceChanges: []*tfjson.ResourceChange{
		{Address: "module.web.aws_instance.this", Type: "aws_instance", Change: &tfjson.Change{Actions: tfjson.Actions{tfjson.ActionCreate}}},
		{Address: "module.web.aws_security_group.instance", Type: "aws_security_group", Change: &tfjson.Change{Actions: tfjson.Actions{tfjson.ActionDelete, tfjson.ActionCreate}}},
		{Address: "module.db.aws_db_instance.this", Type: "aws_db_instance", Change: &tfjson.Change{Actions: tfjson.Actions{tfjson.ActionUpdate}}},
		{Address: "module.db.aws_db_subnet_group.this", Type: "aws_db_subnet_group", Change: &tfjson.Change{Actions: tfjson.Actions{tfjson.ActionNoop}}},
		{Address: "module.gone.aws_s3_bucket.this", Type: "aws_s3_bucket", Change: &tfjson.Change{Actions: tfjson.Actions{tfjson.ActionDelete}}},
		{Address: "module.web.data.aws_ssm_parameter.al2023[0]", Type: "aws_ssm_parameter", Mode: tfjson.DataResourceMode, Change: &tfjson.Change{Actions: tfjson.Actions{tfjson.ActionRead}}},
	}}
	s := tofu.Summarize(p, map[string]string{"web": "web-id", "db": "db-id"})
	if s.Add != 2 || s.Change != 1 || s.Destroy != 2 {
		t.Errorf("counts = %d/%d/%d", s.Add, s.Change, s.Destroy)
	}
	if s.Nodes["web-id"].Action != tofu.ActionReplace || len(s.Nodes["web-id"].Resources) != 3 {
		t.Errorf("web = %+v", s.Nodes["web-id"])
	}
	if s.Nodes["db-id"].Action != tofu.ActionUpdate {
		t.Errorf("db = %+v", s.Nodes["db-id"])
	}
	if len(s.Orphans) != 1 || s.Orphans[0].Address != "module.gone.aws_s3_bucket.this" {
		t.Errorf("orphans = %+v", s.Orphans)
	}
}

func TestChangedKeysOnUpdate(t *testing.T) {
	p := &tfjson.Plan{ResourceChanges: []*tfjson.ResourceChange{
		{Address: "module.web.aws_instance.this", Type: "aws_instance", Change: &tfjson.Change{
			Actions: tfjson.Actions{tfjson.ActionUpdate},
			Before:  map[string]any{"instance_type": "t3.micro", "tags": map[string]any{"a": "1"}, "id": "i-1"},
			After:   map[string]any{"instance_type": "t3.small", "tags": map[string]any{"a": "1"}, "id": "i-1", "extra": true},
		}},
	}}
	s := tofu.Summarize(p, map[string]string{"web": "web"})
	got := s.Nodes["web"].Resources[0].Changed
	if len(got) != 2 || got[0] != "extra" || got[1] != "instance_type" {
		t.Errorf("changed = %v", got)
	}
}
