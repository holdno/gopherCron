package user_func

import (
	"github.com/gin-gonic/gin"
	"ojbk.io/gopherCron/cmd/service/request"
	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/pkg/db"
)

func GetUserInfo(c *gin.Context) {
	var (
		err    error
		errObj errors.Error
		user   *common.User
		uid    string
	)

	uid = c.GetString("jwt_user")

	if user, err = db.GetUserInfo(uid); err != nil {
		request.APIError(c, err)
		return
	}

	if user == nil {
		errObj = errors.ErrDataNotFound
		request.APIError(c, errObj)
		return
	}

	request.APISuccess(c, &gin.H{
		"name":       user.Name,
		"permission": user.Permission,
		"id":         user.ID,
	})
}
