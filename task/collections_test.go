package task

import (
	"reflect"
	"testing"

	"github.com/boolean-maybe/tiki/workflow"
)

func TestNormalizeCollectionFields_BuiltIns(t *testing.T) {
	task := &Task{
		Tags:      []string{"frontend", " frontend ", "backend", ""},
		DependsOn: []string{"tiki-aaa001", "TIKI-AAA001", "tiki-bbb002"},
	}

	NormalizeCollectionFields(task)

	if !reflect.DeepEqual(task.Tags, []string{"frontend", "backend"}) {
		t.Errorf("tags = %v, want [frontend backend]", task.Tags)
	}
	if !reflect.DeepEqual(task.DependsOn, []string{"TIKI-AAA001", "TIKI-BBB002"}) {
		t.Errorf("dependsOn = %v, want [TIKI-AAA001 TIKI-BBB002]", task.DependsOn)
	}
}

func TestNormalizeCollectionFields_CustomLists(t *testing.T) {
	workflow.ClearCustomFields()
	t.Cleanup(func() { workflow.ClearCustomFields() })
	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "labels", Type: workflow.TypeListString, Custom: true},
		{Name: "related", Type: workflow.TypeListRef, Custom: true},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	task := &Task{
		CustomFields: map[string]interface{}{
			"labels":  []string{"backend", " backend ", "backend"},
			"related": []string{"tiki-aaa001", "TIKI-AAA001", "tiki-bbb002"},
		},
	}

	NormalizeCollectionFields(task)

	labels, ok := task.CustomFields["labels"].([]string)
	if !ok {
		t.Fatalf("labels type = %T, want []string", task.CustomFields["labels"])
	}
	if !reflect.DeepEqual(labels, []string{"backend"}) {
		t.Errorf("labels = %v, want [backend]", labels)
	}

	related, ok := task.CustomFields["related"].([]string)
	if !ok {
		t.Fatalf("related type = %T, want []string", task.CustomFields["related"])
	}
	if !reflect.DeepEqual(related, []string{"TIKI-AAA001", "TIKI-BBB002"}) {
		t.Errorf("related = %v, want [TIKI-AAA001 TIKI-BBB002]", related)
	}
}
