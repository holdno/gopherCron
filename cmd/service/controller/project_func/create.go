package project_func

import (
	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/utils"

	"github.com/gin-gonic/gin"
)

type CreateRequest struct {
	Title  string `form:"title" binding:"required"`
	Remark string `form:"remark"`
}

func Create(c *gin.Context) {
	var (
		req     CreateRequest
		err     error
		project *common.Project
		id      int64
		uid     = utils.GetUserID(c)
		srv     = app.GetApp(c)
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if project, err = srv.CheckProjectExistByName(req.Title); err != nil {
		if err != errors.ErrProjectNotExist {
			response.APIError(c, err)
			return
		}
	}

	if project != nil {
		response.APIError(c, errors.ErrProjectExist)
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

	if id, err = srv.CreateProject(tx, common.Project{
		Title:  req.Title,
		Remark: req.Remark,
		UID:    uid,
	}); err != nil {
		response.APIError(c, err)
		return
	}

	if err = srv.CreateProjectRelevance(tx, id, uid); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}
