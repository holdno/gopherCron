package project_func

import (
	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/utils"
	"github.com/spacegrower/watermelon/infra/wlog"
	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
)

type DeleteOneRequest struct {
	ProjectID int64 `json:"project_id" form:"project_id" binding:"required"`
}

func DeleteOne(c *gin.Context) {
	var (
		err error
		req DeleteOneRequest
		uid = utils.GetUserID(c)
		srv = app.GetApp(c)
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	tx := srv.BeginTx()
	defer func() {
		if r := recover(); r != nil || err != nil {
			wlog.Error("failed to delete project", zap.Int64("project_id", req.ProjectID), zap.Any("panic", r))
			tx.Rollback()
		}
	}()

	if err = srv.CheckPermissions(req.ProjectID, uid, app.PermissionDelete); err != nil {
		response.APIError(c, err)
		return
	}

	if err = srv.CleanProject(tx, req.ProjectID); err != nil {
		response.APIError(c, err)
		return
	}

	if err = tx.Commit().Error; err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}

type DeleteProjectWorkflowTaskRequest struct {
	ProjectID int64  `json:"project_id" form:"project_id" binding:"required"`
	TaskID    string `json:"task_id" form:"task_id" binding:"required"`
}

func DeleteProjectWorkflowTask(c *gin.Context) {
	var (
		err error
		req DeleteProjectWorkflowTaskRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	srv := app.GetApp(c)
	uid := utils.GetUserID(c)
	if err = srv.DeleteWorkflowTask(uid, req.ProjectID, req.TaskID); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}
