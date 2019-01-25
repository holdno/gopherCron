package project_func

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/mongodb/mongo-go-driver/bson/primitive"
	"ojbk.io/gopherCron/cmd/service/request"
	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/pkg/db"
	"ojbk.io/gopherCron/pkg/etcd"
	"ojbk.io/gopherCron/utils"
)

type GetUserProjectsResponse struct {
	ProjectID primitive.ObjectID `json:"project_id"`
	UID       primitive.ObjectID `json:"uid"`
	Title     string             `json:"title"`
	Remark    string             `json:"remark"`
	TaskCount int64              `json:"task_count"`
}

func GetUserProjects(c *gin.Context) {
	var (
		err   error
		list  []*common.Project
		uid   string
		objID primitive.ObjectID
		res   []*GetUserProjectsResponse
		count int64
	)

	uid = c.GetString("jwt_user")
	fmt.Println(uid)

	objID, err = primitive.ObjectIDFromHex(uid)
	if err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if list, err = db.GetUserProjects(objID); err != nil {
		request.APIError(c, err)
		return
	}

	// 获取所有项目的任务数
	for _, v := range list {
		count, err = etcd.Manager.GetProjectTaskCount(v.ProjectID.Hex())
		if err != nil {
			request.APIError(c, err)
			return
		}
		res = append(res, &GetUserProjectsResponse{
			ProjectID: v.ProjectID,
			UID:       v.UID,
			Title:     v.Title,
			Remark:    v.Remark,
			TaskCount: count,
		})
	}

	request.APISuccess(c, &gin.H{
		"list": utils.TernaryOperation(res != nil, res, []struct{}{}),
	})
}
