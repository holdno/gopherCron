package db

import (
	"context"

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
