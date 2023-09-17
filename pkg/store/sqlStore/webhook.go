package sqlStore

import (
	"fmt"

	"github.com/jinzhu/gorm"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/pkg/store"
)

type webHookStore struct {
	commonFields
}

// NewWebHookStore
func NewWebHookStore(provider SqlProviderInterface) store.TaskWebHookStore {
	repo := &webHookStore{}

	repo.SetProvider(provider)
	repo.SetTable("gc_webhook")
	return repo
}

func (s *webHookStore) AutoMigrate() {
	if err := s.GetMaster().Table(s.GetTable()).AutoMigrate(&common.WebHook{}).Error; err != nil {
		panic(fmt.Errorf("unable to auto migrate %s, %w", s.GetTable(), err))
	}
	s.provider.Logger().Info(fmt.Sprintf("%s, complete initialization", s.GetTable()))
}

func (s *webHookStore) Create(data common.WebHook) error {
	return s.GetMaster().Table(s.GetTable()).Create(data).Error
}

func (s *webHookStore) GetOne(projectID int64, types string) (*common.WebHook, error) {
	var (
		err error
		res common.WebHook
	)
	err = s.GetReplica().Table(s.GetTable()).
		Where("project_id = ?", projectID).
		Where("`type` = ?", types).First(&res).Error

	if err != nil {
		return nil, err
	}

	return &res, nil
}

func (s *webHookStore) GetList(projectID int64) ([]*common.WebHook, error) {
	var (
		err error
		res []*common.WebHook
	)

	err = s.GetReplica().Table(s.GetTable()).Where("project_id = ?", projectID).Find(&res).Error
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *webHookStore) Delete(tx *gorm.DB, projectID int64, types string) error {
	if tx == nil {
		tx = s.GetMaster()
	}

	return tx.Table(s.GetTable()).
		Where("project_id = ?", projectID).
		Where("`type` = ?", types).
		Delete(nil).Error
}

func (s *webHookStore) DeleteAll(tx *gorm.DB, projectID int64) error {
	if tx == nil {
		tx = s.GetMaster()
	}

	return tx.Table(s.GetTable()).
		Where("project_id = ?", projectID).
		Delete(nil).Error
}
