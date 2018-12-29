package project_func

import (
	"github.com/gin-gonic/gin"
	"ojbk.io/gopherCron/cmd/service/request"
	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/pkg/db"
	"ojbk.io/gopherCron/utils"
)

func GetUserProjects(c *gin.Context) {
	var (
		err  error
		list []*common.Project
		uid  string
	)

	uid = c.GetString("jwt_user")

	if list, err = db.GetUserProjects(uid); err != nil {
		request.APIError(c, err)
		return
	}

	request.APISuccess(c, &gin.H{
		"list": utils.TernaryOperation(list != nil, list, []struct{}{}),
	})
}
