package sqlStore

import (
	"fmt"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/pkg/store"
	"github.com/jinzhu/gorm"
)

type userWorkflowRelevanceStore struct {
	commonFields
}

//NewWebHookStore
func NewUserWorkflowRelevanceStore(provider SqlProviderInterface) store.UserWorkflowRelevanceStore {
	repo := &userWorkflowRelevanceStore{}

	repo.SetProvider(provider)
	repo.SetTable("gc_user_workflow")
	return repo
}

func (s *userWorkflowRelevanceStore) AutoMigrate() {
	if err := s.GetMaster().Table(s.GetTable()).AutoMigrate(&common.UserWorkflowRelevance{}).Error; err != nil {
		panic(fmt.Errorf("unable to auto migrate %s, %w", s.GetTable(), err))
	}
	s.provider.Logger().Infof("%s, complete initialization", s.GetTable())
}

func (s *userWorkflowRelevanceStore) Create(tx *gorm.DB, data *common.UserWorkflowRelevance) error {
	if tx == nil {
		tx = s.GetMaster()
	}
	return tx.Table(s.GetTable()).Create(&data).Error
}

func (s *userWorkflowRelevanceStore) GetUserWorkflows(userID int64) ([]common.UserWorkflowRelevance, error) {
	var list []common.UserWorkflowRelevance
	err := s.GetReplica().Table(s.GetTable()).Where("user_id = ?", userID).Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (s *userWorkflowRelevanceStore) GetWorkflowUsers(workflowID int64) ([]common.UserWorkflowRelevance, error) {
	var list []common.UserWorkflowRelevance
	err := s.GetReplica().Table(s.GetTable()).Where("workflow_id = ?", workflowID).Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (s *userWorkflowRelevanceStore) GetUserWorkflowRelevance(userID int64, workflowID int64) (*common.UserWorkflowRelevance, error) {
	var result common.UserWorkflowRelevance
	err := s.GetReplica().Table(s.GetTable()).Where("user_id = ? AND workflow_id = ?", userID, workflowID).First(&result).Error
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *userWorkflowRelevanceStore) DeleteWorkflowAllUserRelevance(tx *gorm.DB, workflowID int64) error {
	if tx == nil {
		tx = s.GetMaster()
	}
	return tx.Table(s.GetTable()).Where("workflow_id = ?", workflowID).Delete(nil).Error
}

func (s *userWorkflowRelevanceStore) DeleteUserWorkflowRelevance(tx *gorm.DB, workflowID, userID int64) error {
	if tx == nil {
		tx = s.GetMaster()
	}
	return tx.Table(s.GetTable()).Where("workflow_id = ? AND user_id = ?", workflowID, userID).Delete(nil).Error
}
