package sqlStore

import (
	"fmt"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/pkg/store"

	"github.com/jinzhu/gorm"
)

type agentActivityStore struct {
	commonFields
}

// NewAgentActivityStore
func NewAgentActivityStore(provider SqlProviderInterface) store.AgentActivityStore {
	repo := &agentActivityStore{}

	repo.SetProvider(provider)
	repo.SetTable("gc_agent_activity")
	return repo
}

func (s *agentActivityStore) AutoMigrate() {
	if err := s.GetMaster().Table(s.GetTable()).AutoMigrate(&common.AgentActivity{}).Error; err != nil {
		panic(fmt.Errorf("unable to auto migrate %s, %w", s.GetTable(), err))
	}
	if err := s.GetMaster().Table(s.GetTable()).AddUniqueIndex("idx_client_ip_project_id", "client_ip", "project_id").Error; err != nil {
		panic(fmt.Errorf("failed to create unique index for %s, %w", s.GetTable(), err))
	}
	s.provider.Logger().Info(fmt.Sprintf("%s, complete initialization", s.GetTable()))
}

func (s *agentActivityStore) Create(tx *gorm.DB, obj common.AgentActivity) error {
	if tx == nil {
		tx = s.GetMaster()
	}

	err := tx.Exec("INSERT INTO "+s.GetTable()+" (client_ip,project_id,active_time) VALUES (?,?,?) ON DUPLICATE KEY UPDATE active_time = ?",
		obj.ClientIP, obj.ProjectID, obj.ActiveTime, obj.ActiveTime).Error
	if err != nil {
		return err
	}
	return nil
}

func (s *agentActivityStore) DeleteBefore(tx *gorm.DB, activeTime time.Time) error {
	if tx == nil {
		tx = s.GetMaster()
	}
	if err := tx.Table(s.GetTable()).Where("active_time < ?", activeTime.Unix()).Delete(nil).Error; err != nil {
		return err
	}
	return nil
}

func (s *agentActivityStore) GetOne(projectID int64, clientIP string) (*common.AgentActivity, error) {
	var (
		err error
		res common.AgentActivity
	)
	err = s.GetReplica().Table(s.GetTable()).
		Where("client_ip = ? AND project_id = ?", clientIP, projectID).
		First(&res).Error

	if err != nil {
		return nil, err
	}

	return &res, nil
}
