package jwt

import (
	"testing"
	"time"
)

const (
	privateKey = `
-----BEGIN PRIVATE KEY-----
MIICdgIBADANBgkqhkiG9w0BAQEFAASCAmAwggJcAgEAAoGBAMzza7z4KGRHB2k3
FvMnKmKjTFv6N3BYt3kNFwovRxTwWUIPp8kf5i4xMxJ/1ElqpBIirSkqHJ8c8dad
NLAuq52TOYa18qy6nD2uk+QXblv1Q4cveHQo91dpCCPzroPNzjzck9yq5GUmysQ6
F+ljW8K3parT/+8zv0kEUFWgnfWbAgMBAAECgYBd4h/3R2IRVWwyqVas+cLzvkQr
WfptT2Z0YCeutauFDviER3GfsyoY/NadYcsX+m7AE/xof+7ugC7UFd1d23MnAL4V
8gDtD3YpIT8A+4lMV3EDmmhPxdbTTNMMK29rbMxeiaawTAyaF6B/ywTzwPOkPXuX
wDR47br2TjavM0/yMQJBAOb/ayLx6Qe86P1zNMzX948+s0E+hlixwDNhH5Glagox
22NOvN3/5gA2cqBJOg9xLxGLbfGLqFn0gcKOfFMguvkCQQDjIkjwZVGOgFS4+MH2
MJOVJ7RTFrNs5+ho/iH6MVwJc6SGMhDlklWWHLxm0N6sK9FnJ6hsxlFAnbGhRbZk
ReYzAkEAqDHcaaJpAfhcUYdcL7clC4kk7mG/Yr9yajbSzLL75hZtXv7K6H5Wk1sR
1YHcI7hPBGBYmmMNHwq4nNgw0DppyQJAW+BsfMGfQfNrUf9eBkYUDMuox8tw/Oa6
Pm4+NERvJGug65+o8hRFhplNJJHs4NxAsmd6W7XE/ExNpBzc8KbNvQJAOhi61BCp
McN8bX27y5HRVwi45GWtfH5GyEWlsCrLMkXB43JeY+uukQ5tYRxFr5jwB0Ml0Gx1
0eb2zriCpjbd/w==
-----END PRIVATE KEY-----
`

	publicKey = `
-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDM82u8+ChkRwdpNxbzJypio0xb
+jdwWLd5DRcKL0cU8FlCD6fJH+YuMTMSf9RJaqQSIq0pKhyfHPHWnTSwLqudkzmG
tfKsupw9rpPkF25b9UOHL3h0KPdXaQgj866Dzc483JPcquRlJsrEOhfpY1vCt6Wq
0//vM79JBFBVoJ31mwIDAQAB
-----END PUBLIC KEY-----
`
)

func TestGenerate(t *testing.T) {
	token, err := Generate(TokenClaims{
		User: 1,
		Biz:  DefaultBIZ,
		Iat:  time.Now().Unix(),
		Exp:  time.Now().Add(time.Minute).Unix(),
	}, []byte(privateKey))
	if err != nil {
		t.Fatal(err)
	}

	t.Log(token)

	result := Parse(token, []byte(publicKey))
	if result.Error != "" {
		t.Fatal(result.Error)
	}

	t.Log("user:", result.User)
}
