package sqlStore

import (
	"fmt"

	"github.com/holdno/gocommons/selection"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/pkg/store"
	"github.com/jinzhu/gorm"
)

type workflowStore struct {
	commonFields
}

//NewWebHookStore
func NewWorkflowStore(provider SqlProviderInterface) store.WorkflowStore {
	repo := &workflowStore{}

	repo.SetProvider(provider)
	repo.SetTable("gc_workflow")
	return repo
}

func (s *workflowStore) AutoMigrate() {
	if err := s.GetMaster().Table(s.GetTable()).AutoMigrate(&common.Workflow{}).Error; err != nil {
		panic(fmt.Errorf("unable to auto migrate %s, %w", s.GetTable(), err))
	}
	s.provider.Logger().Infof("%s, complete initialization", s.GetTable())
}

func (s *workflowStore) Create(tx *gorm.DB, data *common.Workflow) error {
	if tx == nil {
		tx = s.GetMaster()
	}
	return tx.Table(s.GetTable()).Create(data).Error
}

func (s *workflowStore) Update(tx *gorm.DB, data common.Workflow) error {
	if tx == nil {
		tx = s.GetMaster()
	}
	return tx.Table(s.GetTable()).Update(data).Error
}

func (s *workflowStore) GetOne(id int64) (*common.Workflow, error) {
	var (
		err error
		res common.Workflow
	)
	err = s.GetReplica().Table(s.GetTable()).
		Where("id = ?", id).First(&res).Error

	if err != nil {
		return nil, err
	}

	return &res, nil
}

func (s *workflowStore) GetList(selector selection.Selector, page, pagesize uint64) ([]common.Workflow, error) {
	var (
		err error
		res []common.Workflow
	)

	db := parseSelector(s.GetReplica(), selector, true)

	err = db.Table(s.GetTable()).
		Offset((page - 1) * pagesize).Limit(pagesize).Order("id DESC").Find(&res).Error
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *workflowStore) Delete(tx *gorm.DB, id int64) error {
	if tx == nil {
		tx = s.GetMaster()
	}

	return tx.Table(s.GetTable()).
		Where("id = ?", id).
		Delete(nil).Error
}
