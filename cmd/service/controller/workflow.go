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

type GetWorkflowTaskListRequest struct {
	WorkflowID int64 `json:"workflow_id" form:"workflow_id" binding:"required"`
}

func GetWorkflowTaskList(c *gin.Context) {
	var (
		err error
		req GetWorkflowTaskListRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	srv := app.GetApp(c)
	uid := utils.GetUserID(c)
	if err = srv.GetUserWorkflowPermission(uid, req.WorkflowID); err != nil {
		response.APIError(c, err)
		return
	}

	tasks, err := srv.GetWorkflowTasks(req.WorkflowID)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, tasks)
}

type CreateWorkflowTaskRequest struct {
	WorkflowID int64              `json:"workflow_id" form:"workflow_id" binding:"required"`
	Tasks      []WorkflowTaskItem `json:"tasks" form:"tasks" binding:"required"`
}

type WorkflowTaskItem struct {
	Task         app.WorkflowTaskInfo   `json:"task" form:"task" binding:"required"`
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
	Total int                 `json:"total"`
	List  []WorkflowWithState `json:"list"`
}

type WorkflowWithState struct {
	common.Workflow
	State *app.PlanState `json:"state"`
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

	var listWithState []WorkflowWithState
	for _, v := range list {
		state, err := srv.GetWorkflowState(v.ID)
		if err != nil {
			response.APIError(c, err)
			return
		}
		listWithState = append(listWithState, WorkflowWithState{
			v,
			state,
		})
	}

	response.APISuccess(c, GetWorkflowListResponse{
		Total: total,
		List:  listWithState,
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

type ClearWorkflowLogRequest struct {
	WorkflowID int64 `json:"workflow_id" form:"workflow_id" binding:"required"`
}

func ClearWorkflowLog(c *gin.Context) {
	var (
		err error
		req ClearWorkflowLogRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	srv := app.GetApp(c)
	uid := utils.GetUserID(c)
	if err = srv.GetUserWorkflowPermission(uid, req.WorkflowID); err != nil {
		response.APIError(c, err)
		return
	}

	if err = srv.ClearWorkflowLog(req.WorkflowID); err != nil {
		response.APIError(c, err)
		return
	}
	response.APISuccess(c, nil)
}

type GetWorkflowLogListRequest struct {
	WorkflowID int64  `json:"workflow_id" form:"workflow_id" binding:"required"`
	Page       uint64 `json:"page" form:"page" binding:"required"`
	Pagesize   uint64 `json:"pagesize" form:"pagesize" binding:"required"`
}

type GetWorkflowLogListResponse struct {
	List  []common.WorkflowLog `json:"list"`
	Total int                  `json:"total"`
}

func GetWorkflowLogList(c *gin.Context) {
	var (
		err error
		req GetWorkflowLogListRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	srv := app.GetApp(c)
	uid := utils.GetUserID(c)

	if err = srv.GetUserWorkflowPermission(uid, req.WorkflowID); err != nil {
		response.APIError(c, err)
		return
	}

	list, total, err := srv.GetWorkflowLogList(req.WorkflowID, req.Page, req.Pagesize)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, GetWorkflowLogListResponse{
		List:  list,
		Total: total,
	})
}
