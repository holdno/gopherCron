package user_func

import (
	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/utils"

	"github.com/gin-gonic/gin"
)

// ChangePasswordRequest 修改密码请求参数
type ChangePasswordRequest struct {
	UserID      int64  `json:"user_id" form:"user_id" binding:"required"`
	Password    string `json:"password" form:"password"`
	NewPassword string `json:"new_password" form:"new_password" binding:"required"`
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
		response.APIError(c, err)
		return
	}

	if info, err = srv.GetUserInfo(req.UserID); err != nil {
		response.APIError(c, err)
		return
	}

	isAdmin, err := srv.IsAdmin(uid)
	if err != nil {
		response.APIError(c, err)
		return
	}
	if !isAdmin {
		if uid != req.UserID {
			response.APIError(c, errors.ErrInvalidArgument)
			return
		}
		password = utils.BuildPassword(req.Password, info.Salt)
		if password != info.Password {
			response.APIError(c, errors.ErrPasswordErr)
			return
		}
	}

	info.Salt = utils.RandomStr(6)
	password = utils.BuildPassword(req.NewPassword, info.Salt)

	if err = srv.ChangePassword(req.UserID, password, info.Salt); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}
