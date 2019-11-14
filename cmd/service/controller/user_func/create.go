package user_func

import (
	"strings"
	"time"

	"ojbk.io/gopherCron/app"
	"ojbk.io/gopherCron/cmd/service/response"
	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/utils"

	"github.com/gin-gonic/gin"
)

// CreateUserRequest 创建用户请求参数
type CreateUserRequest struct {
	Name     string `form:"name" binding:"required"`
	Password string `form:"password" binding:"required"`
	Account  string `form:"account" binding:"required"`
}

// CreateUser 创建用户
func CreateUser(c *gin.Context) {
	var (
		req         CreateUserRequest
		err         error
		info        *common.User
		permissions []string
		isAdmin     bool
		salt        string
		uid         = utils.GetUserID(c)
		srv         = app.GetApp(c)
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if info, err = srv.GetUserInfo(uid); err != nil {
		response.APIError(c, err)
		return
	}

	if info == nil {
		response.APIError(c, errors.ErrUserNotFound)
		return
	}

	// 确认该用户是否为管理员
	permissions = strings.Split(info.Permission, ",")
	for _, v := range permissions {
		if v == "admin" {
			isAdmin = true
			break
		}
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
