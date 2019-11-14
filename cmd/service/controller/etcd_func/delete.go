package etcd_func

import (
	"ojbk.io/gopherCron/app"
	"ojbk.io/gopherCron/cmd/service/response"
	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/utils"

	"github.com/gin-gonic/gin"
)

type DeleteTaskRequest struct {
	ProjectID int64  `form:"project_id" binding:"required"`
	TaskID    string `form:"task_id" binding:"required"`
}

// DeleteTask delete task from etcd
// post project & task name
func DeleteTask(c *gin.Context) {
	var (
		err     error
		req     DeleteTaskRequest
		oldTask *common.TaskInfo
		errObj  errors.Error
		exist   bool
		uid     = utils.GetUserID(c)
		srv     = app.GetApp(c)
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		errObj = errors.ErrInvalidArgument
		errObj.Log = err.Error()
		response.APIError(c, errObj)
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

	tx := srv.BeginTx()
	defer func() {
		if r := recover(); r != nil || err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	if err = srv.CleanLog(tx, req.ProjectID, req.TaskID); err != nil {
		response.APIError(c, err)
		return
	}

	if oldTask, err = srv.DeleteTask(req.ProjectID, req.TaskID); err != nil {
		response.APIError(c, err)
	}

	response.APISuccess(c, oldTask)
}
