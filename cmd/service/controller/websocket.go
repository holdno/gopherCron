package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/jwt"
	"github.com/holdno/gopherCron/utils"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/holdno/firetower/protocol"
	"github.com/holdno/firetower/service/tower"
	"github.com/spacegrower/watermelon/infra/wlog"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func Websocket(c *gin.Context) {
	var ws *websocket.Conn
	var err error
	token := c.Query("token")
	srv := app.GetApp(c)
	res := jwt.Parse(token, []byte(srv.GetConfig().JWT.PublicKey))
	if res.Code != 1000 {
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
	newTower, err := tower.BuildTower(ws, id)
	if err != nil {
		response.APIError(c, errors.NewError(http.StatusInternalServerError, "failed to build firetower").WithLog(err.Error()))
		return
	}

	newTower.SetUserID(strconv.FormatInt(res.User, 10))

	newTower.SetReadHandler(func(fire *protocol.FireInfo) bool {
		// 当前用户是不能通过websocket发送消息的，所以固定返回false
		return false
	})

	newTower.SetReceivedHandler(func(fi *protocol.FireInfo) bool {
		return true
	})

	newTower.SetReadTimeoutHandler(func(fire *protocol.FireInfo) {
		wlog.Error("read timeout trigger", zap.String("component", "firetower"))
	})

	newTower.SetBeforeSubscribeHandler(func(fireCtx protocol.FireLife, topics []string) bool {
		for _,v := range topics {
			if strings.Contains(v, "project") {
				pid := filepath.Base(v)
				srv.CheckUserIsInProject()
			}
		}
		
		// 向接入系统验证用户是否可以注册这些topic
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		if err := core.NewWsSubscribeLogic(ctx, s.core).CheckSubscribePermission(app.ApplicationAppID, user, topics); err != nil {
			wlog.Error("failed to subscribe, no permission", zap.Error(err))
			newTower.Close()
			return false
		}

		var topicsWithNamespace []string
		for _, v := range topics {
			topicsWithNamespace = append(topicsWithNamespace, types.TopicWithNamespace(v, app.ApplicationAppID))
		}
		newTower.Subscribe(fireCtx, topicsWithNamespace)
		return false
	})

	newTower.SetSubscribeHandler(func(context protocol.FireLife, topic []string) bool {
		for _, v := range topic {
			resp := &protocol.TopicMessage{
				Topic: v,
				Type:  types.WS_EVENT_SYSTEM_ONSUBSCRIBE,
			}
			resp.Data = json.RawMessage(`{"status":"success"}`)
			msg, _ := json.Marshal(resp)
			newTower.ToSelf(msg)
			return true
		}
		return true
	})

	newTower.SetUnSubscribeHandler(func(context protocol.FireLife, topic []string) bool {
		for _, v := range topic {
			resp := &protocol.TopicMessage{
				Topic: types.TrimTopicAppPrefix(v, app.ApplicationAppID),
				Type:  types.WS_EVENT_SYSTEM_UNSUBSCRIBE,
			}
			resp.Data = json.RawMessage(`{"status":"success"}`)
			msg, _ := json.Marshal(resp)
			newTower.ToSelf(msg)
			return true
		}
		return true
	})

	newTower.Run()
}
