package project_func

import (
	"time"

	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/utils"

	"github.com/gin-gonic/gin"
)

type CreateRequest struct {
	Title  string `json:"title" form:"title" binding:"required"`
	Remark string `json:"remark" form:"remark"`
}

func Create(c *gin.Context) {
	var (
		req     CreateRequest
		err     error
		project *common.Project
		id      int64
		uid     = utils.GetUserID(c)
		srv     = app.GetApp(c)
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	if project, err = srv.CheckProjectExistByName(req.Title); err != nil {
		if err != errors.ErrProjectNotExist {
			response.APIError(c, err)
			return
		}
	}

	if project != nil {
		response.APIError(c, errors.ErrProjectExist)
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

	if id, err = srv.CreateProject(tx, common.Project{
		Title:  req.Title,
		Remark: req.Remark,
		UID:    uid,
	}); err != nil {
		response.APIError(c, err)
		return
	}

	if err = srv.CreateProjectRelevance(tx, id, uid); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}

type CreateProjectWorkflowTaskRequest struct {
	ProjectID int64  `json:"project_id" form:"project_id" binding:"required"`
	TaskName  string `json:"task_name" form:"task_name" binding:"required"`
	Command   string `json:"command" form:"command" binding:"required"`
	Remark    string `json:"remark" form:"remark"`
	Timeout   int    `json:"timeout" form:"timeout" binding:"required"`
}

func CreateProjectWorkflowTask(c *gin.Context) {
	var (
		err error
		req CreateProjectWorkflowTaskRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	srv := app.GetApp(c)
	uid := utils.GetUserID(c)

	err = srv.CreateWorkflowTask(uid, common.WorkflowTask{
		ProjectID:  req.ProjectID,
		TaskName:   req.TaskName,
		Command:    req.Command,
		Remark:     req.Remark,
		Timeout:    req.Timeout,
		CreateTime: time.Now().Unix(),
	})
	if err != nil {
		response.APIError(c, err)
		return
	}
	response.APISuccess(c, nil)
}
