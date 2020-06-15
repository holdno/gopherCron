package log_func

import (
	"time"

	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/utils"

	"github.com/gin-gonic/gin"
)

// GetListRequest 获取任务执行日志
type GetListRequest struct {
	Page      int    `form:"page" binding:"required"`
	Pagesize  int    `form:"pagesize" binding:"required"`
	ProjectID int64  `form:"project_id" binding:"required"`
	TaskID    string `form:"task_id" binding:"required"`
}

// GetList 获取任务执行日志
func GetList(c *gin.Context) {
	var (
		err     error
		req     GetListRequest
		logList []*common.TaskLog
		total   int
		exist   bool
		uid     = utils.GetUserID(c)
		srv     = app.GetApp(c)
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
		if exist, err = srv.CheckUserIsInProject(req.ProjectID, uid); err != nil {
			response.APIError(c, err)
			return
		}

		if !exist {
			response.APIError(c, errors.ErrProjectNotExist)
			return
		}
	}

	if logList, err = srv.GetTaskLogList(req.ProjectID, req.TaskID, req.Page, req.Pagesize); err != nil {
		response.APIError(c, err)
		return
	}

	if total, err = srv.GetTaskLogTotal(req.ProjectID, req.TaskID); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, &gin.H{
		"list":  utils.TernaryOperation(logList != nil, logList, []struct{}{}),
		"total": total,
	})
}

type GetRecentLogCountResponse struct {
	SuccessCount int    `json:"success_count"`
	ErrorCount   int    `json:"error_count"`
	Date         string `json:"date"`
}

// 获取最近七天的任务执行情况
func GetRecentLogCount(c *gin.Context) {
	var (
		err        error
		projects   []*common.Project
		projectIDs []int64
		result     []*GetRecentLogCountResponse

		uid = utils.GetUserID(c)
		srv = app.GetApp(c)
	)

	if projects, err = srv.GetUserProjects(uid); err != nil {
		response.APIError(c, err)
		return
	}

	if projects == nil {
		goto EMPTY
	}

	for _, v := range projects {
		projectIDs = append(projectIDs, v.ID)
	}

	for i := 6; i >= 0; i-- {
		var (
			successCount int
			errorCount   int
			timer        = utils.GetDateFromNow(-i)
		)

		if successCount, err = srv.GetLogTotalByDate(projectIDs, timer.Unix(), common.SuccessLog); err != nil {
			response.APIError(c, err)
			return
		}

		if errorCount, err = srv.GetLogTotalByDate(projectIDs, timer.Unix(), common.ErrorLog); err != nil {
			response.APIError(c, err)
			return
		}

		result = append(result, &GetRecentLogCountResponse{
			SuccessCount: successCount,
			ErrorCount:   errorCount,
			Date:         timer.Add(time.Duration(5) * time.Second).Format("2006-01-02"),
		})
	}

EMPTY:
	response.APISuccess(c, utils.TernaryOperation(len(result) != 0, result, []struct{}{}))
}
