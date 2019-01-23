package etcd_func

import (
	"github.com/gin-gonic/gin"
	"ojbk.io/gopherCron/cmd/service/request"
	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/pkg/etcd"
	"ojbk.io/gopherCron/utils"
)

// ExecuteTask 手动调用任务的请求参数
type ExecuteTaskRequest struct {
	ProjectID string `form:"project_id" binding:"required"`
	TaskID    string `form:"task_id" binding:"required"`
}

// ExecuteTask 立即执行某个任务
func ExecuteTask(c *gin.Context) {
	var (
		req  ExecuteTaskRequest
		err  error
		task *common.TaskInfo
		res  []string
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if res, err = etcd.Manager.GetWorkerList(req.ProjectID); err != nil {
		request.APIError(c, err)
		return
	}

	if len(res) == 0 {
		request.APIError(c, errors.ErrNoWorkingNode)
		return
	}

	if task, err = etcd.Manager.GetTask(req.ProjectID, req.TaskID); err != nil {
		request.APIError(c, err)
		return
	}

	// 调用etcd的put方法以出发watcher从而调度该任务
	if err = etcd.Manager.TemporarySchedulerTask(task); err != nil {
		request.APIError(c, err)
		return
	}

	request.APISuccess(c, nil)
}
