package project_func

import (
	"ojbk.io/gopherCron/app"
	"ojbk.io/gopherCron/cmd/service/response"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/utils"

	"github.com/gin-gonic/gin"
)

type DeleteOneRequest struct {
	ProjectID int64 `form:"project_id" binding:"required"`
}

func DeleteOne(c *gin.Context) {
	var (
		err error
		req DeleteOneRequest
		uid = utils.GetUserID(c)
		srv = app.GetApp(c)
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if _, err = srv.DeleteTask(req.ProjectID, ""); err != nil {
		response.APIError(c, err)
		return
	}

	if err = srv.DeleteProject(req.ProjectID, uid); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}
