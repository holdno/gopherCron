package etcd_func

import (
	"github.com/gin-gonic/gin"
	"ojbk.io/gopherCron/cmd/service/request"
	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/pkg/db"
	"ojbk.io/gopherCron/pkg/etcd"
	"ojbk.io/gopherCron/utils"
)

// GetTaskListRequest 获取任务列表请求参数
type GetTaskListRequest struct {
	Project string `form:"project" binding:"required"`
}

// GetList 获取任务列表
func GetTaskList(c *gin.Context) {
	var (
		taskList []*common.TaskInfo
		errObj   errors.Error
		err      error
		req      GetTaskListRequest
		uid      string
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		errObj = errors.ErrInvalidArgument
		errObj.Log = "[Controller - GetList] GetListRequest args error:" + err.Error()
		request.APIError(c, errObj)
		return
	}

	uid = c.GetString("jwt_user")

	if _, err = db.CheckProjectExist(req.Project, uid); err != nil {
		request.APIError(c, err)
		return
	}

	if taskList, err = etcd.Manager.GetTaskList(req.Project); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Controller - GetList] GetListRequest args error:" + err.Error()
		request.APIError(c, errObj)
		return
	}

	request.APISuccess(c, &gin.H{
		"list": utils.TernaryOperation(taskList != nil, taskList, []struct{}{}),
	})
}

// GetWorkerListRequest 获取节点的请求参数
type GetWorkerListRequest struct {
	Project string `form:"project" binding:"required"`
}

// GetWorkerList 获取节点
func GetWorkerList(c *gin.Context) {
	var (
		err error
		req GetWorkerListRequest
		res []string
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if res, err = etcd.Manager.GetWorkerList(req.Project); err != nil {
		request.APIError(c, err)
		return
	}

	request.APISuccess(c, &gin.H{
		"list": utils.TernaryOperation(res != nil, res, []struct{}{}),
	})
}
