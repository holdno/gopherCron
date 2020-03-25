/*
Package jwt jwt生成算法
Created by wangboyan 2018-11-21
*/
package jwt

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/holdno/gopherCron/config"
)

var (
	secret   = ""
	expHours = 0 // 过期时间
	biz      = "goperCron"
)

// InitJWT
func InitJWT(conf *config.JWTConf) {
	secret = conf.Secret
	expHours = conf.Exp
}

// Build 生成AccessToken
func Build(user int64) string {
	Token := build("SH256") // 传入JWT加密算法
	Token.Payload.Exp = time.Now().Add(time.Hour * time.Duration(expHours)).Unix()
	Token.Payload.User = user
	Token.Payload.Biz = biz
	j, _ := Token.ToJSON(secret) // 传入密钥
	return j
}

// VerifyRes 验证响应
type VerifyRes struct {
	Code  int
	Error string
	Biz   string
	User  int64
}

// Verify 验证JWT的方法
func Verify(verify string) *VerifyRes {
	token := strings.Split(verify, ".") // [header payload signature]

	if len(token) != 3 {
		return &VerifyRes{
			Code:  4001,
			Error: "format error",
		}
	}

	header, _ := base64.StdEncoding.DecodeString(token[0])
	payload, _ := base64.StdEncoding.DecodeString(token[1])

	var headerStruct Header
	err := json.Unmarshal([]byte(header), &headerStruct)
	if err != nil {
		return &VerifyRes{
			Code:  4002,
			Error: "jwt unmarshal error: header has hint",
		}
	}

	var payloadStruct Payload
	err = json.Unmarshal([]byte(payload), &payloadStruct)
	if err != nil {
		return &VerifyRes{
			Code:  4003,
			Error: "jwt unmarshal error: payload has hint",
		}
	}

	// 验证是否过期
	now := time.Now().Unix()
	if now > payloadStruct.Exp {
		// 过期
		return &VerifyRes{
			Code:  4004,
			Error: "token is expired",
		}
	}

	if headerStruct.Alg == "SH256" {
		// 加密处理
		signature := SignatureBuild(token[0]+"."+token[1], secret)
		if signature == token[2] {
			// 验证通过
			return &VerifyRes{
				Code:  1000,
				Error: "",
				User:  payloadStruct.User,
				Biz:   payloadStruct.Biz,
			}
		}
		// 验证不通过
		return &VerifyRes{
			Code:  5001,
			Error: "verification failed",
			User:  payloadStruct.User,
			Biz:   payloadStruct.Biz,
		}

	}
	// 发现加密方式与本服务定义的不符，所以该token肯定不是自己的
	return &VerifyRes{
		Code:  4005,
		Error: "not recognized",
	}
}
