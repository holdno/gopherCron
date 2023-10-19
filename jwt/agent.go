package jwt

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/holdno/gopherCron/common"
	"github.com/spacegrower/watermelon/infra/middleware"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func AgentAuthMiddleware(publicKey []byte) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		md, exist := metadata.FromIncomingContext(ctx)
		if !exist {
			return status.Error(codes.Unauthenticated, codes.Unauthenticated.String())
		}

		jwt := md.Get(common.GOPHERCRON_CENTER_AUTH_KEY)
		if len(jwt) == 0 {
			return status.Error(codes.Unauthenticated, "auth key is undefined")
		}
		claims, err := ParseCenterJWT(jwt[0], publicKey)
		if err != nil {
			return err
		}

		if claims.UserID != 0 {
			middleware.SetInto(ctx, "request_user", claims.UserID)
		}
		return nil
	}
}

type AgentTokenClaims struct {
	Biz        string  `json:"biz"`
	ProjectIDs []int64 `json:"pids"`
	Exp        int64   `json:"exp"`
	Iat        int64   `json:"iat"`
}

func BuildAgentJWT(info AgentTokenClaims, pk []byte) (string, error) {
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

func ParseAgentJWT(tokenString string, key []byte) (*AgentTokenClaims, error) {
	result := AgentTokenClaims{}
	_token, err := jwt.Parse(tokenString, func(i2 *jwt.Token) (i interface{}, e error) {
		publicKey, err := jwt.ParseRSAPublicKeyFromPEM(key)
		if err != nil {
			return nil, fmt.Errorf("invalied public key, %w", err)
		}
		return publicKey, nil
	})

	if err != nil {
		return nil, err
	}

	if !_token.Valid {
		return nil, err
	}

	// 验证token是否过期
	claims, ok := _token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, err
	}

	if !claims.VerifyExpiresAt(time.Now().Unix(), false) {
		return nil, err
	}

	parts := strings.Split(tokenString, ".")
	claimBytes, _ := jwt.DecodeSegment(parts[1])

	if err = json.Unmarshal(claimBytes, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
