package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/utils"
)

type CreateWebHookRequest struct {
	ProjectID   int64  `json:"project_id" form:"project_id" binding:"required"`
	CallBackURL string `json:"call_back_url" form:"call_back_url" binding:"required"`
	Type        string `json:"type" form:"type" binding:"required"`
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

	if err = srv.CheckPermissions(req.ProjectID, uid, app.PermissionEdit); err != nil {
		response.APIError(c, err)
		return
	}

	if err = srv.CreateWebHook(req.ProjectID, req.Type, req.CallBackURL); err != nil {
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

	if err = srv.CheckPermissions(req.ProjectID, uid, app.PermissionEdit); err != nil {
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
	Type      string `json:"type" form:"type"`
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

	if err = srv.CheckPermissions(req.ProjectID, uid, app.PermissionEdit); err != nil {
		response.APIError(c, err)
		return
	}

	webhook, err := srv.GetWebHook(req.ProjectID, req.Type)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, webhook)
}

type DeleteWebHookRequest struct {
	ProjectID int64  `json:"project_id" form:"project_id" binding:"required"`
	Type      string `json:"type" form:"type" binding:"required"`
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

	if err = srv.CheckPermissions(req.ProjectID, uid, app.PermissionEdit); err != nil {
		response.APIError(c, err)
		return
	}

	if err = srv.DeleteWebHook(nil, req.ProjectID, req.Type); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}
