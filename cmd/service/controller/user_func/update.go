package user_func

import (
	"ojbk.io/gopherCron/app"
	"ojbk.io/gopherCron/cmd/service/response"
	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/utils"

	"github.com/gin-gonic/gin"
)

// ChangePasswordRequest 修改密码请求参数
type ChangePasswordRequest struct {
	Password    string `form:"password" binding:"required"`
	NewPassword string `form:"new_password" binding:"required"`
}

// ChangePassword 修改密码
func ChangePassword(c *gin.Context) {
	var (
		req      ChangePasswordRequest
		err      error
		info     *common.User
		password string
		uid      = utils.GetUserID(c)
		srv      = app.GetApp(c)
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if info, err = srv.GetUserInfo(uid); err != nil {
		response.APIError(c, err)
		return
	}

	password = utils.BuildPassword(req.Password, info.Salt)
	if password != info.Password {
		response.APIError(c, errors.ErrPasswordErr)
		return
	}

	info.Salt = utils.RandomStr(6)
	password = utils.BuildPassword(req.NewPassword, info.Salt)

	if err = srv.ChangePassword(uid, password, info.Salt); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}
