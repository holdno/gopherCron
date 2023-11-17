/*
Package jwt jwt生成算法
Created by wangboyan 2018-11-21
*/
package jwt

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
)

const DefaultBIZ = "gophercron"

type VerifyRes struct {
	Code  int
	Error string
	Biz   string
	User  int64
}

// Jwt 结构定义
type TokenClaims struct {
	Biz  string `json:"biz"`  // 对应标准中的iss 表示业务方
	User int64  `json:"user"` // 对应标准中的aud 表示接收用户
	Exp  int64  `json:"exp"`
	Iat  int64  `json:"iat"`
}

func BuildUserJWT(user int64, expHour int, pk []byte) (string, error) {
	return Generate(TokenClaims{
		Biz:  DefaultBIZ,
		User: user,
		Exp:  int64(time.Now().Add(time.Duration(expHour) * time.Hour).Unix()),
		Iat:  time.Now().Unix(),
	}, pk)
}

func Generate(info TokenClaims, pk []byte) (string, error) {
	claims := jwt.MapClaims{}

	_t := reflect.TypeOf(info)
	v := reflect.ValueOf(info)

	for i := 0; i < _t.NumField(); i++ {
		tag := _t.Field(i).Tag.Get("json")
		claims[tag] = v.Field(i).Interface()
	}

	_token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(pk)
	if err != nil {
		return "", err
	}
	return _token.SignedString(privateKey)
}

func Parse(tokenString string, key []byte) *VerifyRes {
	result := TokenClaims{}
	_token, err := jwt.Parse(tokenString, func(i2 *jwt.Token) (i interface{}, e error) {
		publicKey, err := jwt.ParseRSAPublicKeyFromPEM(key)
		if err != nil {
			return nil, fmt.Errorf("invalied public key, %w", err)
		}
		return publicKey, nil
	})

	if err != nil {
		return &VerifyRes{
			Code:  4001,
			Error: "token is invalid",
		}
	}

	if !_token.Valid {
		return &VerifyRes{
			Code:  4001,
			Error: "token is invalid",
		}
	}

	// 验证token是否过期
	claims, ok := _token.Claims.(jwt.MapClaims)
	if !ok {
		return &VerifyRes{
			Code:  4001,
			Error: "token is invalid",
		}
	}

	if !claims.VerifyExpiresAt(time.Now().Unix(), false) {
		return &VerifyRes{
			Code:  4004,
			Error: "token is expired",
		}
	}

	parts := strings.Split(tokenString, ".")
	claimBytes, _ := jwt.DecodeSegment(parts[1])

	if err = json.Unmarshal(claimBytes, &result); err != nil {
		return &VerifyRes{
			Code:  4001,
			Error: "token is invalid",
		}
	}
	return &VerifyRes{
		Code:  1000,
		Error: "",
		User:  result.User,
		Biz:   result.Biz,
	}
}
