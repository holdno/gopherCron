package sqlStore

import (
	"fmt"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/pkg/store"

	"github.com/jinzhu/gorm"
)

type workflowSchedulePlanStore struct {
	commonFields
}

//NewWebHookStore
func NewWorkflowSchedulePlanStore(provider SqlProviderInterface) store.WorkflowSchedulePlanStore {
	repo := &workflowSchedulePlanStore{}

	repo.SetProvider(provider)
	repo.SetTable("gc_workflow_schedule_plan")
	return repo
}

func (s *workflowSchedulePlanStore) AutoMigrate() {
	if err := s.GetMaster().Table(s.GetTable()).AutoMigrate(&common.WorkflowSchedulePlan{}).Error; err != nil {
		panic(fmt.Errorf("unable to auto migrate %s, %w", s.GetTable(), err))
	}
	s.provider.Logger().Infof("%s, complete initialization", s.GetTable())
}

func (s *workflowSchedulePlanStore) Create(tx *gorm.DB, data *common.WorkflowSchedulePlan) error {
	data.BuildIndex()
	if tx == nil {
		tx = s.GetMaster()
	}
	return tx.Table(s.GetTable()).Create(data).Error
}

func (s *workflowSchedulePlanStore) GetList(workflowID int64) ([]common.WorkflowSchedulePlan, error) {
	var (
		err error
		res []common.WorkflowSchedulePlan
	)

	err = s.GetReplica().Table(s.GetTable()).Where("workflow_id = ?", workflowID).Find(&res).Error
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *workflowSchedulePlanStore) GetTaskWorkflowIDs(index []string) ([]common.WorkflowSchedulePlan, error) {
	var (
		err error
		res []common.WorkflowSchedulePlan
	)

	err = s.GetReplica().Table(s.GetTable()).Where("project_task_index IN(?)", index).Find(&res).Error
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (s *workflowSchedulePlanStore) GetTaskList(workflowID int64, taskID string) ([]common.WorkflowSchedulePlan, error) {
	var (
		err error
		res []common.WorkflowSchedulePlan
	)

	err = s.GetReplica().Table(s.GetTable()).Where("workflow_id = ? AND task_id = ?", workflowID, taskID).Find(&res).Error
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *workflowSchedulePlanStore) Delete(tx *gorm.DB, id int64) error {
	if tx == nil {
		tx = s.GetMaster()
	}

	return tx.Table(s.GetTable()).
		Where("id = ?", id).
		Delete(nil).Error
}

func (s *workflowSchedulePlanStore) DeleteAllWorkflowSchedulePlan(tx *gorm.DB, workflowID int64) error {
	if tx == nil {
		tx = s.GetMaster()
	}

	return tx.Table(s.GetTable()).
		Where("workflow_id = ?", workflowID).
		Delete(nil).Error
}

func (s *workflowSchedulePlanStore) DeleteList(tx *gorm.DB, ids []int64) error {
	if tx == nil {
		tx = s.GetMaster()
	}

	return tx.Table(s.GetTable()).
		Where("id IN (?)", ids).
		Delete(nil).Error
}
