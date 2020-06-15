package etcd_func

import (
	"fmt"

	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/utils"

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

	isAdmin, err := srv.IsAdmin(uid)
	if err != nil {
		response.APIError(c, err)
		return
	}

	if !isAdmin {
		if _, err = srv.CheckUserIsInProject(req.ProjectID, uid); err != nil {
			response.APIError(c, err)
			return
		}
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
type GetClientListRequest struct {
	ProjectID int64 `form:"project_id" binding:"required"`
}

// GetWorkerList 获取节点
func GetClientList(c *gin.Context) {
	var (
		err error
		req GetClientListRequest
		res []app.ClientInfo
		srv = app.GetApp(c)
		uid = utils.GetUserID(c)
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, errors.ErrInvalidArgument)
		return
	}

	isAdmin, err := srv.IsAdmin(uid)
	if err != nil {
		response.APIError(c, err)
		return
	}

	if !isAdmin {
		exist, err := srv.CheckUserIsInProject(req.ProjectID, uid)
		if err != nil {
			response.APIError(c, err)
			return
		}

		if !exist {
			response.APIError(c, errors.ErrUnauthorized)
			return
		}
	}

	if res, err = srv.GetWorkerList(req.ProjectID); err != nil {
		response.APIError(c, err)
		return
	}

	var list []string
	for _, v := range res {
		list = append(list, fmt.Sprintf("%s:%s", v.ClientIP, v.Version))
	}

	response.APISuccess(c, &gin.H{
		"list": utils.TernaryOperation(list != nil, list, []struct{}{}),
	})
}

// 通过多个projectID来获取所有workerlist
// GetWorkerListInfoRequest 获取节点的请求参数
type GetWorkerListInfoRequest struct {
	ProjectIDs string `form:"project_ids"`
}

type GetWorkerListInfoResponse struct {
	List []UserWorkerInfo `json:"list"`
}

type UserWorkerInfo struct {
	ProjectID int64  `json:"project_id"`
	ClientIP  string `json:"client_ip"`
	Version   string `json:"version"`
}

// GetWorkerList 获取节点
func GetWorkerListInfo(c *gin.Context) {
	var (
		err                error
		req                GetWorkerListInfoRequest
		workerList         []app.ClientInfo
		noRepeatWorkerList []string
		srv                = app.GetApp(c)
		uid                = utils.GetUserID(c)
		result             []UserWorkerInfo
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, errors.ErrInvalidArgument)
		return
	}

	projects, err := srv.GetUserProjects(uid)
	if err != nil {
		response.APIError(c, err)
		return
	}

	if len(projects) == 0 {
		response.APISuccess(c, result)
		return
	}

	for _, v := range projects {
		if workerList, err = srv.GetWorkerList(v.ID); err != nil {
			response.APIError(c, err)
			return
		}

		for _, worker := range workerList {
			if !utils.StrArrExist(noRepeatWorkerList, worker.ClientIP) {
				result = append(result, UserWorkerInfo{
					ProjectID: v.ID,
					ClientIP:  worker.ClientIP,
					Version:   worker.Version,
				})
				noRepeatWorkerList = append(noRepeatWorkerList, worker.ClientIP)
			}
		}
	}

	// 遍历去重后的节点列表 获取对应的监控信息
	response.APISuccess(c, result)
}

type ReloadConfigRequest struct {
	ClientIP  string `json:"client_ip" form:"client_ip" binding:"required"`
	ProjectID int64  `json:"project_id" form:"project_id" binding:"required"`
}

func ReloadConfig(c *gin.Context) {
	var (
		err error
		req ReloadConfigRequest
		srv = app.GetApp(c)
		uid = utils.GetUserID(c)
		ok  bool
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, errors.ErrInvalidArgument)
		return
	}

	isAdmin, err := srv.IsAdmin(uid)
	if err != nil {
		response.APIError(c, err)
		return
	}

	if !isAdmin {
		if ok, err = srv.CheckUserIsInProject(req.ProjectID, uid); err != nil {
			response.APIError(c, err)
			return
		}

		if !ok {
			response.APIError(c, errors.ErrUnauthorized)
			return
		}
	}

	if ok, err = srv.CheckProjectWorkerExist(req.ProjectID, req.ClientIP); err != nil {
		response.APIError(c, err)
		return
	}

	if !ok {
		response.APIError(c, errors.ErrUnauthorized)
		return
	}

	if err = srv.ReloadWorkerConfig(req.ClientIP); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}
