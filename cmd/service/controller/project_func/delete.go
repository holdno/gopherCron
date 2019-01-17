package project_func

import (
	"github.com/gin-gonic/gin"
	"ojbk.io/gopherCron/cmd/service/request"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/pkg/db"
	"ojbk.io/gopherCron/pkg/etcd"
	"ojbk.io/gopherCron/utils"
)

type DeleteOneRequest struct {
	Project string `form:"project" binding:"required"`
}

func DeleteOne(c *gin.Context) {
	var (
		err error
		req DeleteOneRequest
		uid string
	)

	uid = c.GetString("jwt_user")

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if err = db.DeleteProject(req.Project, uid); err != nil {
		request.APIError(c, err)
		return
	}

	if _, err = etcd.Manager.DeleteTask(req.Project, ""); err != nil {
		request.APIError(c, err)
		return
	}

	request.APISuccess(c, nil)
}
