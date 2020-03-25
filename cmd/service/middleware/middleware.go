package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/jwt"
)

// CrossDomain 全局添加跨域允许
func CrossDomain() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "access-token, x-requested-with, content-type")
	}
}

// BuildResponse 构建Response
func BuildResponse() gin.HandlerFunc {
	return func(c *gin.Context) {
		res := new(response.Response)
		res.Meta = new(response.Meta)
		res.Meta.RequestURI = c.Request.RequestURI
		res.Meta.RequestID = response.GetRequestID(c)
		res.Body = struct{}{}
		c.Set(response.ResponseKey, res)
	}
}

// TokenVerify access token校验
func TokenVerify() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Request.Header.Get("access-token")
		res := jwt.Verify(token)
		if res.Code != 1000 {
			response.APIError(c, errors.ErrUnauthorized)
			c.Abort() // 阻止请求继续执行
		} else {
			c.Set(common.USER_ID, res.User)
			c.Set("jwt_biz", res.Biz)
			c.Next()
		}
	}
}
