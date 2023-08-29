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

type UpdateRequest struct {
	ProjectID int64  `json:"project_id" form:"project_id" binding:"required"`
	Title     string `json:"title" form:"title" binding:"required"`
	Remark    string `json:"remark" form:"remark"`
}

// Update 更新项目信息
// 只有项目创建者才可以更新
func Update(c *gin.Context) {
	var (
		req     UpdateRequest
		err     error
		project *common.Project
		uid     = utils.GetUserID(c)
		srv     = app.GetApp(c)
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	// 检测该项目是否属于请求人
	if project, err = srv.CheckUserProject(req.ProjectID, uid); err != nil {
		response.APIError(c, err)
		return
	}

	if project == nil {
		response.APIError(c, errors.ErrProjectNotExist)
		return
	}

	if err = srv.UpdateProject(req.ProjectID, req.Title, req.Remark); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}

type RemoveUserRequest struct {
	UserID    int64 `json:"user_id" form:"user_id" binding:"required"`
	ProjectID int64 `json:"project_id" form:"project_id" binding:"required"`
}

func RemoveUser(c *gin.Context) {
	var (
		err error
		req RemoveUserRequest
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

	if err = srv.CheckPermissions(req.ProjectID, uid, app.PermissionEdit); err != nil {
		response.APIError(c, err)
		return
	}

	// 验证通过后再执行移出操作
	if err = srv.DeleteProjectRelevance(nil, req.ProjectID, req.UserID); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}

type AddUserRequest struct {
	ProjectID   int64  `json:"project_id" form:"project_id" binding:"required"`
	UserAccount string `json:"user_account" form:"user_account" binding:"required"`
	UserRole    string `json:"user_role" form:"user_role" binding:"required"`
}

func AddUser(c *gin.Context) {
	var (
		err      error
		req      AddUserRequest
		uid      = utils.GetUserID(c)
		srv      = app.GetApp(c)
		userInfo *common.User
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if err = srv.CheckPermissions(req.ProjectID, uid, app.PermissionEdit); err != nil {
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

	if err = srv.CreateProjectRelevance(nil, req.ProjectID, userInfo.ID, req.UserRole); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}

type UpdateProjectWorkflowTaskRequest struct {
	TaskID    string `json:"task_id" form:"task_id" binding:"required"`
	ProjectID int64  `json:"project_id" form:"project_id" binding:"required"`
	TaskName  string `json:"task_name" form:"task_name" binding:"required"`
	Command   string `json:"command" form:"command" binding:"required"`
	Remark    string `json:"remark" form:"remark"`
	Timeout   int    `json:"timeout" form:"timeout" binding:"required"`
}

func UpdateProjectWorkflowTask(c *gin.Context) {
	var (
		err error
		req UpdateProjectWorkflowTaskRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	uid := utils.GetUserID(c)
	srv := app.GetApp(c)

	err = srv.UpdateWorkflowTask(uid, common.WorkflowTask{
		TaskID:     req.TaskID,
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
