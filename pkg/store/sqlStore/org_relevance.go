package sqlStore

import (
	"fmt"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/pkg/store"

	"github.com/jinzhu/gorm"
)

type orgRelevanceStore struct {
	commonFields
}

// NewOrgStore
func NewOrgRelevanceStore(provider SqlProviderInterface) store.OrgRelevanceStore {
	repo := &orgRelevanceStore{}

	repo.SetProvider(provider)
	repo.SetTable("gc_org_relevance")
	return repo
}

func (s *orgRelevanceStore) AutoMigrate() {
	if err := s.GetMaster().Table(s.GetTable()).AutoMigrate(&common.OrgRelevance{}).Error; err != nil {
		panic(fmt.Errorf("unable to auto migrate %s, %w", s.GetTable(), err))
	}
	s.provider.Logger().Info(fmt.Sprintf("%s, complete initialization", s.GetTable()))
}

func (s *orgRelevanceStore) Create(tx *gorm.DB, obj common.OrgRelevance) error {
	if tx == nil {
		tx = s.GetMaster()
	}

	if err := tx.Table(s.table).Create(&obj).Error; err != nil {
		return err
	}
	return nil
}

func (s *orgRelevanceStore) ListUserOrg(uid int64) ([]*common.OrgRelevance, error) {
	var (
		err error
		res []*common.OrgRelevance
	)

	if err = s.GetReplica().Table(s.GetTable()).Where("uid = ?", uid).Find(&res).Error; err != nil {
		return nil, err
	}

	return res, nil
}

func (s *orgRelevanceStore) GetUserOrg(oid string, uid int64) (*common.OrgRelevance, error) {
	var (
		err error
		res common.OrgRelevance
	)

	if err = s.GetReplica().Table(s.GetTable()).Where("oid = ? AND uid = ?", oid, uid).First(&res).Error; err != nil {
		return nil, err
	}

	return &res, nil
}

func (s *orgRelevanceStore) Delete(tx *gorm.DB, oid string, uid int64) error {
	if tx == nil {
		tx = s.GetMaster()
	}
	if err := tx.Table(s.GetTable()).Where("oid = ? AND uid = ?", oid, uid).Delete(nil).Error; err != nil {
		return err
	}
	return nil
}
