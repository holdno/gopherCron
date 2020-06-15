package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/holdno/gopherCron/pkg/daemon"

	"github.com/holdno/gopherCron/pkg/panicgroup"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/config"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/jwt"
	"github.com/holdno/gopherCron/pkg/etcd"
	"github.com/holdno/gopherCron/pkg/logger"
	"github.com/holdno/gopherCron/pkg/store/sqlStore"
	"github.com/holdno/gopherCron/utils"

	"github.com/coreos/etcd/clientv3"
	"github.com/gin-gonic/gin"
	"github.com/holdno/gocommons/selection"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

type App interface {
	CreateProject(tx *gorm.DB, p common.Project) (int64, error)
	GetProject(pid int64) (*common.Project, error)
	GetUserProjects(uid int64) ([]*common.Project, error)
	CheckProjectExistByName(title string) (*common.Project, error)
	CheckUserIsInProject(pid, uid int64) (bool, error)        // 确认该用户是否加入该项目
	CheckUserProject(pid, uid int64) (*common.Project, error) // 确认项目是否属于该用户
	UpdateProject(pid int64, title, remark string) error
	DeleteProject(tx *gorm.DB, pid, uid int64) error
	SaveTask(task *common.TaskInfo, opts ...clientv3.OpOption) (*common.TaskInfo, error)
	DeleteTask(pid int64, tid string) (*common.TaskInfo, error)
	SetTaskRunning(task common.TaskInfo) error
	SetTaskNotRunning(task common.TaskInfo) error
	KillTask(pid int64, tid string) error
	IsAdmin(uid int64) (bool, error)
	GetWorkerList(projectID int64) ([]ClientInfo, error)
	CheckProjectWorkerExist(projectID int64, host string) (bool, error)
	ReloadWorkerConfig(host string) error
	GetProjectTaskCount(projectID int64) (int64, error)
	GetTaskList(projectID int64) ([]*common.TaskInfo, error)
	GetTask(projectID int64, nameID string) (*common.TaskInfo, error)
	GetMonitor(ip string) (*common.MonitorInfo, error)
	TemporarySchedulerTask(task *common.TaskInfo) error
	GetTaskLogList(pid int64, tid string, page, pagesize int) ([]*common.TaskLog, error)
	GetLogTotalByDate(projects []int64, timestamp int64, errType int) (int, error)
	GetTaskLogTotal(pid int64, tid string) (int, error)
	CleanProjectLog(tx *gorm.DB, pid int64) error
	CleanLog(tx *gorm.DB, pid int64, tid string) error
	DeleteAll() error
	CreateProjectRelevance(tx *gorm.DB, pid, uid int64) error
	DeleteProjectRelevance(tx *gorm.DB, pid, uid int64) error
	GetProjectRelevanceUsers(pid int64) ([]*common.ProjectRelevance, error)
	GetUserByAccount(account string) (*common.User, error)
	GetUserInfo(uid int64) (*common.User, error)
	GetUsersByIDs(uids []int64) ([]*common.User, error)
	CreateUser(u common.User) error
	DeleteUser(id int64) error
	GetUserList(args GetUserListArgs) ([]*common.User, error)
	GetUserListTotal(args GetUserListArgs) (int, error)
	ChangePassword(uid int64, password, salt string) error
	GetTaskLocker(task *common.TaskInfo) *etcd.Locker
	GetIP() string
	GetConfig() *config.ServiceConfig

	BeginTx() *gorm.DB
	Close()
	GetVersion() string
	Warner
}

type EtcdManager interface {
	Client() *clientv3.Client
	KV() clientv3.KV
	Lease() clientv3.Lease
	Watcher() clientv3.Watcher
	GetTaskLocker(task *common.TaskInfo) *etcd.Locker
	GetLocker(key string) *etcd.Locker
	Inc(key string) (int64, error)
}

func GetApp(c *gin.Context) App {
	return c.MustGet(common.APP_KEY).(App)
}

type app struct {
	store   sqlStore.SqlStore
	logger  *logrus.Logger
	etcd    EtcdManager
	closeCh chan struct{}
	isClose bool
	localip string

	cfg *config.ServiceConfig

	CommonInterface
	Warner
}

type AppOptions func(a *app)

