package log_func

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mongodb/mongo-go-driver/bson/primitive"
	"ojbk.io/gopherCron/cmd/service/request"
	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/pkg/db"
	"ojbk.io/gopherCron/utils"
)

// GetListRequest 获取任务执行日志
type GetListRequest struct {
	Page      int64  `form:"page" binding:"required"`
	Pagesize  int64  `form:"pagesize" binding:"required"`
	ProjectID string `form:"project_id" binding:"required"`
	TaskID    string `form:"task_id" binding:"required"`
}

// GetList 获取任务执行日志
func GetList(c *gin.Context) {
	var (
		err       error
		req       GetListRequest
		logList   []*common.TaskLog
		total     int64
		uid       string
		userID    primitive.ObjectID
		projectID primitive.ObjectID
		TaskID    primitive.ObjectID
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
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

	if TaskID, err = primitive.ObjectIDFromHex(req.TaskID); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if _, err = db.CheckProjectExist(projectID, userID); err != nil {
		request.APIError(c, err)
		return
	}

	if logList, err = db.GetLogList(projectID, TaskID, req.Page, req.Pagesize); err != nil {
		request.APIError(c, err)
		return
	}

	if total, err = db.GetLogTotal(projectID, TaskID); err != nil {
		request.APIError(c, err)
		return
	}

	request.APISuccess(c, &gin.H{
		"list":  utils.TernaryOperation(logList != nil, logList, []struct{}{}),
		"total": total,
	})
}

//type GetRecentLogCountRequest struct {
//	ProjectID string `form:"project_id" binding:"required"`
//}

type GetRecentLogCountResponse struct {
	SuccessCount int64  `json:"success_count"`
	ErrorCount   int64  `json:"error_count"`
	Date         string `json:"date"`
}

// 获取最近七天的任务执行情况
func GetRecentLogCount(c *gin.Context) {
	var (
		err        error
		uid        string
		userID     primitive.ObjectID
		projects   []*common.Project
		projectIDs []primitive.ObjectID
		result     []*GetRecentLogCountResponse
	)

	uid = c.GetString("jwt_user")

	if userID, err = primitive.ObjectIDFromHex(uid); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if projects, err = db.GetUserProjects(userID); err != nil {
		request.APIError(c, err)
		return
	}

	if projects == nil {
		goto EMPTY
	}

	for _, v := range projects {
		projectIDs = append(projectIDs, v.ProjectID)
	}

	for i := 6; i >= 0; i-- {
		var (
			successCount int64
			errorCount   int64
			timer        = utils.GetDateFromNow(-i)
		)

		if successCount, err = db.GetLogCountByDate(projectIDs, timer.Unix(), common.SuccessLog); err != nil {
			request.APIError(c, err)
			return
		}

		if errorCount, err = db.GetLogCountByDate(projectIDs, timer.Unix(), common.ErrorLog); err != nil {
			request.APIError(c, err)
			return
		}

		result = append(result, &GetRecentLogCountResponse{
			SuccessCount: successCount,
			ErrorCount:   errorCount,
			Date:         timer.Add(time.Duration(5) * time.Second).Format("2006-01-02"),
		})
	}

EMPTY:
	request.APISuccess(c, utils.TernaryOperation(len(result) != 0, result, []struct{}{}))
}
