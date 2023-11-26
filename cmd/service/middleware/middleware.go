package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/spacegrower/watermelon/infra/middleware"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/jwt"
)

var (
	GetFrom           = middleware.GetFrom
	SetInto           = middleware.SetInto
	GetFullMethodFrom = middleware.GetFullMethodFrom
	GetRequestFrom    = middleware.GetRequestFrom
	Next              = middleware.Next
)

// CrossDomain 全局添加跨域允许
func CrossDomain() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", c.Request.Header.Get("Origin"))
		c.Writer.Header().Set("Access-Control-Allow-Headers", "access-token, content-type, Cookie")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Next()
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
func TokenVerify(pubKey []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Request.Header.Get("access-token")
		res := jwt.Parse(token, pubKey)
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

func GetAgentVersionFromContext(ctx context.Context) (string, bool) {
	md, exist := metadata.FromIncomingContext(ctx)
	if !exist {
		return "", exist
	}

	version := md.Get(common.GOPHERCRON_AGENT_VERSION_KEY)
	if len(version) == 0 {
		return "", false
	}
	return version[0], true
}

type agentIPKey struct{}

func CheckoutAgentMeta(legacyMode bool) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		agentIP, exist := getAgentIPFromContext(ctx)
		if !exist {
			if !legacyMode {
				return status.Error(codes.Aborted, "header: "+common.GOPHERCRON_AGENT_IP_MD_KEY+" is not found")
			}
			agentIP = "old-version-agent"
		}
		SetAgentIP(ctx, agentIP)

		agentVersion, exist := GetAgentVersionFromContext(ctx)
		if !exist {
			SetAgentVersion(ctx, "v2.1.1") // 如果没有version说明是比较老的版本，最后一个没有version的版本是2.1.1
		} else {
			SetAgentVersion(ctx, agentVersion)
		}

		return nil
	}

}

func getAgentIPFromContext(ctx context.Context) (string, bool) {
	md, exist := metadata.FromIncomingContext(ctx)
	if !exist {
		return "", exist
	}

	agentIP := md.Get(common.GOPHERCRON_AGENT_IP_MD_KEY)
	if len(agentIP) == 0 {
		return "", false
	}
	return agentIP[0], true
}

func SetAgentIP(ctx context.Context, agentIP string) {
	middleware.SetInto(ctx, agentIPKey{}, agentIP)
}

func GetAgentIP(ctx context.Context) (string, bool) {
	ip, ok := middleware.GetFrom(ctx, agentIPKey{}).(string)
	return ip, ok
}

type agentVersionKey struct{}

func SetAgentVersion(ctx context.Context, version string) {
	middleware.SetInto(ctx, agentVersionKey{}, version)
}

func GetAgentVersion(ctx context.Context) (string, bool) {
	version, ok := middleware.GetFrom(ctx, agentVersionKey{}).(string)
	return version, ok
}
