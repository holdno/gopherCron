package user_func

import (
	"github.com/gin-gonic/gin"
	"github.com/mongodb/mongo-go-driver/bson/primitive"
	"ojbk.io/gopherCron/cmd/service/request"
	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/pkg/db"
	"ojbk.io/gopherCron/utils"
)

func GetUserInfo(c *gin.Context) {
	var (
		err    error
		errObj errors.Error
		user   *common.User
		uid    string
		objID  primitive.ObjectID
	)

	uid = c.GetString("jwt_user")

	if objID, err = primitive.ObjectIDFromHex(uid); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if user, err = db.GetUserInfo(objID); err != nil {
		request.APIError(c, err)
		return
	}

	if user == nil {
		errObj = errors.ErrDataNotFound
		request.APIError(c, errObj)
		return
	}

	request.APISuccess(c, &gin.H{
		"name":       user.Name,
		"permission": user.Permission,
		"id":         user.ID,
	})
}

type GetUsersByProjectRequest struct {
	ProjectID string `form:"project_id"`
}

// GetUsersByProject 获取项目下的用户列表
func GetUsersByProject(c *gin.Context) {
	var (
		err       error
		req       GetUsersByProjectRequest
		projectID primitive.ObjectID
		project   *common.Project
		res       []*common.User
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if projectID, err = primitive.ObjectIDFromHex(req.ProjectID); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if project, err = db.GetProject(projectID); err != nil {
		request.APIError(c, err)
		return
	}

	if res, err = db.GetUsers(project.Relation); err != nil {
		request.APIError(c, err)
		return
	}

	request.APISuccess(c, &gin.H{
		"list": utils.TernaryOperation(res != nil, res, []struct{}{}),
	})
}
