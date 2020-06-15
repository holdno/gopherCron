package user_func

import (
	"time"

	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/utils"

	"github.com/gin-gonic/gin"
)

type DeleteUserRequest struct {
	ID int64 `json:"id" form:"id" binding:"required"`
}

func DeleteUser(c *gin.Context) {
	var (
		err error
		req DeleteUserRequest
		uid = utils.GetUserID(c)
		srv = app.GetApp(c)
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, errors.ErrInvalidArgument)
		return
	}

	isAdmin, err := srv.IsAdmin(uid)
	if err != nil {
		response.APIError(c, err)
		return
	}

	if !isAdmin {
		response.APIError(c, errors.ErrUnauthorized)
		return
	}

	if err = srv.DeleteUser(req.ID); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}

// CreateUserRequest 创建用户请求参数
type CreateUserRequest struct {
	Name     string `form:"name" binding:"required"`
	Password string `form:"password" binding:"required"`
	Account  string `form:"account" binding:"required"`
}

// CreateUser 创建用户
func CreateUser(c *gin.Context) {
	var (
		req     CreateUserRequest
		err     error
		info    *common.User
		isAdmin bool
		salt    string
		uid     = utils.GetUserID(c)
		srv     = app.GetApp(c)
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if isAdmin, err = srv.IsAdmin(uid); err != nil {
		response.APIError(c, err)
		return
	}

	if isAdmin {
		// 检测用户账号是否已经存在
		if info, err = srv.GetUserByAccount(req.Account); err != nil {
			response.APIError(c, err)
			return
		}

		if info != nil {
			response.APIError(c, errors.ErrUserExist)
			return
		}

		salt = utils.RandomStr(6)

		if err = srv.CreateUser(common.User{
			Account:    req.Account,
			Password:   utils.BuildPassword(req.Password, salt),
			Salt:       salt,
			Name:       req.Name,
			CreateTime: time.Now().Unix(),
		}); err != nil {
			response.APIError(c, err)
			return
		}
	} else {
		response.APIError(c, errors.ErrInsufficientPermissions)
		return
	}

	response.APISuccess(c, nil)
}
