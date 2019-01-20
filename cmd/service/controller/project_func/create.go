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

type CreateRequest struct {
	Title  string `form:"title" binding:"required"`
	Remark string `form:"remark"`
}

func Create(c *gin.Context) {
	var (
		req     CreateRequest
		err     error
		uid     string
		project *common.Project
		userID  primitive.ObjectID
	)

	uid = c.GetString("jwt_user")

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if project, err = db.CheckProjectExistByName(req.Title); err != nil {
		if err != errors.ErrProjectNotExist {
			request.APIError(c, err)
			return
		}
	}

	if project != nil {
		request.APIError(c, errors.ErrProjectExist)
		return
	}

	userID, _ = primitive.ObjectIDFromHex(uid)

	if err = db.CreateProject(&common.Project{
		Title:    req.Title,
		Remark:   req.Remark,
		UID:      userID,
		Relation: []primitive.ObjectID{userID},
	}); err != nil {
		request.APIError(c, err)
		return
	}

	request.APISuccess(c, nil)
}
