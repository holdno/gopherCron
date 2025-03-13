package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/config"
	"github.com/holdno/gopherCron/pkg/store"
	"github.com/holdno/gopherCron/pkg/store/sqlStore"

	"github.com/holdno/gocommons/selection"
	"github.com/jinzhu/gorm"
	"github.com/spacegrower/watermelon/infra/wlog"
	"go.uber.org/zap"
)

type ClientTaskReporter interface {
	ResultReport(result *common.TaskExecuteResult) error
}

type TaskResultReporter struct {
	logger       wlog.Logger
	taskLogStore store.TaskLogStore
	projectStore store.ProjectStore
}

func NewDefaultTaskReporter(logger wlog.Logger, mysqlConf *config.MysqlConf) ClientTaskReporter {
	_store := sqlStore.MustSetup(mysqlConf, logger, false)
	return &TaskResultReporter{
		logger:       logger,
		taskLogStore: _store.TaskLog(),
		projectStore: _store.Project(),
	}
}

func (r *TaskResultReporter) ResultReport(result *common.TaskExecuteResult) error {
	if result == nil {
		return errors.New("failed to report task result, empty result")
	}

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
	if projects, err = r.projectStore.GetProject(opts); err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to report task result, %w", err)
	}

	if len(projects) > 0 {
		projectInfo = projects[0]
	} else {
		r.logger.Error("task result report error, project not exist!", zap.Int64("project_id", result.ExecuteInfo.Task.ProjectID))
		return errors.New("task result report error, the project is not exist!")
	}

	taskResult = &common.TaskResultLog{
		Result: result.Output,
	}

	if result.Err != "" {
		taskResult.Error = result.Err
		getError = common.TASK_HAS_ERROR
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
		TmpID:     result.ExecuteInfo.TmpID,
	}

	if projectInfo != nil {
		logInfo.Project = projectInfo.Title
	}

	logInfo.ProjectID = result.ExecuteInfo.Task.ProjectID
	logInfo.TaskID = result.ExecuteInfo.Task.TaskID

	if err = r.taskLogStore.CreateOrUpdateTaskLog(nil, logInfo); err != nil {
		r.logger.With(zap.Any("fields", map[string]interface{}{
			"task_name":  logInfo.Name,
			"result":     logInfo.Result,
			"error":      err.Error(),
			"start_time": time.Unix(logInfo.StartTime, 0).Format("2006-01-02 15:05:05"),
			"end_time":   time.Unix(logInfo.StartTime, 0).Format("2006-01-02 15:05:05"),
		})).Error("任务日志入库失败")

		return fmt.Errorf("failed to save task result, %w", err)
	}
	return nil
}
