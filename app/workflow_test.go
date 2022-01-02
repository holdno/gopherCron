package app

import (
	"fmt"
	"testing"
)

func Test_kahn(t *testing.T) {
	exp := map[WorkflowTaskInfo][]WorkflowTaskInfo{
		WorkflowTaskInfo{1, "1"}: []WorkflowTaskInfo{},
		WorkflowTaskInfo{2, "2"}: []WorkflowTaskInfo{{1, "1"}},
		WorkflowTaskInfo{3, "3"}: []WorkflowTaskInfo{{2, "2"}, {1, "1"}},
		WorkflowTaskInfo{4, "4"}: []WorkflowTaskInfo{{3, "3"}},
		WorkflowTaskInfo{5, "5"}: []WorkflowTaskInfo{{3, "3"}},
		WorkflowTaskInfo{6, "6"}: []WorkflowTaskInfo{{2, "2"}},
	}
	fmt.Println(kahn(exp))
}