func WithWarning(w Warner) AppOptions {
	return func(a *app) {
		a.Warner = w
	}
}

func NewApp(configPath string, opts ...AppOptions) App {
	var err error

	conf := config.InitServiceConfig(configPath)
	app := new(app)
	app.cfg = conf
	app.logger = logger.MustSetup(conf.LogLevel)
	app.store = sqlStore.MustSetup(conf.Mysql, app.logger, true)

	for _, opt := range opts {
		opt(app)
	}

	if app.Warner == nil {
		app.Warner = NewDefaultWarner(app.logger)
	}

	jwt.InitJWT(conf.JWT)

	app.logger.Info("start to connect etcd ...")
	if app.etcd, err = etcd.Connect(conf.Etcd); err != nil {
		panic(err)
	}
	app.logger.Info("connected to etcd")
	app.CommonInterface = NewComm(app.etcd)

	utils.InitIDWorker(1)

	// 自动清理任务
	go func() {
		t := time.NewTicker(time.Hour * 12)
		for {
			select {
			case <-t.C:
				app.AutoCleanLogs()
			case <-app.closeCh:
				t.Stop()
				// app.etcd.Lock(nil).CloseAll()
				return
			}
		}
	}()

	return app
}

func (a *app) GetConfig() *config.ServiceConfig {
	return a.cfg
}

func (a *app) GetIP() string {
	return a.localip
}

func (a *app) GetTaskLocker(task *common.TaskInfo) *etcd.Locker {
	return a.etcd.GetTaskLocker(task)
}

type client struct {
	localip    string
	logger     *logrus.Logger
	etcd       EtcdManager
	scheduler  *TaskScheduler
	configPath string
	cfg        *config.ServiceConfig

	daemon *daemon.ProjectDaemon

	isClose   bool
	closeChan chan struct{}

	panicgroup.PanicGroup
	ClientTaskReporter
	CommonInterface
	Warner
}

type Client interface {
	Go(f func())
	GetIP() string
	Loop()
	ResultReport(result *common.TaskExecuteResult) error
	Close()
	Warner
}

type ClientOptions func(a *client)

func ClientWithTaskReporter(reporter ClientTaskReporter) ClientOptions {
	return func(agent *client) {
		agent.ClientTaskReporter = reporter
	}
}

func ClientWithWarning(w Warner) ClientOptions {
	return func(a *client) {
		a.Warner = w
	}
}

func (agent *client) loadConfigAndSetupAgentFunc() func() {
	inited := false

	return func() {
		var err error
		cfg := config.InitServiceConfig(agent.configPath)
		if !inited {
			inited = true
			agent.cfg = &config.ServiceConfig{}
			if agent.configPath == "" {
				panic("empty config path")
			}

			if agent.etcd, err = etcd.Connect(cfg.Etcd); err != nil {
				panic(err)
			}

			agent.CommonInterface = NewComm(agent.etcd)

			clusterID, err := agent.etcd.Inc(cfg.Etcd.Prefix + common.CLUSTER_AUTO_INDEX)
			if err != nil {
				panic(err)
			}

			// why 1024. view https://github.com/holdno/snowFlakeByGo
			utils.InitIDWorker(clusterID % 1024)
			agent.logger = logger.MustSetup(cfg.LogLevel)
			agent.scheduler = initScheduler()
			agent.daemon = daemon.NewProjectDaemon(nil, agent.logger)
		} else if agent.configPath == "" {
			return
		}

		fmt.Println("--------------load config--------------")

		if cfg.ReportAddr != agent.cfg.ReportAddr {
			agent.logger.Infof("init http task log reporter, address: %s", cfg.ReportAddr)
			reporter := NewHttpReporter(cfg.ReportAddr)
			agent.ClientTaskReporter = reporter
			agent.Warner = reporter
		} else if agent.ClientTaskReporter == nil {
			agent.logger.Info("init default task log reporter, it must be used mysql config")
			agent.ClientTaskReporter = NewDefaultTaskReporter(agent.logger, cfg.Mysql)
		}

		if agent.Warner == nil {
			agent.Warner = NewDefaultWarner(agent.logger)
		}

		addProjects, _ := agent.daemon.DiffProjects(cfg.Etcd.Projects)
		agent.logger.WithField("projects", addProjects).Debug("diff projects")

		agent.TaskWatcher(addProjects)
		agent.TaskKiller(addProjects)
		agent.Register(addProjects)

		agent.cfg = cfg
	}

}

