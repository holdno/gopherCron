package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/utils"
)

type CreateWebHookRequest struct {
	ProjectID   int64  `json:"project_id" form:"project_id" binding:"required"`
	Types       string `json:"types" form:"types" binding:"required"`
	CallBackUrl string `json:"call_back_url" form:"call_back_url" binding:"required"`
}

func CreateWebHook(c *gin.Context) {
	var (
		err error
		req CreateWebHookRequest
		uid = utils.GetUserID(c)
		srv = app.GetApp(c)
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	if err = srv.CheckPermissions(req.ProjectID, uid); err != nil {
		response.APIError(c, err)
		return
	}

	if err = srv.CreateWebHook(req.ProjectID, req.Types, req.CallBackUrl); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}

type GetWebHookListRequest struct {
	ProjectID int64 `json:"project_id" form:"project_id" binding:"required"`
}

func GetWebHookList(c *gin.Context) {
	var (
		err error
		req GetWebHookListRequest
		uid = utils.GetUserID(c)
		srv = app.GetApp(c)
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	if err = srv.CheckPermissions(req.ProjectID, uid); err != nil {
		response.APIError(c, err)
		return
	}

	list, err := srv.GetWebHookList(req.ProjectID)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, list)
}

type GetWebHookRequest struct {
	ProjectID int64  `json:"project_id" form:"project_id" binding:"required"`
	Types     string `json:"types" form:"types" binding:"required"`
}

func GetWebHook(c *gin.Context) {
	var (
		err error
		req GetWebHookRequest
		uid = utils.GetUserID(c)
		srv = app.GetApp(c)
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	if err = srv.CheckPermissions(req.ProjectID, uid); err != nil {
		response.APIError(c, err)
		return
	}

	webhook, err := srv.GetWebHook(req.ProjectID, req.Types)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, webhook)
}

type DeleteWebHookRequest struct {
	ProjectID int64  `json:"project_id" form:"project_id" binding:"required"`
	Types     string `json:"types" form:"types" binding:"required"`
}

func DeleteWebHook(c *gin.Context) {
	var (
		err error
		req DeleteWebHookRequest
		uid = utils.GetUserID(c)
		srv = app.GetApp(c)
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	if err = srv.CheckPermissions(req.ProjectID, uid); err != nil {
		response.APIError(c, err)
		return
	}

	if err = srv.DeleteWebHook(nil, req.ProjectID, req.Types); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}
