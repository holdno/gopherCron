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

		ws, err = upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			wlog.Error("Websocket Upgrade err", zap.Error(err))
			response.APIError(c, errors.NewError(http.StatusInternalServerError, "failed to upgrade http").WithLog(err.Error()))
			return
		}

		id := utils.GetStrID()
		thisTower, err := tm.BuildTower(ws, id)
		if err != nil {
			response.APIError(c, errors.NewError(http.StatusInternalServerError, "failed to build firetower").WithLog(err.Error()))
			return
		}
		thisTower.SetUserID(strconv.FormatInt(userToken.User, 10))

		thisTower.SetReadHandler(func(fire protocol.ReadOnlyFire[app.CloudEventWithNil]) bool {
			// 当前用户是不能通过websocket发送消息的，所以固定返回false
			return false
		})

		thisTower.SetReceivedHandler(func(fi protocol.ReadOnlyFire[app.CloudEventWithNil]) bool {
			raw, err := json.Marshal(fi.GetMessage().Data.Event)
			if err != nil {
				wlog.Error("failed to marshal firetower received message", zap.Error(err))
				return false
			}
			thisTower.SendToClient(raw)
			return false
		})

		thisTower.SetReadTimeoutHandler(func(fire protocol.ReadOnlyFire[app.CloudEventWithNil]) {
			wlog.Error("read timeout trigger", zap.String("component", "firetower"))
		})

		thisTower.SetBeforeSubscribeHandler(func(fireCtx protocol.FireLife, topics []string) bool {
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

		thisTower.SetSubscribeHandler(func(context protocol.FireLife, topic []string) {
			for _, v := range topic {
				resp := &protocol.TopicMessage[json.RawMessage]{
					Topic: v,
					Type:  protocol.SubscribeOperation,
				}
				resp.Data = json.RawMessage(`{"status":"success"}`)
				msg, _ := json.Marshal(resp)
				thisTower.SendToClient(msg)
			}
		})

		thisTower.SetUnSubscribeHandler(func(context protocol.FireLife, topic []string) {
			for _, v := range topic {
				resp := &protocol.TopicMessage[json.RawMessage]{
					Topic: v,
					Type:  protocol.UnSubscribeOperation,
				}
				resp.Data = json.RawMessage(`{"status":"success"}`)
				msg, _ := json.Marshal(resp)
				thisTower.SendToClient(msg)
			}
		})

		thisTower.Run()
	}

}
