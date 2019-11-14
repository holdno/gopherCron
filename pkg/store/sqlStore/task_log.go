package sqlStore

import (
	"fmt"
	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/pkg/selection"
	"ojbk.io/gopherCron/pkg/store"
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

func (s *taskLogStore) Clean(selector selection.Selector) error {
	db := parseSelector(s.GetMaster(), selector, true)

	if err := db.Table(s.GetTable()).Delete(nil).Error; err != nil {
		return err
	}

	return nil
}
