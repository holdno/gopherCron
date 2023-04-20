package syscli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/holdno/gopherCron/cmd/service/controller/user_func"
	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/common"
)

type TaskInfo struct {
	ProjectID int64
	TaskID    int64
	Name      string
	Timeout   int
	Command   string
	Noseize   int
}

type Infrastructure interface {
	Save(TaskInfo) error
	Delete(TaskInfo) error
	SaveResult(TaskInfo, common.TaskResultLog)
}

type Client struct {
	logger   Logger
	i        Infrastructure
	account  *account
	domain   string
	stopChan chan struct{}

	httpClient *http.Client
}

type Options func(c *Client)

func WithTimeout(s int) Options {
	return func(c *Client) {
		if c.httpClient == nil {
			c.httpClient = &http.Client{}
		}

		c.httpClient.Timeout = time.Duration(s) * time.Second
	}
}

func NewClient(account, password, domain string, base Infrastructure, opts ...Options) *Client {
	if account == "" ||
		password == "" ||
		domain == "" ||
		base == nil {
		panic("gopherCron syscli init error: empty params")
	}
	c := &Client{
		domain: domain,
		logger: newNopLogger(),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	c.account = registeAccount(c, account, password)

	return c
}

type account struct {
	client   *Client
	account  string
	password string
	username string
	userid   int64
	token    string
	ttl      int64
}

func registeAccount(c *Client, _account, password string) *account {
	aclient := &account{
		client:   c,
		account:  _account,
		password: password,
	}

	go wait.NonSlidingUntil(func() {
		if aclient.ttl != 0 {
			if time.Now().Before(time.Unix(aclient.ttl, 0).Add(-time.Hour)) {
				return
			}
		}
		if err := aclient.login(); err != nil {
			panic(fmt.Sprintf("failed to get token, error: %s", err.Error()))
		}

	}, time.Minute*10, c.stopChan)

	return aclient
}

type httpBaseResponse struct {
	Meta *response.Meta  `json:"meta"`
	Body json.RawMessage `json:"response"`
}

func (a *account) login() error {
	resp, err := a.client.httpClient.PostForm(a.client.domain+"/api/v1/user/login", url.Values{
		"account":  []string{a.account},
		"password": []string{a.password},
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var res httpBaseResponse
	if err = json.Unmarshal(_body, &res); err != nil {
		return err
	}

	if res.Meta.Code != 0 {
		return errors.New(res.Meta.Msg)
	}

	var body user_func.LoginResponse
	if err = json.Unmarshal(res.Body, &body); err != nil {
		return err
	}

	a.ttl = body.TTL
	a.token = body.Token
	a.userid = body.ID
	a.username = body.Name

	return nil
}
