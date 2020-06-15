package project_func

import (
	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/utils"

	"github.com/gin-gonic/gin"
)

type UpdateRequest struct {
	ProjectID int64  `form:"project_id" binding:"required"`
	Title     string `form:"title" binding:"required"`
	Remark    string `form:"remark"`
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
		response.APIError(c, errors.ErrInvalidArgument)
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
	UserID    int64 `form:"user_id" binding:"required"`
	ProjectID int64 `form:"project_id" binding:"required"`
}

func RemoveUser(c *gin.Context) {
	var (
		err     error
		req     RemoveUserRequest
		project *common.Project
		uid     = utils.GetUserID(c)
		srv     = app.GetApp(c)
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

	// 首先确认操作的用户是否为该项目的管理员
	isAdmin, err := srv.IsAdmin(uid)
	if err != nil {
		response.APIError(c, err)
		return
	}

	if !isAdmin {
		if project, err = srv.CheckUserProject(req.ProjectID, uid); err != nil {
			response.APIError(c, err)
			return
		}

		if project == nil {
			response.APIError(c, errors.ErrUnauthorized)
			return
		}
	}

	// 验证通过后再执行移出操作
	if err = srv.DeleteProjectRelevance(nil, req.ProjectID, req.UserID); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}

type AddUserRequest struct {
	ProjectID   int64  `form:"project_id" binding:"required"`
	UserAccount string `form:"user_account" binding:"required"`
}

func AddUser(c *gin.Context) {
	var (
		err      error
		req      AddUserRequest
		uid      = utils.GetUserID(c)
		srv      = app.GetApp(c)
		userInfo *common.User
		project  *common.Project
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
		if project, err = srv.CheckUserProject(req.ProjectID, uid); err != nil {
			response.APIError(c, err)
			return
		}

		if project == nil {
			response.APIError(c, errors.ErrUnauthorized)
			return
		}
	}

	if userInfo, err = srv.GetUserByAccount(req.UserAccount); err != nil {
		response.APIError(c, err)
		return
	}

	if userInfo == nil {
		response.APIError(c, errors.ErrUserNotFound)
		return
	}

	// 检测用户是否存在项目组中
	if _, err = srv.CheckUserIsInProject(req.ProjectID, userInfo.ID); err != nil {
		if err != errors.ErrProjectNotExist {
			response.APIError(c, err)
			return
		}
	}

	if err = srv.CreateProjectRelevance(nil, req.ProjectID, userInfo.ID); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}