func NewClient(configPath string, opts ...ClientOptions) Client {
	var err error

	agent := &client{
		configPath: configPath,
		isClose:    false,
		closeChan:  make(chan struct{}),
	}
	agent.configPath = configPath
	setupFunc := agent.loadConfigAndSetupAgentFunc()

	if agent.localip, err = utils.GetLocalIP(); err != nil {
		agent.logger.Error("failed to get local ip")
	}

	for _, opt := range opts {
		opt(agent)
	}

	agent.PanicGroup = panicgroup.NewPanicGroup(func(err error) {
		reserr := agent.Warning(WarningData{
			Data:    err.Error(),
			Type:    WarningTypeSystem,
			AgentIP: agent.localip,
		})
		if reserr != nil {
			agent.logger.WithFields(logrus.Fields{
				"desc":         reserr,
				"source_error": err,
			}).Error("panicgroup: failed to warning panic error")
		}
	})

	setupFunc()

	agent.OnCommand(func(command string) {
		switch command {
		case common.AGENT_COMMAND_RELOAD_CONFIG:
			setupFunc()
		}
	})

	return agent
}

func (c *client) Close() {
	if !c.isClose {
		c.isClose = true
		c.daemon.Close()
		c.scheduler.Stop()
		<-c.closeChan
		fmt.Println("shut down")
	}
}

func (c *client) GetIP() string {
	return c.localip
}

func (a *app) Close() {
	if !a.isClose {
		a.isClose = true
		close(a.closeCh)
	}
}

func (a *app) BeginTx() *gorm.DB {
	return a.store.BeginTx()
}

func (a *app) CheckUserIsInProject(pid, uid int64) (bool, error) {
	opt := selection.NewSelector(selection.NewRequirement("project_id", selection.Equals, pid),
		selection.NewRequirement("uid", selection.FindIn, uid))
	opt.Select = "id"
	res, err := a.store.ProjectRelevance().GetMap(opt)
	if err != nil && err != gorm.ErrRecordNotFound {
		errObj := errors.ErrInternalError
		errObj.Msg = "获取项目归属信息失败"
		errObj.Log = err.Error()
		return false, errObj
	}
	if len(res) == 0 {
		return false, nil
	}

	return true, nil
}

func (a *app) CheckUserProject(pid, uid int64) (*common.Project, error) {
	opt := selection.NewSelector(selection.NewRequirement("id", selection.Equals, pid),
		selection.NewRequirement("uid", selection.Equals, uid))
	res, err := a.store.Project().GetProject(opt)
	if err != nil && err != gorm.ErrRecordNotFound {
		errObj := errors.ErrInternalError
		errObj.Msg = "获取项目信息失败"
		errObj.Log = err.Error()
		return nil, errObj
	}
	if len(res) == 0 {
		return nil, nil
	}

	return res[0], nil
}

func (a *app) GetProject(pid int64) (*common.Project, error) {
	opt := selection.NewSelector(selection.NewRequirement("id", selection.Equals, pid))
	opt.Pagesize = 1
	res, err := a.store.Project().GetProject(opt)
	if err != nil && err != gorm.ErrRecordNotFound {
		errObj := errors.ErrInternalError
		errObj.Msg = "无法获取项目信息"
		errObj.Log = err.Error()
		return nil, errObj
	}

	if len(res) == 0 {
		return nil, errors.ErrProjectNotExist
	}

	return res[0], nil
}

func (a *app) GetUserProjects(uid int64) ([]*common.Project, error) {
	opt := selection.NewSelector()
	isAdmin, err := a.IsAdmin(uid)
	if err != nil {
		return nil, err
	}
	if !isAdmin {
		opt.AddQuery(selection.NewRequirement("uid", selection.Equals, uid))
	}

	res, err := a.store.ProjectRelevance().GetList(opt)
	if err != nil {
		errObj := errors.ErrInternalError
		errObj.Msg = "无法获取用户关联产品信息"
		errObj.Log = err.Error()
		return nil, errObj
	}

	var pids []int64
	for _, v := range res {
		pids = append(pids, v.ProjectID)
	}

	opt = selection.NewSelector(selection.NewRequirement("id", selection.In, pids))
	projects, err := a.store.Project().GetProject(opt)
	if err != nil && err != gorm.ErrRecordNotFound {
		errObj := errors.ErrInternalError
		errObj.Msg = "无法获取项目信息"
		errObj.Log = err.Error()
		return nil, errObj
	}

	return projects, nil
}

