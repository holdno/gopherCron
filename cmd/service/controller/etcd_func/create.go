package etcd_func

import (
	"strings"
	"time"

	"ojbk.io/gopherCron/pkg/db"

	"ojbk.io/gopherCron/pkg/etcd"

	"github.com/gin-gonic/gin"
	"ojbk.io/gopherCron/cmd/service/request"
	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/utils"
)

type TaskSaveRequest struct {
	Project string `json:"project" form:"project" binding:"required"`
	Name    string `json:"name" form:"name" binding:"required"`
	Command string `json:"command" form:"command" binding:"required"`
	Cron    string `json:"cron" form:"cron" binding:"required"`
	Remark  string `json:"remark" form:"remark"`
	Status  int    `json:"status" form:"status"` // 执行状态 1立即加入执行队列 0存入etcd但是不执行
}

// TaskSave save tast to etcd
// post a json value like {"project": "xxxx", "name":"task_name", "command": "go run ...", "cron": "*/1 * * * * *", "remark": "write something"}
func SaveTask(c *gin.Context) {
	var (
		req         TaskSaveRequest
		oldTaskInfo *common.TaskInfo
		err         error
		project     *common.Project
		uid         string
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	// 项目名称和任务名称会作为etcd的key进行保存 所以要去除包含空格的情况
	req.Project = strings.Replace(req.Project, " ", "", -1)
	req.Name = strings.Replace(req.Name, " ", "", -1)

	uid = c.GetString("jwt_user")

	if project, err = db.CheckProjectExist(req.Project, uid); err != nil {
		request.APIError(c, err)
		return
	}

	if project == nil {
		request.APIError(c, errors.ErrProjectNotExist)
		return
	}

	if oldTaskInfo, err = etcd.Manager.SaveTask(&common.TaskInfo{
		Project:    req.Project,
		Name:       req.Name,
		Cron:       req.Cron,
		Command:    req.Command,
		Remark:     req.Remark,
		Status:     req.Status,
		CreateTime: time.Now().Unix(),
	}); err != nil {
		request.APIError(c, err)
		return
	}

	request.APISuccess(c, oldTaskInfo)
}
