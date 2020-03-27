package sqlStore

import (
	"fmt"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/pkg/store"

	"github.com/holdno/gocommons/selection"
	"github.com/jinzhu/gorm"
)

type projectRelevanceStore struct {
	commonFields
}

//NewProjectStore
func NewProjectRelevanceStore(provider SqlProviderInterface) store.ProjectRelevanceStore {
	repo := &projectRelevanceStore{}

	repo.SetProvider(provider)
	repo.SetTable("gc_project_relevance")
	return repo
}

func (s *projectRelevanceStore) AutoMigrate() {
	if err := s.GetMaster().Table(s.GetTable()).AutoMigrate(&common.ProjectRelevance{}).Error; err != nil {
		panic(fmt.Errorf("unable to auto migrate %s, %w", s.GetTable(), err))
	}
	s.provider.Logger().Infof("%s, complete initialization", s.GetTable())
}

func (s *projectRelevanceStore) Create(tx *gorm.DB, r common.ProjectRelevance) error {
	if tx == nil {
		tx = s.GetMaster()
	}

	if err := tx.Table(s.GetTable()).Create(&r).Error; err != nil {
		return err
	}

	return nil
}

func (s *projectRelevanceStore) Delete(tx *gorm.DB, pid, uid int64) error {
	if tx == nil {
		tx = s.GetMaster()
	}

	if err := tx.Table(s.GetTable()).Where("project_id = ? AND uid = ?", pid, uid).Delete(nil).Error; err != nil {
		return err
	}

	return nil
}

func (s *projectRelevanceStore) GetList(selector selection.Selector) ([]*common.ProjectRelevance, error) {
	var (
		err error
		res []*common.ProjectRelevance
	)

	db := parseSelector(s.GetReplica(), selector, true)

	if err = db.Table(s.GetTable()).Find(&res).Error; err != nil {
		return nil, err
	}

	return res, nil
}
