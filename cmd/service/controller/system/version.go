package system

import (
	"github.com/gin-gonic/gin"
	"ojbk.io/gopherCron/app"
	"ojbk.io/gopherCron/cmd/service/response"
)

func GetVersion(c *gin.Context) {
	srv := app.GetApp(c)
	response.APISuccess(c, srv.GetVersion())
}