func (a *app) CleanProjectLog(tx *gorm.DB, pid int64) error {
	opt := selection.NewSelector(selection.NewRequirement("project_id", selection.Equals, pid))
	if err := a.store.TaskLog().Clean(tx, opt); err != nil {
		errObj := errors.ErrInternalError
		errObj.Msg = "清除项目日志失败"
		errObj.Log = err.Error()
		return errObj
	}

	return nil
}

func (a *app) CleanLog(tx *gorm.DB, pid int64, tid string) error {
	opt := selection.NewSelector(selection.NewRequirement("project_id", selection.Equals, pid),
		selection.NewRequirement("task_id", selection.Equals, tid))
	if err := a.store.TaskLog().Clean(tx, opt); err != nil {
		errObj := errors.ErrInternalError
		errObj.Msg = "清除日志失败"
		errObj.Log = err.Error()
		return errObj
	}

	return nil
}

func (a *app) GetTaskLogList(pid int64, tid string, page, pagesize int) ([]*common.TaskLog, error) {
	opt := selection.NewSelector(selection.NewRequirement("project_id", selection.Equals, pid),
		selection.NewRequirement("task_id", selection.Equals, tid))
	opt.Page = page
	opt.Pagesize = pagesize
	opt.OrderBy = "id DESC"

	list, err := a.store.TaskLog().GetList(opt)
	if err != nil && err != gorm.ErrRecordNotFound {
		errObj := errors.ErrInternalError
		errObj.Msg = "获取日志列表失败"
		errObj.Log = err.Error()
		return nil, errObj
	}

	return list, nil
}

func (a *app) GetTaskLogTotal(pid int64, tid string) (int, error) {
	opt := selection.NewSelector(selection.NewRequirement("project_id", selection.Equals, pid),
		selection.NewRequirement("task_id", selection.Equals, tid))

	total, err := a.store.TaskLog().GetTotal(opt)
	if err != nil {
		errObj := errors.ErrInternalError
		errObj.Msg = "获取日志条数失败"
		errObj.Log = err.Error()
		return 0, errObj
	}

	return total, nil
}

func (a *app) GetLogTotalByDate(projects []int64, timestamp int64, errType int) (int, error) {
	opt := selection.NewSelector(selection.NewRequirement("project_id", selection.In, projects),
		selection.NewRequirement("start_time", selection.GreaterThan, timestamp),
		selection.NewRequirement("start_time", selection.LessThan, timestamp+86400),
		selection.NewRequirement("with_error", selection.Equals, errType))

	total, err := a.store.TaskLog().GetTotal(opt)
	if err != nil {
		errObj := errors.ErrInternalError
		errObj.Msg = "获取日志条数失败"
		errObj.Log = err.Error()
		return 0, errObj
	}

	return total, nil
}

func (a *app) CheckProjectExistByName(title string) (*common.Project, error) {
	opt := selection.NewSelector(selection.NewRequirement("title", selection.Equals, title))

	p, err := a.store.Project().GetProject(opt)
	if err != nil && err != gorm.ErrRecordNotFound {
		errObj := errors.ErrInternalError
		errObj.Msg = "获取项目信息失败"
		errObj.Log = err.Error()
		return nil, errObj
	}

	if len(p) == 0 {
		return nil, nil
	}

	return p[0], nil
}

func (a *app) CreateProject(tx *gorm.DB, p common.Project) (int64, error) {
	id, err := a.store.Project().CreateProject(tx, p)
	if err != nil {
		errObj := errors.ErrInternalError
		errObj.Msg = "创建项目失败"
		errObj.Log = err.Error()
		return 0, errObj
	}

	return id, nil
}

