package controller

import (
	"net/http"
	"time"

	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
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
		Title:      req.Title,
		Remark:     req.Remark,
		Cron:       req.Cron,
		Status:     req.Status,
		CreateTime: time.Now().Unix(),
	}); err != nil {
		response.APIError(c, err)
		return
	}
	response.APISuccess(c, nil)
}

type GetWorkflowTaskListRequest struct {
	WorkflowID int64 `json:"workflow_id" form:"workflow_id" binding:"required"`
}

type GetWorkflowTaskListResponseItem struct {
	Task       common.WorkflowSchedulePlan `json:"task"`
	TaskDetail common.WorkflowTask         `json:"task_detail"`
	State      *app.WorkflowTaskStates     `json:"state"`
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

	tasks, err := srv.GetWorkflowScheduleTasks(req.WorkflowID)
	if err != nil {
		response.APIError(c, err)
		return
	}

	taskStates, err := srv.GetWorkflowAllTaskStates(req.WorkflowID)
	if err != nil {
		response.APIError(c, err)
		return
	}

	// 获取task名称
	var taskIDs []string
	for _, v := range tasks {
		taskIDs = append(taskIDs, v.TaskID)
	}
	workflowTaskDetails, err := srv.GetMultiWorkflowTaskList(taskIDs)
	if err != nil {
		response.APIError(c, err)
		return
	}

	workflowTaskMap := make(map[string]common.WorkflowTask)
	for _, v := range workflowTaskDetails {
		workflowTaskMap[v.TaskID] = v
	}

	stateMap := make(map[string]*app.WorkflowTaskStates)
	for _, v := range taskStates {
		stateMap[common.BuildWorkflowTaskIndex(v.ProjectID, v.TaskID)] = v
	}

	var res []GetWorkflowTaskListResponseItem
	for _, v := range tasks {
		res = append(res, GetWorkflowTaskListResponseItem{
			Task:       v,
			TaskDetail: workflowTaskMap[v.TaskID],
			State:      stateMap[common.BuildWorkflowTaskIndex(v.ProjectID, v.TaskID)],
		})
	}

	response.APISuccess(c, res)
}

type CreateWorkflowSchedulePlanRequest struct {
	WorkflowID int64                      `json:"workflow_id" form:"workflow_id" binding:"required"`
	Tasks      []WorkflowScheduleTaskItem `json:"tasks" form:"tasks" binding:"required"`
}

type WorkflowScheduleTaskItem struct {
	Task         app.WorkflowTaskInfo   `json:"task" form:"task" binding:"required"`
	Dependencies []app.WorkflowTaskInfo `json:"dependencies" form:"dependencies"`
}

