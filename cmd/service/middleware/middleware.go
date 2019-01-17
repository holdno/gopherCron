package middleware

import (
	"github.com/gin-gonic/gin"
	"ojbk.io/gopherCron/cmd/service/request"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/jwt"
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
		res := new(request.Response)
		res.Meta = new(request.Meta)
		res.Meta.RequestURI = c.Request.RequestURI
		res.Meta.RequestID = request.GetRequestID(c)
		res.Body = struct{}{}
		c.Set(request.ResponseKey, res)
	}
}

// TokenVerify access token校验
func TokenVerify() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Request.Header.Get("access-token")
		res := jwt.Verify(token)
		if res.Code != 1000 {
			request.APIError(c, errors.ErrUnauthorized)
			c.Abort() // 阻止请求继续执行
		} else {
			c.Set("jwt_user", res.User)
			c.Set("jwt_biz", res.Biz)
			c.Next()
		}
	}
}
