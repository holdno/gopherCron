package project_func

import (
	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/utils"

	"github.com/gin-gonic/gin"
)

type GetUserProjectsResponse struct {
	ProjectID int64  `json:"project_id"`
	UID       int64  `json:"uid"`
	Title     string `json:"title"`
	Remark    string `json:"remark"`
	TaskCount int64  `json:"task_count"`
}

func GetUserProjects(c *gin.Context) {
	var (
		err   error
		list  []*common.Project
		uid   = utils.GetUserID(c)
		res   []*GetUserProjectsResponse
		count int64
		srv   = app.GetApp(c)
	)

	if list, err = srv.GetUserProjects(uid); err != nil {
		response.APIError(c, err)
		return
	}

	// 获取所有项目的任务数
	for _, v := range list {
		count, err = srv.GetProjectTaskCount(v.ID)
		if err != nil {
			response.APIError(c, err)
			return
		}
		res = append(res, &GetUserProjectsResponse{
			ProjectID: v.ID,
			UID:       v.UID,
			Title:     v.Title,
			Remark:    v.Remark,
			TaskCount: count,
		})
	}

	response.APISuccess(c, &gin.H{
		"list": utils.TernaryOperation(res != nil, res, []struct{}{}),
	})
}
