package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/holdno/gopherCron/common"

	"github.com/gin-gonic/gin"
	"github.com/holdno/gopherCron/utils"
)

const (
	secret = "123123"
)

func main() {
	engine := gin.New()
	engine.POST("/webhook", func(c *gin.Context) {
		_body, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			panic(err)
		}
		var req common.WebHookBody
		if err = json.Unmarshal(_body, &req); err != nil {
			c.String(http.StatusBadRequest, "bad request")
			return
		}

		sign := req.Sign
		req.Sign = ""

		newSign := utils.MakeSign(req, "123123")
		if sign != newSign || time.Now().Unix()-5 > req.RequestTime {
			c.String(http.StatusUnauthorized, "unauthorized")
			return
		}

		fmt.Println("handle success")

		c.String(http.StatusOK, req.Result)
	})

	httpServer := &http.Server{
		Addr:        ":9988",
		Handler:     engine,
		ReadTimeout: time.Duration(5) * time.Second,
	}

	fmt.Println(utils.GetCurrentTimeText(), "listening and serving HTTP on :9988")
	err := httpServer.ListenAndServe()
	if err != nil {
		fmt.Fprintln(os.Stderr, "http server start failed:", err)
		os.Exit(0)
	}
}
