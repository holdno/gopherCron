package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
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
		projectSecret := c.Request.Header.Get("Authorization")
		if projectSecret != secret {
			c.Status(http.StatusUnauthorized)
			return
		}

		_body, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			panic(err)
		}
		var req common.WebHookBody
		event := cloudevents.NewEvent()
		if err = json.Unmarshal(_body, &event); err != nil {
			c.String(http.StatusBadRequest, "bad request")
			return
		}

		if err := event.Validate(); err != nil {
			c.String(http.StatusBadRequest, "bad request")
			return
		}

		if err = event.DataAs(&req); err != nil {
			c.String(http.StatusBadRequest, "bad request")
			return
		}

		if req.Error != "" {
			// alert
			fmt.Println("got task error event", req.TaskName, req.ProjectName, req.Operator, req.Error)
		}

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
