package user_func

import (
	"ojbk.io/gopherCron/app"
	"ojbk.io/gopherCron/cmd/service/response"
	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/jwt"
	"ojbk.io/gopherCron/utils"

	"github.com/gin-gonic/gin"
)

type LoginRequest struct {
	Account  string `form:"account" binding:"required"`
	Password string `form:"password" binding:"required"`
}

func Login(c *gin.Context) {
	var (
		req      LoginRequest
		err      error
		errObj   errors.Error
		user     *common.User
		password string
		srv      = app.GetApp(c)
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if user, err = srv.GetUserByAccount(req.Account); err != nil {
		response.APIError(c, err)
		return
	}

	if user == nil {
		errObj = errors.ErrUserNotFound
		response.APIError(c, errObj)
		return
	}

	if password = utils.BuildPassword(req.Password, user.Salt); password != user.Password {
		errObj = errors.ErrPasswordErr
		response.APIError(c, errObj)
		return
	}

	response.APISuccess(c, &gin.H{
		"name":       user.Name,
		"permission": user.Permission,
		"id":         user.ID,
		"token":      jwt.Build(user.ID),
	})
}
