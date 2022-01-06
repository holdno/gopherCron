package sqlStore

import (
	"fmt"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/pkg/store"

	"github.com/holdno/gocommons/selection"
	"github.com/jinzhu/gorm"
)

type workflowLogStore struct {
	commonFields
}

//NewProjectStore
func NewWorkflowLogStore(provider SqlProviderInterface) store.WorkflowLogStore {
	repo := &workflowLogStore{}

	repo.SetProvider(provider)
	repo.SetTable("gc_workflow_log")
	return repo
}

func (s *workflowLogStore) AutoMigrate() {
	if err := s.GetMaster().Table(s.GetTable()).AutoMigrate(&common.WorkflowLog{}).Error; err != nil {
		panic(fmt.Errorf("unable to auto migrate %s, %w", s.GetTable(), err))
	}
	s.provider.Logger().Infof("%s, complete initialization", s.GetTable())
}

func (s *workflowLogStore) Create(tx *gorm.DB, data *common.WorkflowLog) error {
	if tx == nil {
		tx = s.GetMaster()
	}
	return tx.Table(s.GetTable()).Create(data).Error
}

func (s *workflowLogStore) GetList(selector selection.Selector, page, pagesize uint64) ([]common.WorkflowLog, error) {
	var (
		err error
		res []common.WorkflowLog
	)

	db := parseSelector(s.GetReplica(), selector, true)

	err = db.Table(s.GetTable()).
		Offset((page - 1) * pagesize).Limit(pagesize).Order("id DESC").Find(&res).Error
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *workflowLogStore) Clear(tx *gorm.DB, selector selection.Selector) error {
	if tx == nil {
		tx = s.GetMaster()
	}
	db := parseSelector(tx, selector, true)

	if err := db.Table(s.GetTable()).Delete(nil).Error; err != nil {
		return err
	}

	return nil
}
