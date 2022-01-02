package controller

import (
	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/utils"

	"github.com/gin-gonic/gin"
)

type CreateWorkflowRequest struct {
	Title  string `json:"title" form:"title" binding:"required"`
	Remark string `json:"remark" form:"remark"`
	Cron   string `json:"cron" form:"cron" binding:"required"`
	Status int    `json:"status" form:"status"`
}

func CreateWorkflow(c *gin.Context) {
	var (
		err error
		req CreateWorkflowRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	srv := app.GetApp(c)
	uid := utils.GetUserID(c)
	if err = srv.CreateWorkflow(uid, common.Workflow{
		Title:  req.Title,
		Remark: req.Remark,
		Cron:   req.Cron,
		Status: req.Status,
	}); err != nil {
		response.APIError(c, err)
		return
	}
	response.APISuccess(c, nil)
}

type CreateWorkflowTaskRequest struct {
	WorkflowID int64              `json:"workflow_id" form:"workflow_id" binding:"required"`
	Tasks      []WorkflowTaskItem `json:"tasks" form:"tasks" binding:"required"`
}

type WorkflowTaskItem struct {
	Task         app.WorkflowTaskInfo   `json:"task_id" form:"task_id" binding:"required"`
	Dependencies []app.WorkflowTaskInfo `json:"dependencies" form:"dependencies"`
}

func CreateWorkflowTask(c *gin.Context) {
	var (
		err error
		req CreateWorkflowTaskRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	srv := app.GetApp(c)
	uid := utils.GetUserID(c)
	var args []app.CreateWorkflowTaskArgs
	for _, v := range req.Tasks {
		args = append(args, app.CreateWorkflowTaskArgs{
			WorkflowTaskInfo: v.Task,
			Dependencies:     v.Dependencies,
		})
	}
	if err = srv.CreateWorkflowTask(uid, req.WorkflowID, args); err != nil {
		response.APIError(c, err)
		return
	}
	response.APISuccess(c, nil)
}

type GetWorkflowListRequest struct {
	Title    string `json:"title" form:"title"`
	Page     uint64 `json:"page" form:"page" binding:"required"`
	Pagesize uint64 `json:"pagesize" form:"pagesize" binding:"required"`
}

type GetWorkflowListResponse struct {
	Total int               `json:"total"`
	List  []common.Workflow `json:"list"`
}

func GetWorkflowList(c *gin.Context) {
	var (
		err error
		req GetWorkflowListRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	srv := app.GetApp(c)
	uid := utils.GetUserID(c)
	workflowIDs, err := srv.GetUserWorkflows(uid)
	if err != nil {
		response.APIError(c, err)
		return
	}
	list, total, err := srv.GetWorkflowList(common.GetWorkflowListOptions{
		Title: req.Title,
		IDs:   workflowIDs,
	}, req.Page, req.Pagesize)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, GetWorkflowListResponse{
		Total: total,
		List:  list,
	})
}

type DeleteWorkflowRequest struct {
	ID int64 `json:"id" form:"id" binding:"required"`
}

func DeleteWorkflow(c *gin.Context) {
	var (
		err error
		req DeleteWorkflowRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	srv := app.GetApp(c)
	uid := utils.GetUserID(c)

	if err = srv.DeleteWorkflow(uid, req.ID); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}

type UpdateWorkflowRequest struct {
	ID     int64  `json:"id" form:"id" binding:"required"`
	Title  string `json:"title" form:"title" binding:"required"`
	Remark string `json:"remark" form:"remark"`
	Cron   string `json:"cron" form:"cron" binding:"required"`
	Status int    `json:"status" form:"status"`
}

func UpdateWorkflow(c *gin.Context) {
	var (
		err error
		req UpdateWorkflowRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	srv := app.GetApp(c)
	uid := utils.GetUserID(c)
	if err = srv.UpdateWorkflow(uid, common.Workflow{
		ID:     req.ID,
		Title:  req.Title,
		Remark: req.Remark,
		Cron:   req.Cron,
		Status: req.Status,
	}); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}
