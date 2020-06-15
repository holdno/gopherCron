package sqlStore

import (
	"fmt"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/pkg/store"
	"github.com/holdno/gopherCron/utils"

	"github.com/holdno/gocommons/selection"
)

type userStore struct {
	commonFields
}

//NewProjectStore
func NewUserStore(provider SqlProviderInterface) store.UserStore {
	repo := &userStore{}

	repo.SetProvider(provider)
	repo.SetTable("gc_user")
	return repo
}

func (s *userStore) AutoMigrate() {
	if err := s.GetMaster().Table(s.GetTable()).AutoMigrate(&common.User{}).Error; err != nil {
		panic(fmt.Errorf("unable to auto migrate %s, %w", s.GetTable(), err))
	}
	s.provider.Logger().Infof("%s, complete initialization", s.GetTable())
}

func (s *userStore) DeleteUser(id int64) error {
	if err := s.GetMaster().Table(s.GetTable()).Where("id = ?", id).Delete(nil).Error; err != nil {
		return err
	}
	return nil
}

func (s *userStore) CreateAdminUser() error {
	var (
		salt string
		err  error
	)
	salt = utils.RandomStr(6)

	user := &common.User{
		Account:    common.ADMIN_USER_ACCOUNT,
		Password:   utils.BuildPassword(common.ADMIN_USER_PASSWORD, salt),
		Salt:       salt,
		Name:       common.ADMIN_USER_NAME,
		Permission: common.ADMIN_USER_PERMISSION,
		CreateTime: time.Now().Unix(),
	}

	if err = s.GetMaster().Table(s.GetTable()).Create(user).Error; err != nil {
		return err
	}

	return nil
}

func (s *userStore) GetAdminUser() (*common.User, error) {
	var (
		err error
		res common.User
	)
	if err = s.GetReplica().Table(s.GetTable()).Where("account = ?", common.ADMIN_USER_ACCOUNT).First(&res).Error; err != nil {
		return nil, err
	}

	return &res, nil
}

// CreateUser 创建新用户
func (s *userStore) CreateUser(user common.User) error {
	if err := s.GetMaster().Table(s.GetTable()).Create(&user).Error; err != nil {
		return err
	}

	return nil
}

func (s *userStore) ChangePassword(uid int64, password, salt string) error {
	if err := s.GetMaster().Table(s.GetTable()).Where("id = ?", uid).Updates(map[string]string{
		"password": password,
		"salt":     salt,
	}).Error; err != nil {
		return err
	}

	return nil
}

func (s *userStore) GetUsers(selector selection.Selector) ([]*common.User, error) {
	var (
		err error
		res []*common.User
	)

	db := parseSelector(s.GetReplica(), selector, true)

	if err = db.Table(s.GetTable()).Find(&res).Error; err != nil {
		return nil, err
	}

	return res, nil
}
