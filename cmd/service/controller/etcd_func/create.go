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
	ProjectID int64  `form:"project_id" json:"project_id" binding:"required"`
	TaskID    string `form:"task_id" json:"task_id"`
	Name      string `form:"name" json:"name" binding:"required"`
	Command   string `form:"command" json:"command" binding:"required"`
	Cron      string `form:"cron" json:"cron" binding:"required"`
	Remark    string `form:"remark" json:"remark"`
	Timeout   int    `form:"timeout" json:"timeout"`
	Status    int    `form:"status" json:"status"` // 执行状态 1立即加入执行队列 0存入etcd但是不执行
	Noseize   int    `form:"noseize" json:"noseize"`
	Exclusion int    `form:"exclusion" json:"exclusion"`
}

// TaskSave save tast to etcd
// post a json value like {"project_id": "xxxx", "task_id": "xxxxx", "name":"task_name", "command": "go run ...", "cron": "*/1 * * * * *", "remark": "write something", "seize": 1}
func SaveTask(c *gin.Context) {
	var (
		req         TaskSaveRequest
		oldTaskInfo *common.TaskInfo
		err         error

		uid = utils.GetUserID(c)
		srv = app.GetApp(c)
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	// 验证 cron表达式
	exp, err := cronexpr.Parse(req.Cron)
	if err != nil {
		response.APIError(c, errors.ErrCron)
		return
	}

	scheduleInterval := exp.NextN(time.Now(), 2)
	if len(scheduleInterval) == 2 && scheduleInterval[1].Sub(scheduleInterval[0]) < time.Second*5-time.Nanosecond {
		response.APIError(c, errors.ErrCronInterval)
		return
	}

	if err = srv.CheckPermissions(req.ProjectID, uid, app.PermissionView); err != nil {
		response.APIError(c, err)
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
		Exclusion:  req.Exclusion,
		CreateTime: time.Now().Unix(),
		IsRunning:  common.TASK_STATUS_UNDEFINED,
	}); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, oldTaskInfo)
}
