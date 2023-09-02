package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/utils"
)

func GetUserOrgList(c *gin.Context) {
	var (
		uid = utils.GetUserID(c)
		srv = app.GetApp(c)
	)

	list, err := srv.GetUserOrgs(uid)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, list)
}

type CreateOrgRequest struct {
	Title string `json:"title" form:"title" binding:"required"`
}

func CreateOrg(c *gin.Context) {
	var (
		err error
		req CreateOrgRequest

		uid = utils.GetUserID(c)
		srv = app.GetApp(c)
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	if _, err = srv.CreateOrg(uid, req.Title); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}
