package project_func

import (
	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/utils"

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

	tx := srv.BeginTx()
	defer func() {
		if r := recover(); r != nil || err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	if err = srv.DeleteProject(tx, req.ProjectID, uid); err != nil {
		response.APIError(c, err)
		return
	}

	if err = srv.CleanProjectLog(tx, req.ProjectID); err != nil {
		response.APIError(c, err)
		return
	}

	if _, err = srv.DeleteTask(req.ProjectID, ""); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}
