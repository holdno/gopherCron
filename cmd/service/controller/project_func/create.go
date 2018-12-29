package project_func

import (
	"github.com/gin-gonic/gin"
	"ojbk.io/gopherCron/cmd/service/request"
	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/pkg/db"
	"ojbk.io/gopherCron/utils"
)

type CreateRequest struct {
	Title   string `form:"title" binding:"required"`
	Project string `form:"project" binding:"required"`
	Remark  string `form:"remark"`
}

func Create(c *gin.Context) {
	var (
		req     CreateRequest
		err     error
		uid     string
		project *common.Project
	)

	uid = c.GetString("jwt_user")

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if project, err = db.CheckProjectExistByName(req.Project); err != nil {
		if err != errors.ErrProjectNotExist {
			request.APIError(c, err)
			return
		}
	}

	if project != nil {
		request.APIError(c, errors.ErrProjectExist)
		return
	}

	if err = db.CreateProject(&common.Project{
		Title:   req.Title,
		Project: req.Project,
		Remark:  req.Remark,
		UID:     uid,
	}); err != nil {
		request.APIError(c, err)
		return
	}

	request.APISuccess(c, nil)
}
