package user_func

import (
	"context"
	"net/http"
	"time"

	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/jwt"
	"github.com/holdno/gopherCron/utils"

	"github.com/gin-gonic/gin"
)

func OIDCAuthURL(c *gin.Context) {
	srv := app.GetApp(c)

	oidcSrv := srv.GetOIDCService()
	if oidcSrv == nil {
		response.APIError(c, errors.NewError(http.StatusMethodNotAllowed, "MethodNotAllowed"))
		return
	}
	response.APISuccess(c, oidcSrv.AuthURL())
}

type OIDCLoginRequest struct {
	State string `json:"state"`
	Code  string `json:"code"`
}

func OIDCLogin(c *gin.Context) {
	var (
		err error
		req OIDCLoginRequest
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	srv := app.GetApp(c)
	oidcSrv := srv.GetOIDCService()

	if oidcSrv == nil {
		response.APIError(c, errors.NewError(http.StatusMethodNotAllowed, "MethodNotAllowed"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second*5)
	defer cancel()
	tokens, err := oidcSrv.Login(ctx, req.Code, req.State)
	if err != nil {
		response.APIError(c, err)
		return
	}

	userInfo, err := srv.GetOIDCService().GetUserInfo(ctx, tokens.AccessToken)
	if err != nil {
		response.APIError(c, err)
		return
	}

	user, err := srv.GetUserByAccount(userInfo.Email)
	if err != nil {
		response.APIError(c, err)
		return
	}

	if user == nil {

		var oidcUserClaims = make(map[string]interface{})
		if err = userInfo.Claims(&oidcUserClaims); err != nil {
			response.APIError(c, errors.NewError(http.StatusInternalServerError, "failed to parser oidc user claims: "+err.Error()))
			return
		}

		userName, exist := oidcUserClaims[oidcSrv.UserNameKey()]
		if !exist {
			response.APIError(c, errors.NewError(http.StatusInternalServerError, "user name not found in claims: "+oidcSrv.UserNameKey()))
			return
		}

		un, ok := userName.(string)
		if !ok {
			response.APIError(c, errors.NewError(http.StatusInternalServerError, "user name column not string, claim key: "+oidcSrv.UserNameKey()))
			return
		}

		//  create user
		salt := utils.RandomStr(6)
		if err = srv.CreateUser(common.User{
			Account:    userInfo.Email,
			Password:   utils.BuildPassword(utils.RandomStr(32), salt), // random
			Salt:       salt,
			Name:       un,
			Permission: "user",
			CreateTime: time.Now().Unix(),
		}); err != nil {
			response.APIError(c, err)
			return
		}

		user, err = srv.GetUserByAccount(userInfo.Email)
		if err != nil {
			response.APIError(c, err)
			return
		}
	}

	response.APISuccess(c, &LoginResponse{
		ID:         user.ID,
		Name:       user.Name,
		Account:    user.Account,
		Permission: user.Permission,
		Token:      jwt.Build(user.ID),
		TTL:        time.Now().Add(time.Duration(srv.GetConfig().JWT.Exp) * time.Second).Unix(),
	})
}
