package log_func

import (
	"github.com/gin-gonic/gin"
	"ojbk.io/gopherCron/cmd/service/request"
	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/pkg/db"
	"ojbk.io/gopherCron/utils"
)

// GetListRequest 获取任务执行日志
type GetListRequest struct {
	Page     int64  `form:"page" binding:"required"`
	Pagesize int64  `form:"pagesize" binding:"required"`
	Project  string `form:"project" binding:"required"`
	Name     string `form:"name" binding:"required"`
}

// GetList 获取任务执行日志
func GetList(c *gin.Context) {
	var (
		err     error
		req     GetListRequest
		logList []*common.TaskLog
		total   int64
		uid     string
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	uid = c.GetString("jwt_user")

	if _, err = db.CheckProjectExist(req.Project, uid); err != nil {
		request.APIError(c, err)
		return
	}

	if logList, err = db.GetLogList(req.Project, req.Name, req.Page, req.Pagesize); err != nil {
		request.APIError(c, err)
		return
	}

	if total, err = db.GetLogTotal(req.Project, req.Name); err != nil {
		request.APIError(c, err)
		return
	}

	request.APISuccess(c, &gin.H{
		"list":  utils.TernaryOperation(logList != nil, logList, []struct{}{}),
		"total": total,
	})
}
