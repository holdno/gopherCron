package log_func

import (
	"github.com/gin-gonic/gin"
	"ojbk.io/gopherCron/cmd/service/request"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/pkg/db"
	"ojbk.io/gopherCron/utils"
)

// ClearLogsRequest 清理日志请求参数
type CleanLogsRequest struct {
	Project  string `form:"project" binding:"required"`
	TaskName string `form:"task_name"`
}

// CleanLogs 清理任务日志
func CleanLogs(c *gin.Context) {
	uid := c.GetString("jwt_user")
	if uid == "" {
		request.APIError(c, errors.ErrUnauthorized)
		return
	}

	var (
		err error
		req CleanLogsRequest
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if _, err = db.CheckProjectExist(req.Project, uid); err != nil {
		request.APIError(c, err)
		return
	}

	if req.TaskName == "" {
		err = db.CleanProjectLog(req.Project)
	} else {
		err = db.CleanLog(req.Project, req.TaskName)
	}

	if err != nil {
		request.APIError(c, err)
		return
	}

	request.APISuccess(c, nil)
}
