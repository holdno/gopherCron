package app

import (
	"encoding/json"
	"time"

	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/config"
	"ojbk.io/gopherCron/pkg/selection"
	"ojbk.io/gopherCron/pkg/store"
	"ojbk.io/gopherCron/pkg/store/sqlStore"

	"github.com/sirupsen/logrus"
)

type ClientTaskReporter interface {
	ResultReport(result *common.TaskExecuteResult)
}

type TaskResultReporter struct {
	logger       *logrus.Logger
	taskLogStore store.TaskLogStore
	projectStore store.ProjectStore
}

func NewDefaultTaskReporter(logger *logrus.Logger, mysqlConf *config.MysqlConf) ClientTaskReporter {
	_store := sqlStore.MustSetup(mysqlConf, logger, false)
	return &TaskResultReporter{
		logger:       logger,
		taskLogStore: _store.TaskLog(),
		projectStore: _store.Project(),
	}
}

func (r *TaskResultReporter) ResultReport(result *common.TaskExecuteResult) {
	var (
		resultBytes    []byte
		projects       []*common.Project
		projectInfo    *common.Project
		err            error
		getError       int
		logInfo        common.TaskLog
		taskResult     *common.TaskResultLog
		jsonMarshalErr error
	)

	opts := selection.NewSelector(selection.NewRequirement("id", selection.Equals, result.ExecuteInfo.Task.ProjectID))
	projects, err = r.projectStore.GetProject(opts)

	if len(projects) > 0 {
		projectInfo = projects[0]
	} else {
		r.logger.WithField("project_id", result.ExecuteInfo.Task.ProjectID).Errorf("task result report error, project not exist!")
		return
	}

	taskResult = &common.TaskResultLog{
		Result: result.Output,
	}

	if err != nil {
		taskResult.Error = err.Error()
		getError = 1
	}

	if result.Err != nil {
		taskResult.SystemError = result.Err.Error()
		getError = 1
	}

	if resultBytes, jsonMarshalErr = json.Marshal(taskResult); jsonMarshalErr != nil {
		resultBytes = []byte("result log json marshal error:" + jsonMarshalErr.Error())
	}

	logInfo = common.TaskLog{
		Name:      result.ExecuteInfo.Task.Name,
		Result:    string(resultBytes),
		StartTime: result.StartTime.Unix(),
		EndTime:   result.EndTime.Unix(),
		Command:   result.ExecuteInfo.Task.Command,
		WithError: getError,
		ClientIP:  result.ExecuteInfo.Task.ClientIP,
	}

	if projectInfo != nil {
		logInfo.Project = projectInfo.Title
	}

	logInfo.ProjectID = result.ExecuteInfo.Task.ProjectID
	logInfo.TaskID = result.ExecuteInfo.Task.TaskID

	if err = r.taskLogStore.CreateTaskLog(logInfo); err != nil {
		r.logger.WithFields(logrus.Fields{
			"task_name":  logInfo.Name,
			"result":     logInfo.Result,
			"error":      err.Error(),
			"start_time": time.Unix(logInfo.StartTime, 0).Format("2006-01-02 15:05:05"),
			"end_time":   time.Unix(logInfo.StartTime, 0).Format("2006-01-02 15:05:05"),
		}).Error("任务日志入库失败")
	}
}
