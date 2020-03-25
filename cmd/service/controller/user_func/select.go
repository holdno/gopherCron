package user_func

import (
	"github.com/gin-gonic/gin/binding"
	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/utils"

	"github.com/gin-gonic/gin"
)

type GetUserListRequest struct {
	ID        int64  `json:"id" form:"id"`
	Name      string `json:"name" form:"name"`
	Account   string `json:"account" form:"account"`
	ProjectID int64  `json:"project_id" form:"project_id"`
	Page      int    `json:"page" form:"page" binding:"required"`
	Pagesize  int    `json:"pagesize" form:"pagesize" binding:"required"`
}

func GetUserList(c *gin.Context) {
	var (
		err error
		req GetUserListRequest
	)

	if err = c.ShouldBindWith(&req, binding.Default(c.Request.Method, c.ContentType())); err != nil {
		errObj := errors.ErrInvalidArgument
		errObj.Msg = "请求参数错误"
		errObj.Log = err.Error()
		response.APIError(c, errObj)
		return
	}

	srv := app.GetApp(c)

	args := app.GetUserListArgs{
		ID:       req.ID,
		Account:  req.Account,
		Name:     req.Name,
		Page:     req.Page,
		Pagesize: req.Pagesize,
	}
	list, err := srv.GetUserList(args)
	if err != nil {
		response.APIError(c, err)
		return
	}

	total, err := srv.GetUserListTotal(args)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, &gin.H{
		"total": total,
		"list":  list,
	})
}

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
