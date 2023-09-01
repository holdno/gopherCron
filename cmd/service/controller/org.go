package controller

import "github.com/gin-gonic/gin"

func GetUserOrgList(c *gin.Context) {
	var (
		err error
		uid = utils.GetUserID(c)
		srv = app.GetApp(c)
	)


}