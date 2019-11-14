package etcd_func

import (
	"strconv"
	"strings"

	"ojbk.io/gopherCron/app"
	"ojbk.io/gopherCron/cmd/service/response"
	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/utils"

	"github.com/gin-gonic/gin"
)

// GetTaskListRequest 获取任务列表请求参数
type GetTaskListRequest struct {
	ProjectID int64 `form:"project_id" binding:"required"`
}

// GetList 获取任务列表
func GetTaskList(c *gin.Context) {
	var (
		taskList []*common.TaskInfo
		errObj   errors.Error
		err      error
		req      GetTaskListRequest
		uid      = utils.GetUserID(c)
		srv      = app.GetApp(c)
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		errObj = errors.ErrInvalidArgument
		errObj.Log = "[Controller - GetList] GetListRequest args error:" + err.Error()
		response.APIError(c, errObj)
		return
	}

	if _, err = srv.CheckUserIsInProject(req.ProjectID, uid); err != nil {
		response.APIError(c, err)
		return
	}

	if taskList, err = srv.GetTaskList(req.ProjectID); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Controller - GetList] GetListRequest args error:" + err.Error()
		response.APIError(c, errObj)
		return
	}

	response.APISuccess(c, &gin.H{
		"list": utils.TernaryOperation(taskList != nil, taskList, []struct{}{}),
	})
}

// GetWorkerListRequest 获取节点的请求参数
type GetWorkerListRequest struct {
	ProjectID int64 `form:"project_id" binding:"required"`
}

// GetWorkerList 获取节点
func GetWorkerList(c *gin.Context) {
	var (
		err error
		req GetWorkerListRequest
		res []string
		srv = app.GetApp(c)
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if res, err = srv.GetWorkerList(req.ProjectID); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, &gin.H{
		"list": utils.TernaryOperation(res != nil, res, []struct{}{}),
	})
}

// 通过多个projectID来获取所有workerlist
// GetWorkerListInfoRequest 获取节点的请求参数
type GetWorkerListInfoRequest struct {
	ProjectIDs string `form:"project_ids" binding:"required"`
}

// GetWorkerList 获取节点
func GetWorkerListInfo(c *gin.Context) {
	var (
		err                error
		req                GetWorkerListInfoRequest
		workerList         []string
		noRepeatWorkerList []string
		projects           []string
		monitorList        []*common.MonitorInfo
		srv                = app.GetApp(c)
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, errors.ErrInvalidArgument)
		return
	}

	projects = strings.Split(req.ProjectIDs, ",")

	if len(projects) == 0 {
		response.APISuccess(c, &gin.H{
			"list": []struct{}{},
		})
		return
	}

	for _, v := range projects {
		id, _ := strconv.ParseInt(v, 10, 64)
		if workerList, err = srv.GetWorkerList(id); err != nil {
			response.APIError(c, err)
			return
		}

		for _, worker := range workerList {
			if !utils.StrArrExist(noRepeatWorkerList, worker) {
				noRepeatWorkerList = append(noRepeatWorkerList, worker)
			}
		}
	}

	// 遍历去重后的节点列表 获取对应的监控信息
	for _, worker := range noRepeatWorkerList {
		if m, err := srv.GetMonitor(worker); err == nil {
			monitorList = append(monitorList, m)
		} else {
			response.APIError(c, err)
			return
		}
	}

	response.APISuccess(c, &gin.H{
		"list": utils.TernaryOperation(monitorList != nil, monitorList, []struct{}{}),
	})
}
