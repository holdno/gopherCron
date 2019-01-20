package log_func

import (
	"github.com/gin-gonic/gin"
	"github.com/mongodb/mongo-go-driver/bson/primitive"
	"ojbk.io/gopherCron/cmd/service/request"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/pkg/db"
	"ojbk.io/gopherCron/utils"
)

// ClearLogsRequest 清理日志请求参数
type CleanLogsRequest struct {
	ProjectID string `form:"project_id" binding:"required"`
	TaskID    string `form:"task_id"`
}

// CleanLogs 清理任务日志
func CleanLogs(c *gin.Context) {
	uid := c.GetString("jwt_user")
	if uid == "" {
		request.APIError(c, errors.ErrUnauthorized)
		return
	}

	var (
		err       error
		req       CleanLogsRequest
		userID    primitive.ObjectID
		projectID primitive.ObjectID
		taskID    primitive.ObjectID
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if userID, err = primitive.ObjectIDFromHex(uid); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if projectID, err = primitive.ObjectIDFromHex(req.ProjectID); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if taskID, err = primitive.ObjectIDFromHex(req.TaskID); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if _, err = db.CheckProjectExist(projectID, userID); err != nil {
		request.APIError(c, err)
		return
	}

	if req.TaskID == "" {
		err = db.CleanProjectLog(projectID)
	} else {
		err = db.CleanLog(projectID, taskID)
	}

	if err != nil {
		request.APIError(c, err)
		return
	}

	request.APISuccess(c, nil)
}
