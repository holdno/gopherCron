package user_func

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/mongodb/mongo-go-driver/bson/primitive"
	"ojbk.io/gopherCron/cmd/service/request"
	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/pkg/db"
	"ojbk.io/gopherCron/utils"
)

// ChangePasswordRequest 修改密码请求参数
type ChangePasswordRequest struct {
	Password    string `form:"password" binding:"required"`
	NewPassword string `form:"new_password" binding:"required"`
}

// ChangePassword 修改密码
func ChangePassword(c *gin.Context) {
	uid := c.GetString("jwt_user")
	if uid == "" {
		request.APIError(c, errors.ErrUnauthorized)
		return
	}

	fmt.Println(uid)

	var (
		req      ChangePasswordRequest
		err      error
		info     *common.User
		password string
		objID    primitive.ObjectID
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if objID, err = primitive.ObjectIDFromHex(uid); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if info, err = db.GetUserInfo(objID); err != nil {
		request.APIError(c, err)
		return
	}

	password = utils.BuildPassword(req.Password, info.Salt)
	if password != info.Password {
		request.APIError(c, errors.ErrPasswordErr)
		return
	}

	info.Salt = utils.RandomStr(6)
	password = utils.BuildPassword(req.NewPassword, info.Salt)

	if err = db.ChangePassword(objID, password, info.Salt); err != nil {
		request.APIError(c, err)
		return
	}

	request.APISuccess(c, nil)
}
