package sqlStore

import (
	"fmt"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/pkg/store"

	"github.com/holdno/gocommons/selection"
	"github.com/jinzhu/gorm"
)

type taskLogStore struct {
	commonFields
}

// NewProjectStore
func NewTaskLogStore(provider SqlProviderInterface) store.TaskLogStore {
	repo := &taskLogStore{}

	repo.SetProvider(provider)
	repo.SetTable("gc_task_log")
	return repo
}

func (s *taskLogStore) AutoMigrate() {
	if err := s.GetMaster().Table(s.GetTable()).AutoMigrate(&common.TaskLog{}).Error; err != nil {
		panic(fmt.Errorf("unable to auto migrate %s, %w", s.GetTable(), err))
	}
	s.provider.Logger().Info(fmt.Sprintf("%s, complete initialization", s.GetTable()))
}

func (s *taskLogStore) CreateOrUpdateTaskLog(tx *gorm.DB, data common.TaskLog) error {
	var tmpLog common.TaskLog

	if data.TmpID != "" && data.PlanTime > 0 {
		err := s.GetMaster().Table(s.table).Where("project_id = ? AND task_id = ? AND tmp_id = ?",
			data.ProjectID, data.TaskID, data.TmpID).
			First(&tmpLog).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
	}

	if tx == nil {
		tx = s.GetMaster()
	}

	if tmpLog.TaskID == "" {
		return tx.Table(s.table).Create(&data).Error
	} else {
		data.PlanTime = tmpLog.PlanTime
		return tx.Table(s.table).Where("project_id = ? AND task_id = ? AND tmp_id = ? AND plan_time = ?",
			data.ProjectID, data.TaskID, data.TmpID, data.PlanTime).UpdateColumns(map[string]interface{}{
			"end_time":   data.EndTime,
			"result":     data.Result,
			"with_error": data.WithError,
		}).Error
	}
}

func (s *taskLogStore) GetList(selector selection.Selector) ([]*common.TaskLog, error) {
	var (
		err error
		res []*common.TaskLog
	)

	db := parseSelector(s.GetReplica(), selector, true)

	if err = db.Table(s.GetTable()).Find(&res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

func (s *taskLogStore) GetOne(projectID int64, taskID, tmpID string) (*common.TaskLog, error) {
	var (
		err error
		res common.TaskLog
	)

	err = s.GetReplica().Table(s.GetTable()).
		Where("project_id = ? AND task_id = ? AND tmp_id = ?", projectID, taskID, tmpID).First(&res).Error
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (s *taskLogStore) CheckOrCreateScheduleLog(tx *gorm.DB, taskInfo *common.TaskExecutingInfo, agentIP, agentVersion string) (bool, error) {
	if tx == nil {
		tx = s.GetMaster()
	}
	var exist common.ExistResult
	if taskInfo.Task.Noseize == common.TASK_EXECUTE_NOSEIZE {
		err := tx.
			Raw("SELECT EXISTS(SELECT 1 FROM gc_task_log WHERE project_id = ? AND task_id = ? AND tmp_id = ? AND plan_time = ?) AS result",
				taskInfo.Task.ProjectID, taskInfo.Task.TaskID, taskInfo.TmpID, taskInfo.PlanTime.Unix()).
			Scan(&exist).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return false, err
		}
	} else {
		err := tx.
			Raw("SELECT EXISTS(SELECT 1 FROM gc_task_log WHERE project_id = ? AND task_id = ? AND plan_time = ?) AS result",
				taskInfo.Task.ProjectID, taskInfo.Task.TaskID, taskInfo.PlanTime.Unix()).
			Scan(&exist).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return false, err
		}
	}

	if exist.Result {
		return false, nil
	}

	if taskInfo.RealTime.IsZero() {
		taskInfo.RealTime = time.Now()
	}

	return true, tx.Table(s.table).Create(&common.TaskLog{
		ProjectID:    taskInfo.Task.ProjectID,
		TaskID:       taskInfo.Task.TaskID,
		TmpID:        taskInfo.TmpID,
		PlanTime:     taskInfo.PlanTime.Unix(),
		Name:         taskInfo.Task.Name,
		Command:      taskInfo.Task.Command,
		AgentVersion: agentVersion,
		ClientIP:     agentIP,
		StartTime:    taskInfo.RealTime.Unix(),
	}).Error
}

func (s *taskLogStore) LoadRunningTasks(tx *gorm.DB, before, after time.Time) ([]*common.TaskLog, error) {
	var (
		err error
		res []*common.TaskLog
	)

	if tx == nil {
		tx = s.GetReplica()
	}

	if err = tx.Table(s.GetTable()).Where("end_time = 0 AND start_time > ? AND start_time < ?", after.Unix(), before.Unix()).Find(&res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

func (s *taskLogStore) Clean(tx *gorm.DB, selector selection.Selector) error {
	if tx == nil {
		tx = s.GetMaster()
	}
	db := parseSelector(tx, selector, true)

	if err := db.Table(s.GetTable()).Delete(nil).Error; err != nil {
		return err
	}

	return nil
}
