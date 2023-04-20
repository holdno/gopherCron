package etcd_func

import (
	"fmt"

	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/utils"

	"github.com/gin-gonic/gin"
)

type TmpExecuteRequest struct {
	ProjectID int64  `form:"project_id" json:"project_id" binding:"required"`
	Name      string `form:"name" json:"name" binding:"required"`
	Command   string `form:"command" json:"command" binding:"required"`
	Timeout   int    `form:"timeout" json:"timeout" binding:"required"`
	Noseize   int    `form:"noseize" json:"noseize"`
}

func TmpExecute(c *gin.Context) {
	var (
		req TmpExecuteRequest
		err error
		res []common.ClientInfo

		srv = app.GetApp(c)
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if res, err = srv.GetWorkerList(req.ProjectID); err != nil {
		response.APIError(c, err)
		return
	}
	if len(res) == 0 {
		response.APIError(c, errors.ErrNoWorkingNode)
		return
	}

	taskID := fmt.Sprintf("tmp_%s", utils.GetStrID())
	// 调用etcd的put方法以触发watcher从而调度该任务
	if err = srv.TemporarySchedulerTask(&common.TaskInfo{
		TaskID:    taskID,
		ProjectID: req.ProjectID,
		Name:      req.Name,
		Command:   req.Command,
		Timeout:   req.Timeout,
		Noseize:   req.Noseize,
	}); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, taskID)
}

// ExecuteTask 手动调用任务的请求参数
type ExecuteTaskRequest struct {
	ProjectID int64  `form:"project_id" json:"project_id" binding:"required"`
	TaskID    string `form:"task_id" json:"task_id" binding:"required"`
}

// ExecuteTask 立即执行某个任务
func ExecuteTask(c *gin.Context) {
	var (
		req  ExecuteTaskRequest
		err  error
		task *common.TaskInfo
		res  []common.ClientInfo

		srv = app.GetApp(c)
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	if res, err = srv.GetWorkerList(req.ProjectID); err != nil {
		response.APIError(c, err)
		return
	}

	if len(res) == 0 {
		response.APIError(c, errors.ErrNoWorkingNode)
		return
	}

	if task, err = srv.GetTask(req.ProjectID, req.TaskID); err != nil {
		response.APIError(c, err)
		return
	}

	if task.IsRunning == common.TASK_STATUS_RUNNING {
		response.APIError(c, errors.ErrTaskIsRunning)
		return
	}

	// 调用etcd的put方法以触发watcher从而调度该任务
	if err = srv.TemporarySchedulerTask(task); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}
