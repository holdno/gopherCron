package sqlStore

import (
	"fmt"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/pkg/store"
	"github.com/jinzhu/gorm"
)

type workflowTaskStore struct {
	commonFields
}

//NewWebHookStore
func NewWorkflowTaskStore(provider SqlProviderInterface) store.WorkflowTaskStore {
	repo := &workflowTaskStore{}

	repo.SetProvider(provider)
	repo.SetTable("gc_workflow_task")
	return repo
}

func (s *workflowTaskStore) AutoMigrate() {
	if err := s.GetMaster().Table(s.GetTable()).AutoMigrate(&common.WorkflowTask{}).Error; err != nil {
		panic(fmt.Errorf("unable to auto migrate %s, %w", s.GetTable(), err))
	}
	s.provider.Logger().Infof("%s, complete initialization", s.GetTable())
}

func (s *workflowTaskStore) Create(tx *gorm.DB, data common.WorkflowTask) error {
	if tx == nil {
		tx = s.GetMaster()
	}
	return tx.Table(s.GetTable()).Create(data).Error
}

func (s *workflowTaskStore) GetList(workflowID int64) ([]common.WorkflowTask, error) {
	var (
		err error
		res []common.WorkflowTask
	)

	err = s.GetReplica().Table(s.GetTable()).Where("workflow_id = ?", workflowID).Group("task_id").Find(&res).Error
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *workflowTaskStore) GetTaskList(workflowID int64, taskID string) ([]common.WorkflowTask, error) {
	var (
		err error
		res []common.WorkflowTask
	)

	err = s.GetReplica().Table(s.GetTable()).Where("workflow_id = ? AND task_id = ?", workflowID, taskID).Find(&res).Error
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *workflowTaskStore) Delete(tx *gorm.DB, id int64) error {
	if tx == nil {
		tx = s.GetMaster()
	}

	return tx.Table(s.GetTable()).
		Where("id = ?", id).
		Delete(nil).Error
}

func (s *workflowTaskStore) DeleteList(tx *gorm.DB, ids []int64) error {
	if tx == nil {
		tx = s.GetMaster()
	}

	return tx.Table(s.GetTable()).
		Where("id IN (?)", ids).
		Delete(nil).Error
}
