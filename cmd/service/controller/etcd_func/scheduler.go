package etcd_func

import (
	"fmt"
	"time"

	"github.com/mongodb/mongo-go-driver/bson/primitive"

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

	// 实现一个方案  put一个 下一秒立即执行的任务 然后自动过期掉
	if err = etcd.Manager.TemporarySchedulerTask(&common.TaskInfo{
		ProjectID:  req.ProjectID,
		TaskID:     utils.TernaryOperation(req.TaskID == "", primitive.NewObjectID().Hex(), req.TaskID).(string),
		Name:       task.Name,
		Cron:       fmt.Sprintf(common.TEMPORARY_CRON, time.Now().AddDate(0, 0, 1).Format("02")),
		Command:    task.Command,
		Remark:     task.Remark,
		Timeout:    task.Timeout,
		Status:     common.TASK_STATUS_START,
		CreateTime: time.Now().Unix(),
	}); err != nil {
		request.APIError(c, err)
		return
	}

	request.APISuccess(c, nil)
}
