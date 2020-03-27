package sqlStore

import (
	"fmt"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/pkg/store"

	"github.com/holdno/gocommons/selection"
	"github.com/jinzhu/gorm"
)

type taskLogStore struct {
	commonFields
}

//NewProjectStore
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
	s.provider.Logger().Infof("%s, complete initialization", s.GetTable())
}

func (s *taskLogStore) CreateTaskLog(data common.TaskLog) error {
	if err := s.GetMaster().Table(s.table).Create(&data).Error; err != nil {
		return err
	}
	return nil
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
