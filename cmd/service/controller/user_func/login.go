package user_func

import (
	"github.com/gin-gonic/gin"
	"ojbk.io/gopherCron/cmd/service/request"
	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/jwt"
	"ojbk.io/gopherCron/pkg/db"
	"ojbk.io/gopherCron/utils"
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
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if user, err = db.GetUserWithAccount(req.Account); err != nil {
		request.APIError(c, err)
		return
	}

	if user == nil {
		errObj = errors.ErrDataNotFound
		request.APIError(c, errObj)
		return
	}

	if password = utils.BuildPassword(req.Password, user.Salt); password != user.Password {
		errObj = errors.ErrPasswordErr
		request.APIError(c, errObj)
		return
	}

	request.APISuccess(c, &gin.H{
		"name":       user.Name,
		"permission": user.Permission,
		"id":         user.ID,
		"token":      jwt.Build(user.ID),
	})
}