func CreateWorkflowSchedulePlan(c *gin.Context) {
	var (
		err error
		req CreateWorkflowSchedulePlanRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	srv := app.GetApp(c)
	uid := utils.GetUserID(c)
	var args []app.CreateWorkflowSchedulePlanArgs
	for _, v := range req.Tasks {
		args = append(args, app.CreateWorkflowSchedulePlanArgs{
			WorkflowTaskInfo: v.Task,
			Dependencies:     v.Dependencies,
		})
	}

	if err = srv.CreateWorkflowSchedulePlan(uid, req.WorkflowID, args); err != nil {
		response.APIError(c, err)
		return
	}
	response.APISuccess(c, nil)
}

type GetWorkflowRequest struct {
	ID int64 `json:"id" form:"id" binding:"required"`
}

func GetWorkflow(c *gin.Context) {
	var (
		err error
		req GetWorkflowRequest
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	srv := app.GetApp(c)
	uid := utils.GetUserID(c)

	if err = srv.GetUserWorkflowPermission(uid, req.ID); err != nil {
		response.APIError(c, err)
		return
	}
	data, err := srv.GetWorkflow(req.ID)
	if err != nil {
		response.APIError(c, err)
		return
	}

	if data == nil {
		response.APIError(c, errors.NewError(http.StatusNotFound, "workflow不存在"))
		return
	}

	state, err := srv.GetWorkflowState(data.ID)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, WorkflowWithState{
		Workflow: *data,
		State:    state,
	})
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
	isAdmin, err := srv.IsAdmin(uid)
	if err != nil {
		response.APIError(c, err)
		return
	}

	var workflowIDs []int64
	if !isAdmin {
		workflowIDs, err = srv.GetUserWorkflows(uid)
		if err != nil {
			response.APIError(c, err)
			return
		}
		if len(workflowIDs) == 0 {
			response.APISuccess(c, GetWorkflowListResponse{
				Total: 0,
				List:  nil,
			})
			return
		}
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

type StartWorkflowRequest struct {
	WorkflowID int64 `json:"workflow_id" form:"workflow_id" binding:"required"`
}

func StartWorkflow(c *gin.Context) {
	var (
		err error
		req StartWorkflowRequest
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

	if err = srv.StartWorkflow(req.WorkflowID); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}

type KillWorkflowRequest struct {
	WorkflowID int64 `json:"workflow_id" form:"workflow_id" binding:"required"`
}

func KillWorkflow(c *gin.Context) {
	var (
		err error
		req KillWorkflowRequest
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

	if err = srv.KillWorkflow(req.WorkflowID); err != nil {
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

type WorkflowAddUserRequest struct {
	WorkflowID  int64  `json:"workflow_id" form:"workflow_id" binding:"required"`
	UserAccount string `json:"user_account" form:"user_account" binding:"required"`
}

func WorkflowAddUser(c *gin.Context) {
	var (
		err      error
		req      WorkflowAddUserRequest
		uid      = utils.GetUserID(c)
		srv      = app.GetApp(c)
		userInfo *common.User
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if err = srv.GetUserWorkflowPermission(uid, req.WorkflowID); err != nil {
		response.APIError(c, err)
		return
	}

	if userInfo, err = srv.GetUserByAccount(req.UserAccount); err != nil {
		response.APIError(c, err)
		return
	}

	if userInfo == nil {
		response.APIError(c, errors.ErrUserNotFound)
		return
	}

	if err = srv.WorkflowAddUser(req.WorkflowID, userInfo.ID); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}

type WorkflowRemoveUserRequest struct {
	UserID     int64 `json:"user_id" form:"user_id" binding:"required"`
	WorkflowID int64 `json:"workflow_id" form:"workflow_id" binding:"required"`
}

func WorkflowRemoveUser(c *gin.Context) {
	var (
		err error
		req WorkflowRemoveUserRequest
		uid = utils.GetUserID(c)
		srv = app.GetApp(c)
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, errors.ErrInvalidArgument)
		return
	}

	// 验证该项目是否属于该用户
	if req.UserID == uid {
		// 不能将项目管理员移出项目
		response.APIError(c, errors.ErrDontRemoveProjectAdmin)
		return
	}

	if err = srv.GetUserWorkflowPermission(uid, req.WorkflowID); err != nil {
		response.APIError(c, err)
		return
	}

	// 验证通过后再执行移出操作
	if err = srv.WorkflowRemoveUser(req.WorkflowID, req.UserID); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}

type GetUsersByWorkflowRequest struct {
	WorkflowID int64 `json:"workflow_id" form:"workflow_id" binding:"required"`
}

// GetUsersByProject 获取项目下的用户列表
func GetUsersByWorkflow(c *gin.Context) {
	var (
		err error
		req GetUsersByWorkflowRequest
		wr  []common.UserWorkflowRelevance
		res []*common.User
		uid = utils.GetUserID(c)
		srv = app.GetApp(c)
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	if err = srv.GetUserWorkflowPermission(uid, req.WorkflowID); err != nil {
		response.APIError(c, err)
		return
	}

	if wr, err = srv.GetWorkflowRelevanceUsers(req.WorkflowID); err != nil {
		response.APIError(c, err)
		return
	}

	var userIDs []int64
	for _, v := range wr {
		userIDs = append(userIDs, v.UserID)
	}

	if res, err = srv.GetUsersByIDs(userIDs); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, &gin.H{
		"list": utils.TernaryOperation(res != nil, res, []struct{}{}),
	})
}
