package sqlStore

import (
	"fmt"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/pkg/store"

	"github.com/holdno/gocommons/selection"
	"github.com/jinzhu/gorm"
)

type temporaryTaskStore struct {
	commonFields
}

// NewProjectStore
func NewTemporaryTaskStoreStore(provider SqlProviderInterface) store.TemporaryTaskStore {
	repo := &temporaryTaskStore{}

	repo.SetProvider(provider)
	repo.SetTable("gc_temporary_task")
	return repo
}

func (s *temporaryTaskStore) AutoMigrate() {
	if err := s.GetMaster().Table(s.GetTable()).AutoMigrate(&common.TemporaryTask{}).Error; err != nil {
		panic(fmt.Errorf("unable to auto migrate %s, %w", s.GetTable(), err))
	}
	s.provider.Logger().Info(fmt.Sprintf("%s, complete initialization", s.GetTable()))
}

func (s *temporaryTaskStore) Create(data common.TemporaryTask) error {
	if err := s.GetMaster().Table(s.table).Create(&data).Error; err != nil {
		return err
	}
	return nil
}

func (s *temporaryTaskStore) UpdateTaskScheduleStatus(tx *gorm.DB, projectID int64, taskTmpID string, scheduleStatus int32) error {
	if tx == nil {
		tx = s.GetMaster()
	}
	return tx.Table(s.GetTable()).Where("project_id = ? AND tmp_id = ?", projectID, taskTmpID).
		Update("schedule_status", scheduleStatus).Error
}

func (s *temporaryTaskStore) Delete(tx *gorm.DB, id int64) error {
	if tx == nil {
		tx = s.GetMaster()
	}
	return tx.Table(s.GetTable()).Where("id = ?", id).Delete(nil).Error
}

func (s *temporaryTaskStore) GetList(selector selection.Selector) ([]*common.TemporaryTask, error) {
	var (
		err error
		res []*common.TemporaryTask
	)

	db := parseSelector(s.GetReplica(), selector, true)

	if err = db.Table(s.GetTable()).Order("id DESC").Find(&res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

func (s *temporaryTaskStore) Clean(tx *gorm.DB, selector selection.Selector) error {
	if tx == nil {
		tx = s.GetMaster()
	}
	db := parseSelector(tx, selector, true)

	if err := db.Table(s.GetTable()).Delete(nil).Error; err != nil {
		return err
	}

	return nil
}
