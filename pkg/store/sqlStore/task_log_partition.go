package sqlStore

import (
	"fmt"
	"time"

	"github.com/holdno/gocommons/selection"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/pkg/store"
	"github.com/holdno/gopherCron/utils"

	"github.com/jinzhu/gorm"
)

type taskLogPartitionStore struct {
	commonFields
}

func NewTaskLogPartitionStore(provider SqlProviderInterface) store.TaskLogStore {
	repo := &taskLogPartitionStore{}
	repo.SetProvider(provider)
	repo.SetTable("gc_task_log_p")
	return repo
}

func (s *taskLogPartitionStore) AutoMigrate() {
	// 创建主表结构
	if err := s.GetMaster().Exec(`
		CREATE TABLE IF NOT EXISTS gc_task_log_p (
			id BIGINT AUTO_INCREMENT,
			project_id BIGINT NOT NULL,
			project VARCHAR(128) NOT NULL,
			task_id VARCHAR(64) NOT NULL,
			tmp_id VARCHAR(64) NOT NULL DEFAULT '',
			name VARCHAR(128) NOT NULL,
			command TEXT NOT NULL,
			plan_time BIGINT NOT NULL,
			start_time BIGINT NOT NULL,
			end_time BIGINT NOT NULL DEFAULT 0,
			result TEXT,
			with_error TINYINT(1) NOT NULL DEFAULT 0,
			agent_version VARCHAR(128) NOT NULL DEFAULT '',
			client_ip VARCHAR(64) NOT NULL DEFAULT '',
			create_time BIGINT NOT NULL,
			PRIMARY KEY (id, plan_time),
			UNIQUE KEY uk_project_task_tmp (plan_time, project_id, task_id, tmp_id)
		) PARTITION BY RANGE (plan_time) (
			PARTITION p_default VALUES LESS THAN (0)
		)
	`).Error; err != nil {
		panic(fmt.Errorf("failed to create partition table: %w", err))
	}

	// 初始化未来7天的分区
	for i := 0; i < 7; i++ {
		day := time.Now().AddDate(0, 0, i)
		s.EnsurePartition(day)
	}

	s.provider.Logger().Info(fmt.Sprintf("%s, complete initialization", s.GetTable()))
}

// EnsurePartition 确保指定日期的分区存在
func (s *taskLogPartitionStore) EnsurePartition(day time.Time) error {
	partitionName := "p_" + day.Format("20060102")
	startUnix := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.Local).Unix()
	endUnix := startUnix + 86400 // 24小时

	// 先检查分区是否已存在
	var count int
	err := s.GetMaster().Raw(`
		SELECT COUNT(*) FROM information_schema.PARTITIONS 
		WHERE TABLE_NAME = 'gc_task_log_p' AND PARTITION_NAME = ?
	`, partitionName).Row().Scan(&count)
	if err != nil {
		return err
	}

	// 分区不存在则创建
	if count == 0 {
		return s.GetMaster().Exec(fmt.Sprintf(`
			ALTER TABLE gc_task_log_p ADD PARTITION (
				PARTITION %s VALUES LESS THAN (%d)
			)`, partitionName, endUnix)).Error
	}

	return nil
}

// cleanOldPartitions 清理7天前的旧分区
func (s *taskLogPartitionStore) CleanOldPartitions() error {
	st, _ := utils.GetLast7DaysTimeRange()
	oldestDay := st.AddDate(0, 0, -1)
	partitionName := "p_" + oldestDay.Format("20060102")

	return s.GetMaster().Exec(fmt.Sprintf(`
		ALTER TABLE gc_task_log_p DROP PARTITION %s
	`, partitionName)).Error
}

func (s *taskLogPartitionStore) CreateOrUpdateTaskLog(tx *gorm.DB, data common.TaskLog) error {
	if data.CreateTime == 0 {
		data.CreateTime = time.Now().Unix()
	}

	if tx == nil {
		tx = s.GetMaster()
	}

	// 使用原生SQL实现ON DUPLICATE KEY UPDATE
	return tx.Exec(`
		INSERT INTO gc_task_log_p (
			project_id, project, task_id, tmp_id, name, command, 
			plan_time, start_time, end_time, result, 
			with_error, agent_version, client_ip, create_time
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			end_time = VALUES(end_time),
			result = VALUES(result),
			with_error = VALUES(with_error),
			project = VALUES(project)
	`,
		data.ProjectID, data.Project, data.TaskID, data.TmpID, data.Name, data.Command,
		data.PlanTime, data.StartTime, data.EndTime, data.Result,
		data.WithError, data.AgentVersion, data.ClientIP, data.CreateTime,
	).Error
}

func (s *taskLogPartitionStore) GetList(selector selection.Selector) ([]*common.TaskLog, error) {
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

func (s *taskLogPartitionStore) GetOne(projectID int64, taskID, tmpID string) (*common.TaskLog, error) {
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

func (s *taskLogPartitionStore) CheckOrCreateScheduleLog(tx *gorm.DB, taskInfo *common.TaskExecutingInfo, agentIP, agentVersion string) (bool, error) {
	if tx == nil {
		tx = s.GetMaster()
	}
	var exist common.ExistResult
	if taskInfo.Task.Noseize == common.TASK_EXECUTE_NOSEIZE {
		err := tx.
			Raw("SELECT EXISTS(SELECT 1 FROM gc_task_log_p WHERE plan_time = ? AND  project_id = ? AND task_id = ? AND tmp_id = ?) AS result",
				taskInfo.PlanTime.Unix(), taskInfo.Task.ProjectID, taskInfo.Task.TaskID, taskInfo.TmpID).
			Scan(&exist).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return false, err
		}
	} else {
		err := tx.
			Raw("SELECT EXISTS(SELECT 1 FROM gc_task_log_p WHERE plan_time = ? AND project_id = ? AND task_id = ?) AS result",
				taskInfo.PlanTime.Unix(), taskInfo.Task.ProjectID, taskInfo.Task.TaskID).
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
		CreateTime:   time.Now().Unix(),
	}).Error
}

func (s *taskLogPartitionStore) LoadRunningTasks(tx *gorm.DB, before, after time.Time) ([]*common.TaskLog, error) {
	var (
		err error
		res []*common.TaskLog
	)

	if tx == nil {
		tx = s.GetReplica()
	}

	if err = tx.Table(s.GetTable()).Where("end_time = 0 AND plan_time > ? AND plan_time < ?", after.Unix(), before.Unix()).Find(&res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

func (s *taskLogPartitionStore) Clean(tx *gorm.DB, selector selection.Selector) error {
	if tx == nil {
		tx = s.GetMaster()
	}
	db := parseSelector(tx, selector, true)

	if err := db.Table(s.GetTable()).Delete(nil).Error; err != nil {
		return err
	}

	return nil
}
