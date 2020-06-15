package log_func

import (
	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/utils"

	"github.com/gin-gonic/gin"
)

// ClearLogsRequest 清理日志请求参数
type CleanLogsRequest struct {
	ProjectID int64  `form:"project_id" binding:"required"`
	TaskID    string `form:"task_id"`
}

// CleanLogs 清理任务日志
func CleanLogs(c *gin.Context) {
	uid := utils.GetUserID(c)
	if uid == 0 {
		response.APIError(c, errors.ErrUnauthorized)
		return
	}

	var (
		err   error
		req   CleanLogsRequest
		srv   = app.GetApp(c)
		exist bool
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, errors.ErrInvalidArgument)
		return
	}

	isAdmin, err := srv.IsAdmin(uid)
	if err != nil {
		response.APIError(c, err)
		return
	}

	if !isAdmin {
		if exist, err = srv.CheckUserIsInProject(req.ProjectID, uid); err != nil {
			response.APIError(c, err)
			return
		}

		if !exist {
			response.APIError(c, errors.ErrProjectNotExist)
			return
		}
	}

	if req.TaskID == "" {
		err = srv.CleanProjectLog(nil, req.ProjectID)
	} else {
		err = srv.CleanLog(nil, req.ProjectID, req.TaskID)
	}

	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}
