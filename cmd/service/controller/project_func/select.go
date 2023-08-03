package project_func

import (
	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/utils"

	"github.com/gin-gonic/gin"
)

type GetProjectWorkflowTasksRequest struct {
	ProjectID int64 `json:"project_id" form:"project_id" binding:"required"`
}

func GetProjectWorkflowTasks(c *gin.Context) {
	var (
		err error
		req GetProjectWorkflowTasksRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	srv := app.GetApp(c)
	uid := utils.GetUserID(c)

	if err = srv.CheckPermissions(req.ProjectID, uid, app.PermissionView); err != nil {
		response.APIError(c, err)
		return
	}

	list, err := srv.GetProjectWorkflowTask(req.ProjectID)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, list)
}

type GetProjectTokenRequest struct {
	ProjectID int64 `json:"project_id" form:"project_id" binding:"required"`
}

func GetProjectToken(c *gin.Context) {
	var (
		err error
		req GetProjectTokenRequest
		srv = app.GetApp(c)
		uid = utils.GetUserID(c)
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	if err = srv.CheckPermissions(req.ProjectID, uid, app.PermissionAll); err != nil {
		response.APIError(c, err)
		return
	}

	project, err := srv.GetProject(req.ProjectID)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, project.Token)
}

type GetUserProjectsResponse struct {
	ProjectID int64  `json:"project_id"`
	UID       int64  `json:"uid"`
	Title     string `json:"title"`
	Remark    string `json:"remark"`
	TaskCount int64  `json:"task_count"`
	Role      string `json:"role"`
}

func GetUserProjects(c *gin.Context) {
	var (
		err   error
		list  []*common.ProjectWithUserRole
		uid   = utils.GetUserID(c)
		res   []*GetUserProjectsResponse
		count int64
		srv   = app.GetApp(c)
	)

	if list, err = srv.GetUserProjects(uid); err != nil {
		response.APIError(c, err)
		return
	}

	// 获取所有项目的任务数
	for _, v := range list {
		count, err = srv.GetProjectTaskCount(v.ID)
		if err != nil {
			response.APIError(c, err)
			return
		}
		res = append(res, &GetUserProjectsResponse{
			ProjectID: v.ID,
			UID:       v.UID,
			Title:     v.Title,
			Remark:    v.Remark,
			TaskCount: count,
			Role:      v.Role,
		})
	}

	response.APISuccess(c, &gin.H{
		"list": utils.TernaryOperation(res != nil, res, []struct{}{}),
	})
}
