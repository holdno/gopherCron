package project_func

import (
	"github.com/gin-gonic/gin"
	"github.com/mongodb/mongo-go-driver/bson/primitive"
	"ojbk.io/gopherCron/cmd/service/request"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/pkg/db"
	"ojbk.io/gopherCron/pkg/etcd"
	"ojbk.io/gopherCron/utils"
)

type DeleteOneRequest struct {
	ProjectID string `form:"project_id" binding:"required"`
}

func DeleteOne(c *gin.Context) {
	var (
		err       error
		req       DeleteOneRequest
		uid       string
		userID    primitive.ObjectID
		projectID primitive.ObjectID
	)

	uid = c.GetString("jwt_user")

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if userID, err = primitive.ObjectIDFromHex(uid); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if projectID, err = primitive.ObjectIDFromHex(req.ProjectID); err != nil {
		request.APIError(c, errors.ErrInvalidArgument)
		return
	}

	if _, err = etcd.Manager.DeleteTask(req.ProjectID, ""); err != nil {
		request.APIError(c, err)
		return
	}

	if err = db.DeleteProject(projectID, userID); err != nil {
		request.APIError(c, err)
		return
	}

	request.APISuccess(c, nil)
}
