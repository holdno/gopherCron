package sqlStore

import (
	"fmt"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/pkg/store"

	"github.com/holdno/gocommons/selection"
	"github.com/jinzhu/gorm"
)

type projectStore struct {
	commonFields
}

//NewProjectStore
func NewProjectStore(provider SqlProviderInterface) store.ProjectStore {
	repo := &projectStore{}

	repo.SetProvider(provider)
	repo.SetTable("gc_project")
	return repo
}

func (s *projectStore) AutoMigrate() {
	if err := s.GetMaster().Table(s.GetTable()).AutoMigrate(&common.Project{}).Error; err != nil {
		panic(fmt.Errorf("unable to auto migrate %s, %w", s.GetTable(), err))
	}
	s.provider.Logger().Infof("%s, complete initialization", s.GetTable())
}

func (s *projectStore) CreateProject(tx *gorm.DB, obj common.Project) (int64, error) {
	if tx == nil {
		tx = s.GetMaster()
	}

	if err := tx.Table(s.table).Create(&obj).Error; err != nil {
		return 0, err
	}
	return obj.ID, nil
}

func (s *projectStore) UpdateProject(id int64, title, remark string) error {
	if err := s.GetMaster().Table(s.table).Where("id = ?", id).Updates(map[string]string{
		"title":  title,
		"remark": remark,
	}).Error; err != nil {
		return err
	}
	return nil
}

func (s *projectStore) GetProject(selector selection.Selector) ([]*common.Project, error) {
	var (
		err error
		res []*common.Project
	)

	db := parseSelector(s.GetReplica(), selector, true)

	if err = db.Table(s.GetTable()).Find(&res).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return res, nil
}

func (s *projectStore) DeleteProject(tx *gorm.DB, selector selection.Selector) error {
	if tx == nil {
		tx = s.GetMaster()
	}
	db := parseSelector(tx, selector, false)
	if err := db.Table(s.GetTable()).Delete(nil).Error; err != nil {
		return err
	}
	return nil
}

func (s *projectStore) UpdateRelation(projectID int64, relation string) error {
	if err := s.GetMaster().Table(s.GetTable()).Where("id = ?", projectID).Update("relation", relation).Error; err != nil {
		return err
	}
	return nil
}
