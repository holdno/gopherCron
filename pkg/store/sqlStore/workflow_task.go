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
	s.provider.Logger().Info(fmt.Sprintf("%s, complete initialization", s.GetTable()))
}

func (s *workflowTaskStore) Create(tx *gorm.DB, data *common.WorkflowTask) error {
	if tx == nil {
		tx = s.GetMaster()
	}
	return tx.Table(s.GetTable()).Create(data).Error
}

func (s *workflowTaskStore) Save(tx *gorm.DB, data *common.WorkflowTask) error {
	if tx == nil {
		tx = s.GetMaster()
	}
	return tx.Table(s.GetTable()).Where("project_id = ? AND task_id = ?", data.ProjectID, data.TaskID).Debug().Save(data).Error
}

func (s *workflowTaskStore) GetList(projectID int64) ([]common.WorkflowTask, error) {
	var (
		err error
		res []common.WorkflowTask
	)

	err = s.GetReplica().Table(s.GetTable()).Where("project_id = ?", projectID).Order("task_id DESC").Find(&res).Error
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *workflowTaskStore) GetMultiList(taskIDs []string) ([]common.WorkflowTask, error) {
	var (
		err error
		res []common.WorkflowTask
	)

	err = s.GetReplica().Table(s.GetTable()).Where("task_id IN(?)", taskIDs).Order("task_id DESC").Find(&res).Error
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *workflowTaskStore) GetOne(projectID int64, taskID string) (*common.WorkflowTask, error) {
	var (
		err error
		res common.WorkflowTask
	)
	if err = s.GetReplica().Table(s.GetTable()).Where("project_id = ? AND task_id = ?", projectID, taskID).First(&res).Error; err != nil {
		return nil, err
	}
	return &res, nil
}

func (s *workflowTaskStore) Delete(tx *gorm.DB, projectID int64, taskID string) error {
	if tx == nil {
		tx = s.GetMaster()
	}
	err := tx.Table(s.GetTable()).Where("project_id = ? AND task_id = ?", projectID, taskID).Delete(nil).Error
	return err
}
