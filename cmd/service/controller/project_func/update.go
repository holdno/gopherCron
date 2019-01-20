package project_func

import (
	"github.com/gin-gonic/gin"
	"github.com/mongodb/mongo-go-driver/bson/primitive"
	"ojbk.io/gopherCron/cmd/service/request"
	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/pkg/db"
	"ojbk.io/gopherCron/utils"
)

type UpdateRequest struct {
	Title     string `form:"title" binding:"required"`
	Remark    string `form:"remark"`
	ProjectID string `form:"id"`
}

// Update 更新项目信息
// 只有项目创建者才可以更新
func Update(c *gin.Context) {
	var (
		req       UpdateRequest
		err       error
		uid       string
		project   *common.Project
		userID    primitive.ObjectID
		projectID primitive.ObjectID
	)

	uid = c.GetString("jwt_user")

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if userID, err = primitive.ObjectIDFromHex(uid); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if projectID, err = primitive.ObjectIDFromHex(req.ProjectID); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if project, err = db.CheckProjectExist(projectID, userID); err != nil {
		if err != errors.ErrProjectNotExist {
			request.APIError(c, err)
			return
		}
	}

	if project != nil {
		request.APIError(c, errors.ErrProjectExist)
		return
	}

	if err = db.UpdateProject(&common.Project{
		Title:     req.Title,
		Remark:    req.Remark,
		ProjectID: projectID,
	}); err != nil {
		request.APIError(c, err)
		return
	}

	request.APISuccess(c, nil)
}

type RemoveUserRequest struct {
	UserID    string `form:"user_id" binding:"required"`
	ProjectID string `form:"project_id" binding:"required"`
}

func RemoveUser(c *gin.Context) {
	var (
		err          error
		req          RemoveUserRequest
		uid          string
		userID       primitive.ObjectID
		removeUserId primitive.ObjectID
		projectID    primitive.ObjectID
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	// 验证该项目是否属于该用户
	uid = c.GetString("jwt_user")

	if req.UserID == uid {
		// 不能将项目管理员移出项目
		request.APIError(c, errors.ErrDontRemoveProjectAdmin)
		return
	}

	if userID, err = primitive.ObjectIDFromHex(uid); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if projectID, err = primitive.ObjectIDFromHex(req.ProjectID); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	// 首先确认操作的用户是否为该项目的管理员
	if _, err = db.CheckUserProject(projectID, userID); err != nil {
		request.APIError(c, err)
		return
	}

	if removeUserId, err = primitive.ObjectIDFromHex(req.UserID); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	// 验证通过后再执行移出操作
	if err = db.RemoveUserFromProject(projectID, removeUserId); err != nil {
		request.APIError(c, err)
		return
	}

	request.APISuccess(c, nil)
}

type AddUserRequest struct {
	ProjectID   string `form:"project_id" binding:"required"`
	UserAccount string `form:"user_account" binding:"required"`
}

func AddUser(c *gin.Context) {
	var (
		err       error
		req       AddUserRequest
		uid       string
		userID    primitive.ObjectID
		projectID primitive.ObjectID
		userInfo  *common.User
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	uid = c.GetString("jwt_user")

	if userID, err = primitive.ObjectIDFromHex(uid); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if projectID, err = primitive.ObjectIDFromHex(req.ProjectID); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if _, err = db.CheckUserProject(projectID, userID); err != nil {
		request.APIError(c, err)
		return
	}

	if userInfo, err = db.GetUserWithAccount(req.UserAccount); err != nil {
		request.APIError(c, err)
		return
	}

	if userInfo == nil {
		request.APIError(c, errors.ErrUserNotFound)
		return
	}

	// 检测用户是否存在项目组中
	if _, err = db.CheckUserProject(projectID, userInfo.ID); err != nil {
		if err != errors.ErrProjectNotExist {
			request.APIError(c, err)
			return
		}
	}

	if err = db.AddUserToProject(projectID, userInfo.ID); err != nil {
		request.APIError(c, err)
		return
	}

	request.APISuccess(c, nil)
}
