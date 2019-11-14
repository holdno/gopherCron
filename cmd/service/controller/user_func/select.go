package user_func

import (
	"ojbk.io/gopherCron/app"
	"ojbk.io/gopherCron/cmd/service/response"
	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/utils"

	"github.com/gin-gonic/gin"
)

func GetUserInfo(c *gin.Context) {
	var (
		err    error
		errObj errors.Error
		user   *common.User
		uid    = utils.GetUserID(c)
		srv    = app.GetApp(c)
	)

	if user, err = srv.GetUserInfo(uid); err != nil {
		response.APIError(c, err)
		return
	}

	if user == nil {
		errObj = errors.ErrUserNotFound
		response.APIError(c, errObj)
		return
	}

	response.APISuccess(c, &gin.H{
		"name":       user.Name,
		"permission": user.Permission,
		"id":         user.ID,
	})
}

type GetUsersByProjectRequest struct {
	ProjectID int64 `form:"project_id" json:"project_id"`
}

// GetUsersByProject 获取项目下的用户列表
func GetUsersUnderTheProject(c *gin.Context) {
	var (
		err error
		req GetUsersByProjectRequest
		pr  []*common.ProjectRelevance
		res []*common.User
		srv = app.GetApp(c)
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if pr, err = srv.GetProjectRelevanceUsers(req.ProjectID); err != nil {
		response.APIError(c, err)
		return
	}

	if len(pr) == 0 {
		response.APIError(c, errors.ErrProjectNotExist)
		return
	}

	var userIDs []int64
	for _, v := range pr {
		userIDs = append(userIDs, v.UID)
	}

	if res, err = srv.GetUsersByIDs(userIDs); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, &gin.H{
		"list": utils.TernaryOperation(res != nil, res, []struct{}{}),
	})
}
