/*
Package jwt jwt生成算法
Created by wangboyan 2018-11-21
*/
package jwt

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"time"
)

// Jwt 结构定义
type Jwt struct {
	Header  Header
	Payload Payload
}

// Header jwt的头部承载两部分信息
// 声明类型，这里是jwt
// 声明加密的算法 通常直接使用 HMAC SHA256
// 完整的头部就像下面这样的JSON：
// {
//   'typ': 'JWT',
//   'alg': 'HS256'
// }
type Header struct {
	Typ string `json:"typ"`
	Alg string `json:"alg"`
}

// Payload 载荷就是存放有效信息的地方。这个名字像是特指飞机上承载的货品，这些有效信息包含三个部分
// 标准中注册的声明
// 公共的声明
// 私有的声明
// 标准中注册的声明 (建议但不强制使用) ：
// iss: jwt签发者
// sub: jwt所面向的用户
// aud: 接收jwt的一方
// exp: jwt的过期时间，这个过期时间必须要大于签发时间
// nbf: 定义在什么时间之前，该jwt都是不可用的.
// iat: jwt的签发时间
// jti: jwt的唯一身份标识，主要用来作为一次性token,从而回避重放攻击。
// 公共的声明 ：
// 公共的声明可以添加任何的信息，一般添加用户的相关信息或其他业务需要的必要信息.但不建议添加敏感信息，因为该部分在客户端可解密.
// 私有的声明 ：
// 私有声明是提供者和消费者所共同定义的声明，一般不建议存放敏感信息，因为base64是对称解密的，意味着该部分信息可以归类为明文信息。
type Payload struct {
	Biz  string `json:"biz"`  // 对应标准中的iss 表示业务方
	User int64  `json:"user"` // 对应标准中的aud 表示接收用户
	Exp  int64  `json:"exp"`
	Iat  int64  `json:"iat"`
}

// Token jwt的第三部分是一个签证信息，这个签证信息由三部分组成：
// header (base64后的)
// payload (base64后的)
// secret
// 这个部分需要base64加密后的header和base64加密后的payload使用.连接组成的字符串，
// 然后通过header中声明的加密方式进行加盐secret组合加密，然后就构成了jwt的第三部分。
// 三个部分用"."拼接成一个完整的字符串,构成了最终的jwt
// 注意：secret是保存在服务器端的，jwt的签发生成也是在服务器端的，secret就是用来进行jwt的签发和jwt的验证，
// 所以，它就是你服务端的私钥，在任何场景都不应该流露出去。一旦客户端得知这个secret, 那就意味着客户端是可以自我签发jwt了。
type Token struct {
	Token string `json:"token"`
}

// 初始化JWT参数模板 根据业务定义好Header 和 Payload的结构
func build(method string) *Jwt {
	return &Jwt{
		Header{
			Typ: "JWT",
			Alg: method,
		},
		Payload{
			User: 0,
			Exp:  0,
			Iat:  time.Now().Unix(),
		},
	}
}

// ToJSON 生成最终token字符串
func (jwt *Jwt) ToJSON(secret string) (string, error) {
	// 将header转为json字符串
	header, err := json.Marshal(jwt.Header)
	if err != nil {
		return "", err
	}
	// 成功之后进行base64编码
	headerBase64 := base64.StdEncoding.EncodeToString([]byte(string(header)))

	// 将payload转字符串
	payload, err := json.Marshal(jwt.Payload)
	if err != nil {
		return "", err
	}
	// 成功后进行base64编码
	payloadBase64 := base64.StdEncoding.EncodeToString([]byte(string(payload)))

	// 第三段参数 是要对前两步生成的base64用.拼接 然后加盐进行加密
	token := headerBase64 + "." + payloadBase64

	// SH256加密(加盐加密)
	signature := SignatureBuild(token, secret)

	return token + "." + signature, err
}

// SignatureBuild 服务密钥SH256加密
// 当然加密方式有多种
// 这个服务就只用这一种，如果需要其他的自行拓展一下就可以了
// 但是注意要和 header.Alg 中定义的加密方式保持一致!
func SignatureBuild(token, secret string) string {
	key := []byte(secret)
	h := hmac.New(sha256.New, key)
	h.Write([]byte(token))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
