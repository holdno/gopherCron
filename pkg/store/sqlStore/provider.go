package sqlStore

import (
	"fmt"
	"reflect"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/config"
	"github.com/holdno/gopherCron/pkg/store"
	"github.com/holdno/gopherCron/utils"

	"github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

type SqlProvider struct {
	master   *gorm.DB
	replicas []*gorm.DB
	stores   SqlProviderStores
	logger   *logrus.Logger
}

func (p *SqlProvider) Logger() *logrus.Logger {
	return p.logger
}

func (s *SqlProvider) GetMaster() *gorm.DB {
	return s.master
}

func (s *SqlProvider) GetReplica() *gorm.DB {
	return s.replicas[utils.Random(0, len(s.replicas)-1)]
}

func (s *SqlProvider) batchExecStoreFuncs(fname string) {
	val := reflect.ValueOf(s.stores)
	// get all stores
	num := val.NumField()
	for i := 0; i < num; i++ {
		// install database
		val.Field(i).MethodByName(fname).Call([]reflect.Value{})
	}
}

// Install a function to init database tables
func (s *SqlProvider) Install() {
	s.batchExecStoreFuncs("AutoMigrate")
	s.Logger().Info("All stores are installed!")
}

// CheckStores check provider's stores are inited
func (s *SqlProvider) CheckStores() {
	s.batchExecStoreFuncs("CheckSelf")
	s.Logger().Info("All stores are ready!")
}

type SqlProviderStores struct {
	User             store.UserStore
	Project          store.ProjectStore
	ProjectRelevance store.ProjectRelevanceStore
	TaskLog          store.TaskLogStore
}

func MustSetup(conf *config.MysqlConf, logger *logrus.Logger, install bool) SqlStore {
	provider := new(SqlProvider)

	provider.logger = logger
	if err := provider.initConnect(conf); err != nil {
		panic(err)
	}

	provider.stores.User = NewUserStore(provider)
	provider.stores.Project = NewProjectStore(provider)
	provider.stores.TaskLog = NewTaskLogStore(provider)
	provider.stores.ProjectRelevance = NewProjectRelevanceStore(provider)
	provider.CheckStores()

	if install {
		provider.logger.Info("start install database ...")
		provider.Install()
		provider.logger.Info("finish")
	}

	// 检测是否需要创建管理员
	admin, err := provider.stores.User.GetAdminUser()
	if err != nil && err != gorm.ErrRecordNotFound {
		panic(err)
	}
	if admin == nil {
		provider.logger.Info("start create admin user")
		if err = provider.stores.User.CreateAdminUser(); err != nil {
			panic(err)
		}
		provider.logger.WithFields(logrus.Fields{
			"account":  common.ADMIN_USER_ACCOUNT,
			"password": common.ADMIN_USER_PASSWORD,
		}).Info("admin user created")
	}
	return provider
}

func (s *SqlProvider) User() store.UserStore {
	return s.stores.User
}

func (s *SqlProvider) Project() store.ProjectStore {
	return s.stores.Project
}

func (s *SqlProvider) ProjectRelevance() store.ProjectRelevanceStore {
	return s.stores.ProjectRelevance
}

func (s *SqlProvider) TaskLog() store.TaskLogStore {
	return s.stores.TaskLog
}

func (s *SqlProvider) BeginTx() *gorm.DB {
	return s.GetMaster().Begin()
}

// Shutdown close all connect
func (s *SqlProvider) Shutdown() {
	s.master.Close()
	for _, v := range s.replicas {
		v.Close()
	}
}

func (s *SqlProvider) initConnect(conf *config.MysqlConf) error {
	var (
		err    error
		mc     mysql.Config
		engine *gorm.DB
	)
	mc = mysql.Config{
		User:                 conf.Username,
		Passwd:               conf.Password,
		Net:                  "tcp",
		Addr:                 conf.Service,
		DBName:               conf.Database,
		AllowNativePasswords: true,
	}
	if engine, err = gorm.Open("mysql", mc.FormatDSN()); err != nil {
		return fmt.Errorf("failed to create seller, %w", err)
	}
	if err = engine.DB().Ping(); err != nil {
		return fmt.Errorf("connect to database, but ping was failed, %w", err)
	}

	s.master = engine
	s.replicas = append(s.replicas, engine)

	return nil
}

type SqlStore interface {
	User() store.UserStore
	Project() store.ProjectStore
	ProjectRelevance() store.ProjectRelevanceStore
	TaskLog() store.TaskLogStore
	BeginTx() *gorm.DB
	Install()
	Shutdown()
}
