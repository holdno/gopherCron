package user_func

import (
	"strings"
	"time"

	"ojbk.io/gopherCron/utils"

	"github.com/gin-gonic/gin"
	"ojbk.io/gopherCron/cmd/service/request"
	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/pkg/db"
)

// CreateUserRequest 创建用户请求参数
type CreateUserRequest struct {
	Name     string `form:"name" binding:"required"`
	Password string `form:"password" binding:"required"`
	Account  string `form:"account" binding:"required"`
}

// CreateUser 创建用户
func CreateUser(c *gin.Context) {
	uid := c.GetString("jwt_user")
	if uid == "" {
		request.APIError(c, errors.ErrUnauthorized)
		return
	}

	var (
		req         CreateUserRequest
		err         error
		info        *common.User
		permissions []string
		isAdmin     bool
		salt        string
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if info, err = db.GetUserInfo(uid); err != nil {
		request.APIError(c, err)
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

		salt = utils.RandomStr(6)

		err = db.CreateUser(&common.User{
			Account:    req.Account,
			Password:   utils.BuildPassword(req.Password, salt),
			Salt:       salt,
			Name:       req.Name,
			CreateTime: time.Now().Unix(),
		})

		if err != nil {
			request.APIError(c, err)
			return
		}
	} else {
		request.APIError(c, errors.ErrInsufficientPermissions)
		return
	}

	request.APISuccess(c, nil)
}
