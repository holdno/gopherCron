package etcd_func

import (
	"strings"
	"time"

	"github.com/gorhill/cronexpr"

	"github.com/mongodb/mongo-go-driver/bson/primitive"

	"ojbk.io/gopherCron/pkg/db"

	"ojbk.io/gopherCron/pkg/etcd"

	"github.com/gin-gonic/gin"
	"ojbk.io/gopherCron/cmd/service/request"
	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/utils"
)

type TaskSaveRequest struct {
	ProjectID string `form:"project_id" binding:"required"`
	TaskID    string `form:"task_id"`
	Name      string `form:"name" binding:"required"`
	Command   string `form:"command" binding:"required"`
	Cron      string `form:"cron" binding:"required"`
	Remark    string `form:"remark"`
	Timeout   int    `form:"timeout"`
	Status    int    `form:"status"` // 执行状态 1立即加入执行队列 0存入etcd但是不执行
}

// TaskSave save tast to etcd
// post a json value like {"project_id": "xxxx", "task_id": "xxxxx", "name":"task_name", "command": "go run ...", "cron": "*/1 * * * * *", "remark": "write something"}
func SaveTask(c *gin.Context) {
	var (
		req         TaskSaveRequest
		oldTaskInfo *common.TaskInfo
		err         error
		uid         string
		project     *common.Project

		userID    primitive.ObjectID
		projectID primitive.ObjectID
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	// 验证 cron表达式
	if _, err = cronexpr.Parse(req.Cron); err != nil {
		request.APIError(c, errors.ErrCron)
		return
	}

	// 项目名称和任务名称会作为etcd的key进行保存 所以要去除包含空格的情况
	req.ProjectID = strings.Replace(req.ProjectID, " ", "", -1)
	req.TaskID = strings.Replace(req.TaskID, " ", "", -1)

	uid = c.GetString("jwt_user")

	if userID, err = primitive.ObjectIDFromHex(uid); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if projectID, err = primitive.ObjectIDFromHex(req.ProjectID); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if project, err = db.CheckProjectExist(projectID, userID); err != nil {
		request.APIError(c, err)
		return
	}

	if project == nil {
		request.APIError(c, errors.ErrProjectNotExist)
		return
	}

	if oldTaskInfo, err = etcd.Manager.SaveTask(&common.TaskInfo{
		ProjectID:  req.ProjectID,
		TaskID:     utils.TernaryOperation(req.TaskID == "", primitive.NewObjectID().Hex(), req.TaskID).(string),
		Name:       req.Name,
		Cron:       req.Cron,
		Command:    req.Command,
		Remark:     req.Remark,
		Timeout:    req.Timeout,
		Status:     req.Status,
		CreateTime: time.Now().Unix(),
	}); err != nil {
		request.APIError(c, err)
		return
	}

	request.APISuccess(c, oldTaskInfo)
}
