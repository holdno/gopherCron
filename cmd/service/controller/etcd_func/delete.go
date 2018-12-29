package etcd_func

import (
	"github.com/gin-gonic/gin"
	"ojbk.io/gopherCron/cmd/service/request"
	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/pkg/db"
	"ojbk.io/gopherCron/pkg/etcd"
	"ojbk.io/gopherCron/utils"
)

type DeleteTaskRequest struct {
	Project string `form:"project" binding:"required"`
	Name    string `form:"name" binding:"required"`
}

// DeleteTask delete task from etcd
// post project & task name
func DeleteTask(c *gin.Context) {
	var (
		err     error
		req     DeleteTaskRequest
		oldTask *common.TaskInfo
		errObj  errors.Error
		uid     string
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		errObj = errors.ErrInvalidArgument
		errObj.Log = "[Controller - DeleteTask] DeleteTaskRequest args error:" + err.Error()
		request.APIError(c, errObj)
		return
	}

	uid = c.GetString("jwt_user")

	if _, err = db.CheckProjectExist(req.Project, uid); err != nil {
		request.APIError(c, err)
		return
	}

	if oldTask, err = etcd.Manager.DeleteTask(req.Project, req.Name); err != nil {
		request.APIError(c, err)
		return
	}

	request.APISuccess(c, oldTask)
}
