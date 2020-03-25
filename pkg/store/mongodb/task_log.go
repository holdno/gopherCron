package mongodb

import (
	"context"

	"github.com/mongodb/mongo-go-driver/bson/primitive"

	"github.com/holdno/gopherCron/utils"

	"github.com/sirupsen/logrus"

	"github.com/mongodb/mongo-go-driver/mongo"

	"github.com/mongodb/mongo-go-driver/mongo/options"

	"github.com/mongodb/mongo-go-driver/bson"

	"github.com/holdno/gopherCron/errors"

	"github.com/holdno/gopherCron/common"
)

const (
	TaskLogTable = "task_log"
)

// CreateTaskLog 记录任务执行日志
func CreateTaskLog(data *common.TaskLog) error {
	var (
		err    error
		errObj errors.Error
		ctx    context.Context
	)

	ctx, _ = utils.GetContextWithTimeout()

	if data.ID.IsZero() {
		data.ID = primitive.NewObjectID()
	}

	if _, err = Database.Collection(TaskLogTable).InsertOne(ctx, data); err != nil {
		logrus.WithFields(logrus.Fields{
			"ProjectID": data.ProjectID.Hex(),
			"TaskID":    data.TaskID.Hex(),
			"Project":   data.Project,
			"Name":      data.Name,
			"Command":   data.Command,
			"Error":     err.Error(),
		}).Error("执行结果写入mongodb失败")
		errObj = errors.ErrInternalError
		errObj.Log = "[TaskLog - CreateTaskLog] InsertOne error:" + err.Error()
		return errObj
	}

	return nil
}

// GetLogCountByDate 获取某天的日志数
func GetLogCountByDate(projects []primitive.ObjectID, timestamp int64, errType int) (int64, error) {
	var (
		err    error
		errObj errors.Error
		ctx    context.Context
		count  int64
	)

	ctx, _ = utils.GetContextWithTimeout()
	if count, err = Database.Collection(TaskLogTable).Count(ctx,
		bson.M{
			"project_id": bson.M{
				"$in": projects,
			},
			"start_time": bson.M{
				"$gte": timestamp,
				"$lt":  timestamp + 86400,
			},
			"with_error": errType}); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[TaskLog - GetLogCountByDate] count error:" + err.Error()
		return 0, errObj
	}

	return count, err
}

// GetLogList 获取日志列表
func GetLogList(projectID, taskID primitive.ObjectID, page, pagesize int64) ([]*common.TaskLog, error) {
	var (
		err      error
		errObj   errors.Error
		skip     int64
		findRes  mongo.Cursor
		taskList []*common.TaskLog
		ctx      context.Context
	)

	ctx, _ = utils.GetContextWithTimeout()

	skip = (page - 1) * pagesize

	if findRes, err = Database.Collection(TaskLogTable).Find(ctx, bson.M{"project_id": projectID, "task_id": taskID}, &options.FindOptions{
		Skip:  &skip,
		Limit: &pagesize,
		Sort:  bson.M{"_id": -1},
	}); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		errObj = errors.ErrInternalError
		errObj.Log = "[TaskLog - GetLogList] Find error:" + err.Error()
		return nil, errObj
	}

	for findRes.Next(context.TODO()) {
		var task common.TaskLog
		if err = findRes.Decode(&task); err != nil {
			errObj = errors.ErrInternalError
			errObj.Log = "[TaskLog - GetLogList] Decode error:" + err.Error()
			return nil, errObj
		}
		taskList = append(taskList, &task)
	}

	return taskList, nil
}

// GetLogTotal 获取日志列表总数
func GetLogTotal(projectID, taskID primitive.ObjectID) (int64, error) {
	var (
		err    error
		errObj errors.Error
		total  int64
		ctx    context.Context
	)

	ctx, _ = utils.GetContextWithTimeout()

	if total, err = Database.Collection(TaskLogTable).Count(ctx, bson.M{"project_id": projectID, "task_id": taskID}); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[TaskLog - GetLogTotal] Count error:" + err.Error()
		return 0, errObj
	}

	return total, nil
}

// CleanLog 清除任务日志
func CleanLog(projectID, taskID primitive.ObjectID) error {
	var (
		ctx    context.Context
		err    error
		errObj errors.Error
	)

	ctx, _ = utils.GetContextWithTimeout()
	if _, err = Database.Collection(TaskLogTable).DeleteMany(ctx, bson.M{"project_id": projectID, "task_id": taskID}); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[TaskLog - CleanLog] Delete error:" + err.Error()
		return errObj
	}

	return nil
}

// CleanProjectLog 清除项目下全部任务的日志
func CleanProjectLog(projectID primitive.ObjectID) error {
	var (
		ctx    context.Context
		err    error
		errObj errors.Error
	)

	ctx, _ = utils.GetContextWithTimeout()
	if _, err = Database.Collection(TaskLogTable).DeleteMany(ctx, bson.M{"project_id": projectID}); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[TaskLog - CleanProjectLog] Delete error:" + err.Error()
		return errObj
	}

	return nil
}
