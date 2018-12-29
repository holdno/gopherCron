/*
Package jwt 单元测试
Modified by chenguolin 2018-12-06
*/
package jwt

import (
	"testing"
)

func TestBuild(t *testing.T) {
	token := Build("")
	t.Log(token)
}

const verifyToken = "eyJ0eXAiOiJKV1QiLCJhbGciOiJTSDI1NiJ9.eyJiaXoiOiJmYW1pbHkiLCJ1c2VyIjoiMTIzNDU2IiwiZXhwIjoxNTQyODE0ODYyLCJpYXQiOjE1NDI3NzE2NjJ9.rmUq38RKM1NZsLoCGZzfAjdF6dqBd02xCguR0Yplrc4="

func TestVerify(t *testing.T) {
	t.Log(Verify(verifyToken))
}
