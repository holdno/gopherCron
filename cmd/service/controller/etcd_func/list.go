package etcd_func

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mongodb/mongo-go-driver/bson/primitive"
	"ojbk.io/gopherCron/cmd/service/request"
	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/pkg/db"
	"ojbk.io/gopherCron/pkg/etcd"
	"ojbk.io/gopherCron/utils"
)

// GetTaskListRequest 获取任务列表请求参数
type GetTaskListRequest struct {
	ProjectID string `form:"project_id" binding:"required"`
}

// GetList 获取任务列表
func GetTaskList(c *gin.Context) {
	var (
		taskList  []*common.TaskInfo
		errObj    errors.Error
		err       error
		req       GetTaskListRequest
		uid       string
		projectID primitive.ObjectID
		userID    primitive.ObjectID
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		errObj = errors.ErrInvalidArgument
		errObj.Log = "[Controller - GetList] GetListRequest args error:" + err.Error()
		request.APIError(c, errObj)
		return
	}

	uid = c.GetString("jwt_user")

	if userID, err = primitive.ObjectIDFromHex(uid); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if projectID, err = primitive.ObjectIDFromHex(req.ProjectID); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if _, err = db.CheckProjectExist(projectID, userID); err != nil {
		request.APIError(c, err)
		return
	}

	if taskList, err = etcd.Manager.GetTaskList(req.ProjectID); err != nil {
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
	ProjectID string `form:"project_id" binding:"required"`
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

	if res, err = etcd.Manager.GetWorkerList(req.ProjectID); err != nil {
		request.APIError(c, err)
		return
	}

	request.APISuccess(c, &gin.H{
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
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	projects = strings.Split(req.ProjectIDs, ",")

	if len(projects) == 0 {
		request.APISuccess(c, &gin.H{
			"list": []struct{}{},
		})
		return
	}

	for _, v := range projects {
		if workerList, err = etcd.Manager.GetWorkerList(v); err != nil {
			request.APIError(c, err)
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
		if m, err := etcd.Manager.GetMonitor(worker); err == nil {
			monitorList = append(monitorList, m)
		} else {
			request.APIError(c, err)
			return
		}
	}

	request.APISuccess(c, &gin.H{
		"list": utils.TernaryOperation(monitorList != nil, monitorList, []struct{}{}),
	})
}
