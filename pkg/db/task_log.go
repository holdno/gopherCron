package db

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/mongodb/mongo-go-driver/mongo"

	"github.com/mongodb/mongo-go-driver/mongo/options"

	"github.com/mongodb/mongo-go-driver/bson"

	"ojbk.io/gopherCron/errors"

	"ojbk.io/gopherCron/common"
)

const (
	TaskLogTable = "task_log"
)

// CreateTaskLog 记录任务执行日志
func CreateTaskLog(data *common.TaskLog) error {
	var (
		err    error
		errObj errors.Error
	)

	if _, err = Database.Collection(TaskLogTable).InsertOne(context.TODO(), data); err != nil {
		logrus.WithFields(logrus.Fields{
			"Project": data.Project,
			"Name":    data.Name,
			"Command": data.Command,
			"Error":   err.Error(),
		}).Error("执行结果写入mongodb失败")
		errObj = errors.ErrInternalError
		errObj.Log = "[TaskLog - CreateTaskLog] InsertOne error:" + err.Error()
		return errObj
	}

	return nil
}

// GetLogList 获取日志列表
func GetLogList(project, name string, page, pagesize int64) ([]*common.TaskLog, error) {
	var (
		err      error
		errObj   errors.Error
		skip     int64
		findRes  mongo.Cursor
		taskList []*common.TaskLog
	)

	skip = (page - 1) * pagesize

	if findRes, err = Database.Collection(TaskLogTable).Find(context.TODO(), bson.M{"project": project, "name": name}, &options.FindOptions{
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
		task.ID = task.ObjID.Hex()
		taskList = append(taskList, &task)
	}

	return taskList, nil
}

// GetLogTotal 获取日志列表总数
func GetLogTotal(project, name string) (int64, error) {
	var (
		err    error
		errObj errors.Error
		total  int64
	)

	if total, err = Database.Collection(TaskLogTable).Count(context.TODO(), bson.M{"project": project, "name": name}); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[TaskLog - GetLogTotal] Count error:" + err.Error()
		return 0, errObj
	}

	return total, nil
}

// CleanLog 清除任务日志
func CleanLog(project, task string) error {
	var (
		ctx    context.Context
		err    error
		errObj errors.Error
	)

	ctx, _ = context.WithTimeout(context.TODO(), time.Duration(common.DBTimeout)*time.Second)
	if _, err = Database.Collection(TaskLogTable).DeleteMany(ctx, bson.M{"project": project, "name": task}); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[TaskLog - CleanLog] Delete error:" + err.Error()
		return errObj
	}

	return nil
}

// CleanProjectLog 清除项目下全部任务的日志
func CleanProjectLog(project string) error {
	var (
		ctx    context.Context
		err    error
		errObj errors.Error
	)

	ctx, _ = context.WithTimeout(context.TODO(), time.Duration(common.DBTimeout)*time.Second)
	if _, err = Database.Collection(TaskLogTable).DeleteMany(ctx, bson.M{"project": project}); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[TaskLog - CleanProjectLog] Delete error:" + err.Error()
		return errObj
	}

	return nil
}
