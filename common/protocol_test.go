package common

import (
	"encoding/json"
	"testing"
)

func Test_JSON_Unmarshal(t *testing.T) {
	task := TaskInfo{
		TaskID:    "testTaskID",
		Name:      "test",
		ProjectID: 1,
		FlowInfo: &WorkflowInfo{
			WorkflowID: 1,
		},
	}

	taskBytes, _ := json.Marshal(task)

	newTask := TaskInfo{}
	if err := json.Unmarshal(taskBytes, &newTask); err != nil {
		t.Fatal(err)
	}

	if newTask.FlowInfo == nil {
		t.Log("fail")
	} else {
		t.Log("success")
	}

}
