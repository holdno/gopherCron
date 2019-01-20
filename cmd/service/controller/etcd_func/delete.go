package etcd_func

import (
	"github.com/gin-gonic/gin"
	"github.com/mongodb/mongo-go-driver/bson/primitive"
	"ojbk.io/gopherCron/cmd/service/request"
	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/pkg/db"
	"ojbk.io/gopherCron/pkg/etcd"
	"ojbk.io/gopherCron/utils"
)

type DeleteTaskRequest struct {
	ProjectID string `form:"project_id" binding:"required"`
	TaskID    string `form:"task_id" binding:"required"`
}

// DeleteTask delete task from etcd
// post project & task name
func DeleteTask(c *gin.Context) {
	var (
		err       error
		req       DeleteTaskRequest
		oldTask   *common.TaskInfo
		errObj    errors.Error
		uid       string
		userID    primitive.ObjectID
		projectID primitive.ObjectID
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		errObj = errors.ErrInvalidArgument
		errObj.Log = "[Controller - DeleteTask] DeleteTaskRequest binding error:" + err.Error()
		request.APIError(c, errObj)
		return
	}

	uid = c.GetString("jwt_user")

	if userID, err = primitive.ObjectIDFromHex(uid); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if projectID, err = primitive.ObjectIDFromHex(req.ProjectID); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if _, err = db.CheckProjectExist(projectID, userID); err != nil {
		request.APIError(c, err)
		return
	}

	if oldTask, err = etcd.Manager.DeleteTask(req.ProjectID, req.TaskID); err != nil {
		request.APIError(c, err)
		return
	}

	request.APISuccess(c, oldTask)
}