func (a *app) DeleteProject(tx *gorm.DB, pid, uid int64) error {
	opt := selection.NewSelector(selection.NewRequirement("id", selection.Equals, pid),
		selection.NewRequirement("uid", selection.Equals, uid))
	if err := a.store.Project().DeleteProject(tx, opt); err != nil {
		errObj := errors.ErrInternalError
		errObj.Msg = "删除项目失败"
		errObj.Log = err.Error()
		return errObj
	}

	return nil
}

func (a *app) UpdateProject(pid int64, title, remark string) error {
	if err := a.store.Project().UpdateProject(pid, title, remark); err != nil {
		errObj := errors.ErrInternalError
		errObj.Msg = "更新项目失败"
		errObj.Log = err.Error()
		return errObj
	}
	return nil
}

func (a *app) CreateProjectRelevance(tx *gorm.DB, pid, uid int64) error {
	if err := a.store.ProjectRelevance().Create(tx, common.ProjectRelevance{
		ProjectID:  pid,
		UID:        uid,
		CreateTime: time.Now().Unix(),
	}); err != nil {
		errObj := errors.ErrInternalError
		errObj.Msg = "创建项目关联关系失败"
		errObj.Log = err.Error()
		return errObj
	}

	return nil
}

func (a *app) DeleteProjectRelevance(tx *gorm.DB, pid, uid int64) error {
	if err := a.store.ProjectRelevance().Delete(tx, pid, uid); err != nil {
		errObj := errors.ErrInternalError
		errObj.Msg = "删除项目关联关系失败"
		errObj.Log = err.Error()
		return errObj
	}
	return nil
}

func (a *app) GetUserByAccount(account string) (*common.User, error) {
	opt := selection.NewSelector(selection.NewRequirement("account", selection.Equals, account))
	opt.Pagesize = 1

	res, err := a.store.User().GetUsers(opt)
	if err != nil && err != gorm.ErrRecordNotFound {
		errObj := errors.ErrInternalError
		errObj.Msg = "获取用户信息失败"
		errObj.Log = err.Error()
		return nil, errObj
	}

	if len(res) == 0 {
		return nil, nil
	}

	return res[0], nil
}

func (a *app) IsAdmin(uid int64) (bool, error) {
	isAdmin := false

	info, err := a.GetUserInfo(uid)
	if err != nil && err != gorm.ErrRecordNotFound {
		errObj := errors.ErrInternalError
		errObj.Msg = "获取用户信息失败"
		errObj.Log = err.Error()
		return false, errObj
	}

	if info == nil {
		return false, errors.ErrDataNotFound
	}

	// 确认该用户是否为管理员
	permissions := strings.Split(info.Permission, ",")
	for _, v := range permissions {
		if v == "admin" {
			isAdmin = true
			break
		}
	}

	return isAdmin, nil
}

func (a *app) GetUserInfo(uid int64) (*common.User, error) {
	opt := selection.NewSelector(selection.NewRequirement("id", selection.Equals, uid))
	res, err := a.store.User().GetUsers(opt)
	if err != nil && err != gorm.ErrRecordNotFound {
		errObj := errors.ErrInternalError
		errObj.Msg = "获取用户信息失败"
		errObj.Log = err.Error()
		return nil, errObj
	}

	if len(res) == 0 {
		return nil, nil
	}

	return res[0], nil
}

func (a *app) DeleteUser(id int64) error {
	if err := a.store.User().DeleteUser(id); err != nil {
		errObj := errors.ErrInternalError
		errObj.Msg = "删除用户失败"
		errObj.Log = err.Error()
		return errObj
	}
	return nil
}

func (a *app) CreateUser(u common.User) error {
	if err := a.store.User().CreateUser(u); err != nil {
		errObj := errors.ErrInternalError
		errObj.Msg = "创建用户失败"
		errObj.Log = err.Error()
		return errObj
	}

	return nil
}

func (a *app) GetProjectRelevanceUsers(pid int64) ([]*common.ProjectRelevance, error) {
	opt := selection.NewSelector(selection.NewRequirement("project_id", selection.Equals, pid))
	res, err := a.store.ProjectRelevance().GetList(opt)
	if err != nil && err != gorm.ErrRecordNotFound {
		errObj := errors.ErrInternalError
		errObj.Msg = "获取用户项目关联列表失败"
		errObj.Log = err.Error()
		return nil, errObj
	}

	return res, nil
}

