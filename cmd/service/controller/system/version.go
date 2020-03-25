package system

import (
	"github.com/gin-gonic/gin"
	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/response"
)

func GetVersion(c *gin.Context) {
	srv := app.GetApp(c)
	response.APISuccess(c, srv.GetVersion())
}
