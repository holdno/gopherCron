package controller

import (
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/jwt"
	"github.com/holdno/gopherCron/utils"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/holdno/firetower/protocol"
	"github.com/holdno/firetower/service/tower"
	json "github.com/json-iterator/go"
	"github.com/spacegrower/watermelon/infra/wlog"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func Websocket(tm tower.Manager[app.CloudEventWithNil]) func(c *gin.Context) {
	if tm == nil {
		return func(c *gin.Context) {
			response.APIError(c, errors.NewError(http.StatusForbidden, "this server not install firetower service"))
		}
	}
	return func(c *gin.Context) {
		var ws *websocket.Conn
		var err error
		token := c.Query("token")
		srv := app.GetApp(c)
		userToken := jwt.Parse(token, []byte(srv.GetConfig().JWT.PublicKey))
		if userToken.Code != 1000 {
			response.APIError(c, errors.NewError(http.StatusForbidden, "permission denied"))
			return
		}

		if token == "" {
			// 做用户身份验证
			token := c.Request.Header.Get("Sec-WebSocket-Protocol")
			// chatLogic := core.NewChatLogic(c.Request.Context(), s.core)
			// 验证成功才升级连接
			ws, _ = upgrader.Upgrade(c.Writer, c.Request, map[string][]string{
				"Sec-WebSocket-Protocol": {token},
			})
		} else {
			// chatLogic := core.NewChatLogic(c.Request.Context(), s.core)
			// 验证成功才升级连接
			ws, err = upgrader.Upgrade(c.Writer, c.Request, nil)
			if err != nil {
				wlog.Error("Websocket Upgrade err", zap.Error(err))
				response.APIError(c, errors.NewError(http.StatusInternalServerError, "failed to upgrade http").WithLog(err.Error()))
			}
		}

		id := utils.GetStrID()
		newTower, err := tm.BuildTower(ws, id)
		if err != nil {
			response.APIError(c, errors.NewError(http.StatusInternalServerError, "failed to build firetower").WithLog(err.Error()))
			return
		}

		newTower.SetUserID(strconv.FormatInt(userToken.User, 10))

		newTower.SetReadHandler(func(fire protocol.ReadOnlyFire[app.CloudEventWithNil]) bool {
			// 当前用户是不能通过websocket发送消息的，所以固定返回false
			return false
		})

		newTower.SetReceivedHandler(func(fi protocol.ReadOnlyFire[app.CloudEventWithNil]) bool {
			return true
		})

		newTower.SetReadTimeoutHandler(func(fire protocol.ReadOnlyFire[app.CloudEventWithNil]) {
			wlog.Error("read timeout trigger", zap.String("component", "firetower"))
		})

		newTower.SetBeforeSubscribeHandler(func(fireCtx protocol.FireLife, topics []string) bool {
			for _, v := range topics {
				if strings.Contains(v, "project") {
					_pid := filepath.Base(v)
					pid, err := strconv.ParseInt(_pid, 10, 64)
					if err != nil {
						wlog.Error("failed to parse project id from topic", zap.String("component", "firetower"),
							zap.String("topic", v))
						return false
					}
					if _, err = srv.CheckUserIsInProject(pid, userToken.User); err != nil {
						wlog.Error("failed to subscribe topic, user is not belong to project", zap.String("component", "firetower"),
							zap.Int64("user", userToken.User), zap.String("topic", v))
						return false
					}
				}
			}
			return true
		})

		newTower.SetSubscribeHandler(func(context protocol.FireLife, topic []string) {
			for _, v := range topic {
				resp := &protocol.TopicMessage[json.RawMessage]{
					Topic: v,
					Type:  protocol.SubscribeOperation,
				}
				resp.Data = json.RawMessage(`{"status":"success"}`)
				msg, _ := json.Marshal(resp)
				newTower.SendToClient(msg)

			}
		})

		newTower.SetUnSubscribeHandler(func(context protocol.FireLife, topic []string) {
			for _, v := range topic {
				resp := &protocol.TopicMessage[json.RawMessage]{
					Topic: v,
					Type:  protocol.UnSubscribeOperation,
				}
				resp.Data = json.RawMessage(`{"status":"success"}`)
				msg, _ := json.Marshal(resp)
				newTower.SendToClient(msg)
			}
		})

		newTower.Run()
	}

}
