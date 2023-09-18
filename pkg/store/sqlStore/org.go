package sqlStore

import (
	"fmt"

	"github.com/holdno/gocommons/selection"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/pkg/store"

	"github.com/jinzhu/gorm"
)

type orgStore struct {
	commonFields
}

// NewOrgStore
func NewOrgStore(provider SqlProviderInterface) store.OrgStore {
	repo := &orgStore{}

	repo.SetProvider(provider)
	repo.SetTable("gc_org")
	return repo
}

func (s *orgStore) AutoMigrate() {
	if err := s.GetMaster().Table(s.GetTable()).AutoMigrate(&common.Org{}).Error; err != nil {
		panic(fmt.Errorf("unable to auto migrate %s, %w", s.GetTable(), err))
	}
	s.provider.Logger().Info(fmt.Sprintf("%s, complete initialization", s.GetTable()))
}

func (s *orgStore) CreateOrg(tx *gorm.DB, obj common.Org) error {
	if tx == nil {
		tx = s.GetMaster()
	}

	if err := tx.Table(s.table).Create(&obj).Error; err != nil {
		return err
	}
	return nil
}

func (s *orgStore) List(selector selection.Selector) ([]*common.Org, error) {
	var (
		err error
		res []*common.Org
	)

	db := parseSelector(s.GetReplica(), selector, true)

	if err = db.Table(s.GetTable()).Find(&res).Error; err != nil {
		return nil, err
	}

	return res, nil
}

func (s *orgStore) Delete(tx *gorm.DB, id string) error {
	if tx == nil {
		tx = s.GetMaster()
	}
	if err := tx.Table(s.GetTable()).Where("id = ? ", id).Delete(nil).Error; err != nil {
		return err
	}
	return nil
}