func (a *app) GetUsersByIDs(uids []int64) ([]*common.User, error) {
	opt := selection.NewSelector(selection.NewRequirement("id", selection.In, uids))
	res, err := a.store.User().GetUsers(opt)
	if err != nil && err != gorm.ErrRecordNotFound {
		errObj := errors.ErrInternalError
		errObj.Msg = "获取用户列表失败"
		errObj.Log = err.Error()
		return nil, errObj
	}

	return res, nil
}

type GetUserListArgs struct {
	ID        int64
	Account   string
	Name      string
	ProjectID int64
	Page      int
	Pagesize  int
}

func (a *app) parseUserSearchArgs(args GetUserListArgs) (selection.Selector, error) {
	opts := selection.NewSelector()

	if args.ProjectID != 0 {
		re, err := a.GetProjectRelevanceUsers(args.ProjectID)
		if err != nil {
			return selection.Selector{}, err
		}

		var ids []int64
		for _, v := range re {
			ids = append(ids, v.UID)
		}

		opts.AddQuery(selection.NewRequirement("id", selection.In, ids))
	} else if args.ID != 0 {
		opts.AddQuery(selection.NewRequirement("id", selection.Equals, args.ID))
	}

	if args.Account != "" {
		opts.AddQuery(selection.NewRequirement("account", selection.Equals, args.Account))
	}

	if args.Name != "" {
		opts.AddQuery(selection.NewRequirement("name", selection.Like, args.Name))
	}

	opts.Page = args.Page
	opts.Pagesize = args.Pagesize
	return opts, nil
}

func (a *app) GetUserList(args GetUserListArgs) ([]*common.User, error) {
	opts, err := a.parseUserSearchArgs(args)
	if err != nil {
		return nil, err
	}

	list, err := a.store.User().GetUsers(opts)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		errObj := errors.ErrInternalError
		errObj.Msg = "获取用户列表失败"
		errObj.Log = err.Error()
		return nil, errObj
	}

	return list, nil
}

func (a *app) ReloadWorkerConfig(host string) error {
	var (
		schedulerKey   string
		leaseGrantResp *clientv3.LeaseGrantResponse
		ctx            context.Context
		errObj         errors.Error
		err            error
	)

	// build etcd save key
	schedulerKey = common.BuildAgentCommandKey(host, common.AGENT_COMMAND_RELOAD_CONFIG)

	ctx, _ = utils.GetContextWithTimeout()
	// make lease to notify worker
	// 创建一个租约 让其稍后过期并自动删除
	if leaseGrantResp, err = a.etcd.Lease().Grant(ctx, 1); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - ReloadAgentConfig] lease grant error:" + err.Error()
		return errObj
	}

	ctx, _ = utils.GetContextWithTimeout()
	// save to etcd
	if _, err = a.etcd.KV().Put(ctx, schedulerKey, "", clientv3.WithLease(leaseGrantResp.ID)); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - ReloadAgentConfig] etcd client kv put error:" + err.Error()
		return errObj
	}

	return nil
}

func (a *app) GetUserListTotal(args GetUserListArgs) (int, error) {
	opts, err := a.parseUserSearchArgs(args)
	if err != nil {
		return 0, err
	}

	total, err := a.store.User().GetTotal(opts)
	if err != nil {
		errObj := errors.ErrInternalError
		errObj.Msg = "获取用户数量失败"
		errObj.Log = err.Error()
		return 0, errObj
	}

	return total, nil
}

func (a *app) ChangePassword(uid int64, password, salt string) error {
	if err := a.store.User().ChangePassword(uid, password, salt); err != nil {
		errObj := errors.ErrInternalError
		errObj.Msg = "更新密码失败"
		errObj.Log = err.Error()
		return errObj
	}

	return nil
}

func (a *app) AutoCleanLogs() {
	opt := selection.NewSelector(selection.NewRequirement("start_time", selection.LessThan, time.Now().Unix()-86400*7))
	if err := a.store.TaskLog().Clean(nil, opt); err != nil {
		a.logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Error("failed to clean logs by auto clean")
	}
}
