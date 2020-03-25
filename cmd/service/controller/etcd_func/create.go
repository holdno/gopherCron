package etcd_func

import (
	"time"

	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/utils"

	"github.com/gin-gonic/gin"
	"github.com/gorhill/cronexpr"
)

type TaskSaveRequest struct {
	ProjectID int64  `form:"project_id" binding:"required"`
	TaskID    string `form:"task_id"`
	Name      string `form:"name" binding:"required"`
	Command   string `form:"command" binding:"required"`
	Cron      string `form:"cron" binding:"required"`
	Remark    string `form:"remark"`
	Timeout   int    `form:"timeout"`
	Status    int    `form:"status"` // 执行状态 1立即加入执行队列 0存入etcd但是不执行
	Noseize   int    `form:"noseize"`
}

// TaskSave save tast to etcd
// post a json value like {"project_id": "xxxx", "task_id": "xxxxx", "name":"task_name", "command": "go run ...", "cron": "*/1 * * * * *", "remark": "write something", "seize": 1}
func SaveTask(c *gin.Context) {
	var (
		req         TaskSaveRequest
		oldTaskInfo *common.TaskInfo
		err         error
		exist       bool

		uid = utils.GetUserID(c)
		srv = app.GetApp(c)
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, errors.ErrInvalidArgument)
		return
	}

	// 验证 cron表达式
	if _, err = cronexpr.Parse(req.Cron); err != nil {
		response.APIError(c, errors.ErrCron)
		return
	}

	if exist, err = srv.CheckUserIsInProject(req.ProjectID, uid); err != nil {
		response.APIError(c, err)
		return
	}

	if !exist {
		response.APIError(c, errors.ErrProjectNotExist)
		return
	}

	if oldTaskInfo, err = srv.SaveTask(&common.TaskInfo{
		ProjectID:  req.ProjectID,
		TaskID:     utils.TernaryOperation(req.TaskID == "", utils.GetStrID(), req.TaskID).(string),
		Name:       req.Name,
		Cron:       req.Cron,
		Command:    req.Command,
		Remark:     req.Remark,
		Timeout:    req.Timeout,
		Status:     req.Status,
		Noseize:    req.Noseize,
		CreateTime: time.Now().Unix(),
		IsRunning:  common.TASK_STATUS_UNDEFINED,
	}); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, oldTaskInfo)
}
