package user_func

import (
	"time"

	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/jwt"
	"github.com/holdno/gopherCron/utils"

	"github.com/gin-gonic/gin"
)

type LoginRequest struct {
	Account  string `form:"account" binding:"required"`
	Password string `form:"password" binding:"required"`
}

type LoginResponse struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Account    string `json:"account"`
	Permission string `json:"permission"`
	Token      string `json:"token"`
	TTL        int64  `json:"ttl"`
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
		response.APIError(c, err)
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

	token, err := jwt.BuildUserJWT(user.ID, srv.GetConfig().JWT.Exp, []byte(srv.GetConfig().JWT.PrivateKey))
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, &LoginResponse{
		ID:         user.ID,
		Name:       user.Name,
		Account:    user.Account,
		Permission: user.Permission,
		Token:      token,
		TTL:        time.Now().Add(time.Duration(srv.GetConfig().JWT.Exp) * time.Second).Unix(),
	})
}
