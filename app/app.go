package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/config"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/pkg/cronpb"
	"github.com/holdno/gopherCron/pkg/etcd"
	"github.com/holdno/gopherCron/pkg/infra"
	"github.com/holdno/gopherCron/pkg/metrics"
	"github.com/holdno/gopherCron/pkg/panicgroup"
	"github.com/holdno/gopherCron/pkg/store/sqlStore"
	"github.com/holdno/gopherCron/pkg/warning"
	"github.com/holdno/gopherCron/protocol"
	"github.com/holdno/gopherCron/utils"
	"github.com/mikespook/gorbac"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/ugurcsen/gods-generic/maps/hashmap"

	"github.com/gin-gonic/gin"
	"github.com/holdno/gocommons/selection"
	"github.com/jinzhu/gorm"
	"github.com/spacegrower/watermelon/infra/wlog"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type App interface {
	Run()
	Log() wlog.Logger
	CreateProject(tx *gorm.DB, p common.Project) (int64, error)
	ReGenProjectToken(uid, pid int64) (string, error)
	GetProject(pid int64) (*common.Project, error)
	CleanProject(tx *gorm.DB, pid int64) error
	GetUserProjects(uid int64, oid string) ([]*common.ProjectWithUserRole, error)
	CheckProjectExist(oid, title string) (*common.Project, error)
	CheckUserIsInProject(pid, uid int64) (*common.ProjectRelevance, error) // 确认该用户是否加入该项目
	CheckUserProject(pid, uid int64) (*common.Project, error)              // 确认项目是否属于该用户
	UpdateProject(pid int64, title, remark string) error
	DeleteProject(tx *gorm.DB, pid, uid int64) error
	GetUserOrgs(userID int64) ([]*common.Org, error)
	SaveTask(task *common.TaskInfo, opts ...clientv3.OpOption) (*common.TaskInfo, error)
	DeleteTask(pid int64, tid string) (*common.TaskInfo, error)
	DeleteProjectAllTasks(projectID int64) error
	CreateOrg(userID int64, title string) (string, error)
	DeleteOrg(orgID string, userID int64) error
	KillTask(pid int64, tid string) error
	IsAdmin(uid int64) (bool, error)
	GetWorkerList(projectID int64) ([]common.ClientInfo, error)
	CheckProjectWorkerExist(projectID int64, host string) (bool, error)
	ReloadWorkerConfig(host string) error
	GetProjectTaskCount(projectID int64) (int64, error)
	GetTaskList(projectID int64) ([]*common.TaskListItemWithWorkflows, error)
	GetTask(projectID int64, taskID string) (*common.TaskInfo, error)
	TemporarySchedulerTask(user *common.User, task *common.TaskInfo) error
	GetTaskLogList(pid int64, tid string, page, pagesize int) ([]*common.TaskLog, error)
	GetTaskLogDetail(pid int64, tid, tmpID string) (*common.TaskLog, error)
	GetLogTotalByDate(projects []int64, timestamp int64, errType int) (int, error)
	GetTaskLogTotal(pid int64, tid string) (int, error)
	CleanProjectLog(tx *gorm.DB, pid int64) error
	CleanLog(tx *gorm.DB, pid int64, tid string) error
	DeleteAll() error
	CreateProjectRelevance(tx *gorm.DB, pid, uid int64, role string) error
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
	ClusterID() int64
	GetConfig() *config.ServiceConfig
	CreateWebHook(projectID int64, types, CallBackURL string) error
	GetWebHook(projectID int64, types string) (*common.WebHook, error)
	GetWebHookList(projectID int64) ([]*common.WebHook, error)
	DeleteWebHook(tx *gorm.DB, projectID int64, types string) error
	DeleteAllWebHook(tx *gorm.DB, projectID int64) error
	CheckPermissions(projectID, uid int64, permission gorbac.Permission) error
	GetErrorLogs(pids []int64, page, pagesize int) ([]*common.TaskLog, int, error)
	// workflow
	CreateWorkflow(userID int64, data common.Workflow) error
	DeleteWorkflow(userID int64, workflowID int64) error
	UpdateWorkflow(userID int64, data common.Workflow) error
	CreateWorkflowTask(userID int64, data common.WorkflowTask) error
	CreateWorkflowSchedulePlan(userID int64, workflowID int64, taskList []CreateWorkflowSchedulePlanArgs) error
	GetWorkflowList(opts common.GetWorkflowListOptions, page, pagesize uint64) ([]common.Workflow, int, error)
	GetWorkflow(id int64) (*common.Workflow, error)
	GetWorkflowTask(projectID int64, taskID string) (*common.WorkflowTask, error)
	GetProjectWorkflowTask(projectID int64) ([]common.WorkflowTask, error)
	GetUserWorkflows(userID int64) ([]int64, error)
	GetWorkflowScheduleTasks(workflowID int64) ([]common.WorkflowSchedulePlan, error)
	GetUserWorkflowPermission(userID, workflowID int64) error
	GetWorkflowLogList(workflowID int64, page, pagesize uint64) ([]common.WorkflowLog, int, error)
	CreateWorkflowLog(workflowID int64, startTime, endTime int64, result string) error
	ClearWorkflowLog(workflowID int64) error
	GetWorkflowState(workflowID int64) (*PlanState, error)
	GetWorkflowAllTaskStates(workflowID int64) ([]*WorkflowTaskStates, error)
	GetMultiWorkflowTaskList(taskIDs []string) ([]common.WorkflowTask, error)
	StartWorkflow(workflowID int64) error
	KillWorkflow(workflowID int64) error
	UpdateWorkflowTask(userID int64, data common.WorkflowTask) error
	DeleteWorkflowTask(userID, projectID int64, taskID string) error
	WorkflowRemoveUser(workflowID, userID int64) error
	WorkflowAddUser(workflowID, userID int64) error
	GetWorkflowRelevanceUsers(workflowID int64) ([]common.UserWorkflowRelevance, error)
	// metrics
	Metrics() *metrics.Metrics
	// web sockets
	PublishMessage(data PublishData)
	// temporary task
	CreateTemporaryTask(data common.TemporaryTask) error
	GetTemporaryTaskListWithUser(projectID int64) ([]TemporaryTaskListWithUser, error)
	DeleteTemporaryTask(id int64) error
	TemporaryTaskSchedule(tmpTask common.TemporaryTask) error
	AutoCleanScheduledTemporaryTask()

	BeginTx() *gorm.DB
	Close()
	GetVersion() string

	GetAgentClient(region string, projectID int64) (*AgentClient, error)
	GetEtcdClient() *clientv3.Client
	Go(f func())
	warning.Warner

	// registry
	StreamManager() *streamManager[*cronpb.Event]
	StreamManagerV2() *streamManager[*cronpb.ServiceEvent]
	DispatchAgentJob(projectID int64) error
	RemoveClientRegister(client string) error
	HandleCenterEvent(event *cronpb.ServiceEvent) error

	// proxy
	GetGrpcDirector() func(ctx context.Context, fullMethodName string) (context.Context, *grpc.ClientConn, error)

	// task status
	SetTaskRunning(agentIP string, execInfo *common.TaskExecutingInfo) error
	CheckTaskIsRunning(projectID int64, taskID string) ([]common.TaskRunningInfo, error)
	HandlerTaskFinished(agentIP string, result *common.TaskFinishedV2) error
	SaveTaskLog(agentIP string, result common.TaskFinishedV2)

	GetOIDCService() *OIDCService
}

func GetApp(c *gin.Context) App {
	return c.MustGet(common.APP_KEY).(App)
}

type app struct {
	ctx        context.Context
	cancelFunc context.CancelFunc
	// connPool   keypool.Pool[*grpc.ClientConn]
	clusterID  int64
	httpClient *http.Client
	store      sqlStore.SqlStore
	etcd       protocol.EtcdManager

	isClose bool
	localip string
	metrics *metrics.Metrics

	workflowRunner *workflowRunner
	messageChan    chan PublishData

	cfg *config.ServiceConfig

	panicgroup.PanicGroup
	protocol.CommonInterface
	warning.Warner

	pusher          *SystemPusher
	streamManager   *streamManager[*cronpb.Event]
	streamManagerV2 *streamManager[*cronpb.ServiceEvent]

	oidcSrv *OIDCService
	author  *Author
	rbacSrv RBACImpl

	__centerConncets *hashmap.Map[string, *CenterClient]
}

type WebClientPusher interface {
	Publish(messageId, source, topic string, data json.RawMessage) error
}

func (a *app) GetEtcdClient() *clientv3.Client {
	return a.etcd.Client()
}

func (a *app) GetOIDCService() *OIDCService {
	return a.oidcSrv
}

var (
	POOL_ERROR_FACTORY_NOT_FOUND = fmt.Errorf("factory not found")
)

func NewApp(configPath string) App {
	var err error

	conf := config.InitServiceConfig(configPath)

	infra.RegisterETCDRegisterPrefixKey(conf.Etcd.Prefix + "/registry")
	wlog.SetGlobalLogger(wlog.NewLogger(&wlog.Config{
		Level: wlog.ParseLevel(conf.LogLevel),
		RotateConfig: &wlog.RotateConfig{
			MaxAge:  24,
			MaxSize: 1000,
		},
		File: conf.LogPath, // print to sedout if config.File undefined
	}))
	app := &app{
		Warner: warning.NewDefaultWarner(wlog.With(zap.String("component", "warner"))),
		cfg:    conf,
		author: &Author{
			privateKey: []byte(conf.JWT.PrivateKey),
		},
		rbacSrv: NewRBACSrv(),
	}
	app.ctx, app.cancelFunc = context.WithCancel(context.Background())
	if conf.ReportAddr != "" {
		app.Warner = warning.NewHttpReporter(conf.ReportAddr, func() (string, error) {
			return app.author.token, nil
		})
	}

	if conf.Mysql != nil && conf.Mysql.Service != "" {
		app.store = sqlStore.MustSetup(conf.Mysql, wlog.With(zap.String("component", "sqlprovider")), conf.Mysql.AutoCreate)
	}

	if app.localip, err = utils.GetLocalIP(); err != nil {
		wlog.Error("!!! --- failed to get local ip --- !!!")
	}

	app.metrics = metrics.NewMetrics("center", app.GetIP())

	if conf.OIDC.ClientID != "" {
		if app.oidcSrv, err = NewOIDCService(app, conf.OIDC); err != nil {
			wlog.Panic("failed to setup oidc service", zap.Any("config", conf.OIDC), zap.Error(err))
		}
	}

	app.httpClient = &http.Client{
		Timeout: time.Second * 5,
	}

	{
		// watermelon config
		wlog.NewLogger(&wlog.Config{
			Level: wlog.DebugLevel,
		})
		// register etcd registry
		// todo register etcd client instead of config
		infra.RegisterEtcdClient(clientv3.Config{
			Endpoints:   conf.Etcd.Service, // cluster list
			Username:    conf.Etcd.Username,
			Password:    conf.Etcd.Password,
			DialTimeout: time.Duration(conf.Etcd.DialTimeout) * time.Millisecond,
		})

		app.streamManager = &streamManager[*cronpb.Event]{
			aliveSrv:  make(map[string]map[string]*Stream[*cronpb.Event]),
			hostIndex: make(map[string]streamHostIndex),
		}
		app.streamManagerV2 = &streamManager[*cronpb.ServiceEvent]{
			isV2:      true,
			aliveSrv:  make(map[string]map[string]*Stream[*cronpb.ServiceEvent]),
			hostIndex: make(map[string]streamHostIndex),
			responseMap: &responseMap{
				m: cmap.New[*streamResponse](),
			},
		}
	}

	wlog.Info("start to connect etcd ...")
	if app.etcd, err = etcd.Connect(conf.Etcd); err != nil {
		panic(err)
	}

	wlog.Info("connected to etcd")
	app.CommonInterface = protocol.NewComm(app.etcd)

	clusterID, err := app.etcd.Inc(conf.Etcd.Prefix + common.CLUSTER_AUTO_INDEX)
	if err != nil {
		panic(err)
	}

	// why 1024. view https://github.com/holdno/snowFlakeByGo
	app.clusterID = clusterID % 1024
	utils.InitIDWorker(app.clusterID)
	app.PanicGroup = panicgroup.NewPanicGroup(func(err error) {
		reserr := app.Warning(warning.NewSystemWarningData(warning.SystemWarning{
			Endpoint: app.GetIP(),
			Type:     warning.SERVICE_TYPE_CENTER,
			Message:  fmt.Sprintf("center-service: %s, panic: %s", app.localip, err.Error()),
		}))
		if reserr != nil {
			wlog.With(zap.Any("fields", map[string]interface{}{
				"desc":         reserr,
				"source_error": err,
			})).Error("panicgroup: failed to warning panic error")
		}
	})

	if app.cfg.Publish.Enable {
		wlog.Info("enable publish service", zap.String("endpoint", app.cfg.Publish.Endpoint))
		app.pusher = &SystemPusher{
			clientID: app.localip,
			Endpoint: app.cfg.Publish.Endpoint,
			client:   &http.Client{Timeout: time.Duration(app.cfg.Deploy.Timeout) * time.Second},
			Header:   app.cfg.Publish.Header,
		}

		app.messageChan = make(chan PublishData, 1000)
		publishQpsInc := app.metrics.CustomIncFunc("publish_message", "", "")
		app.Go(func() {
			for {
				select {
				case res := <-app.messageChan:
					publishQpsInc()
					app.publishEventToWebClient(res)
				case <-app.ctx.Done():
					return
				}
			}
		})
	}

	return app
}

func (a *app) Run() {
	go resolveCenterService(a)
	startCleanupTask(a)
	startWebhook(a)
	startWorkflow(a)
	startTemporaryTaskWorker(a)
	startCalcDataConsistency(a)
}

func (a *app) GetVersion() string {
	return protocol.GetVersion()
}

func (a *app) Metrics() *metrics.Metrics {
	return a.metrics
}

func (a *app) HandleCenterEvent(event *cronpb.ServiceEvent) error {
	if event == nil {
		return nil
	}
	switch event.Type {
	case cronpb.EventType_EVENT_WORKFLOW_REFRESH:
		if err := a.workflowRunner.RefreshPlan(); err != nil {
			return err
		}
		if a.workflowRunner.isLeader {
			a.workflowRunner.reCalcScheduleTimeChan <- struct{}{}
		}
	default:
		return fmt.Errorf("unsupport event %s, version: %s", event.Type, a.GetVersion())
	}
	return nil
}

func startWebhook(app *app) {
	// app.election(common.BuildWebhookMasterKey(), func(s *concurrency.Session) error {
	// 	wlog.Info("new webhook leader")
	// 	for {
	// 		select {
	// 		case <-app.ctx.Done():
	// 			return app.ctx.Err()
	// 		case <-s.Done():
	// 			return nil
	// 		default:
	// 			if err := app.WebHookWorker(s.Done()); err != nil {
	// 				wlog.Error("webhook runner return error", zap.Error(err))
	// 				return err
	// 			}
	// 		}
	// 	}
	// })
}

func startCleanupTask(app *app) {
	// 自动清理任务
	app.election(common.BuildCleanupMasterKey(), func(s *concurrency.Session) error {
		wlog.Info("new tasks cleanup leader")
		t := time.NewTicker(time.Hour * 12)
	BreakHere:
		for {
			select {
			case <-t.C:
				app.AutoCleanLogs()
				app.AutoCleanScheduledTemporaryTask()
			case <-app.ctx.Done():
				t.Stop()
				s.Close()
				break BreakHere
			case <-s.Done():
				t.Stop()
				break BreakHere
			}
		}
		return nil
	})
}

func (a *app) election(key string, successFunc func(s *concurrency.Session) error) error {
	ele := func() error {
		s, err := concurrency.NewSession(a.GetEtcdClient(), concurrency.WithTTL(60))
		if err != nil {
			wlog.Error("failed to new concurrency.Session", zap.Error(err), zap.String("key", key))
			return err
		}
		defer func() {
			if r := recover(); r != nil {
				wlog.Error("election was recovered", zap.Any("info", r))
			}
			if s != nil {
				s.Close()
			}
		}()

		e := concurrency.NewElection(s, key)
		if err = e.Campaign(a.ctx, a.GetIP()); err != nil {
			return err
		}

		return successFunc(s)
	}
	a.Go(func() {
		for {
			select {
			case <-a.ctx.Done():
				return
			default:
			}
			if err := ele(); err != nil {
				wlog.Error("failed to election", zap.String("key", key), zap.Error(err))
			}
			time.Sleep(time.Second * 5)
		}
	})
	return nil
}

func startWorkflow(app *app) {
	var err error
	if app.workflowRunner, err = NewWorkflowRunner(app, app.etcd.Client()); err != nil {
		panic(err)
	}

	app.election(common.BuildWorkflowMasterKey(), func(s *concurrency.Session) error {
		defer func() {
			app.workflowRunner.isLeader = false
		}()
		app.workflowRunner.isLeader = true
		wlog.Info("new workflow leader")
		if err = app.workflowRunner.RefreshPlan(); err != nil {
			return err
		}
		app.workflowRunner.Loop(s.Done())
		return nil
	})
}

func startCalcDataConsistency(app *app) {
	app.election(common.BuildCalaConsistencyMasterKey(), func(s *concurrency.Session) error {
		wlog.Info("new calc leader")
		app.metrics.CustomInc("agents_task_calc_leader", app.localip, "")

		if err := app.CalcAgentDataConsistency(s.Done()); err != nil {
			wlog.Error("failed to calc agent data consistency", zap.Error(err))
			return err
		}
		return nil
	})
}

func startTemporaryTaskWorker(app *app) {
	app.election(common.BuildTemporaryMasterKey(), func(s *concurrency.Session) error {
		wlog.Info("new temporary scheduler leader")
		app.metrics.CustomInc("temporary_scheduler_leader", app.localip, "")

		c := time.NewTicker(time.Minute)
		defer c.Stop()
		for {
			select {
			case <-s.Done():
				return nil
			case <-app.ctx.Done():
				return nil
			case <-c.C:
			}

			list, err := app.GetNeedToScheduleTemporaryTask(time.Now())
			if err != nil {
				wlog.Error("failed to refresh temporary task list", zap.Error(err))
				continue
			}

			for _, v := range list {
				if err = app.TemporaryTaskSchedule(*v); err != nil {
					wlog.Error("temporary task worker: failed to schedule task", zap.Error(err))
					return err
				}
			}
		}
	})
}

func (a *app) ClusterID() int64 {
	return a.clusterID
}

func (a *app) Log() wlog.Logger {
	return wlog.With()
}

func (a *app) PublishMessage(data PublishData) {
	if a.messageChan == nil || len(a.messageChan) == 1000 {
		// todo
		return
	}
	a.messageChan <- data
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

func (a *app) Close() {
	if !a.isClose {
		a.isClose = true
		a.workflowRunner.cancelFunc()
		a.cancelFunc()
	}
}

func (a *app) BeginTx() *gorm.DB {
	return a.store.BeginTx()
}

func (a *app) CheckUserIsInProject(pid, uid int64) (*common.ProjectRelevance, error) {
	opt := selection.NewSelector(selection.NewRequirement("project_id", selection.Equals, pid),
		selection.NewRequirement("uid", selection.FindIn, uid))
	opt.Select = "*"
	res, err := a.store.ProjectRelevance().GetList(opt)
	if err != nil && err != common.ErrNoRows {
		errObj := errors.ErrInternalError
		errObj.Msg = "获取项目归属信息失败"
		errObj.Log = err.Error()
		return nil, errObj
	}
	if len(res) == 0 {
		return nil, nil
	}

	return res[0], nil
}

func (a *app) CheckUserProject(pid, uid int64) (*common.Project, error) {
	opt := selection.NewSelector(selection.NewRequirement("id", selection.Equals, pid))
	if uid != common.ADMIN_USER_ID {
		opt.AddQuery(selection.NewRequirement("uid", selection.Equals, uid))
	}
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

func (a *app) GetProjects(oid string) ([]*common.Project, error) {
	opt := selection.NewSelector(selection.NewRequirement("oid", selection.Equals, oid))
	projects, err := a.store.Project().GetProject(opt)
	if err != nil && err != gorm.ErrRecordNotFound {
		errObj := errors.ErrInternalError
		errObj.Msg = "无法获取项目信息"
		errObj.Log = err.Error()
		return nil, errObj
	}

	return projects, nil
}

func (a *app) GetUserProjects(uid int64, oid string) ([]*common.ProjectWithUserRole, error) {
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

	projectsMap := hashmap.New[int64, *common.ProjectRelevance]()
	for _, v := range res {
		projectsMap.Put(v.ProjectID, v)
	}

	opt = selection.NewSelector(selection.NewRequirement("oid", selection.Equals, oid),
		selection.NewRequirement("id", selection.In, projectsMap.Keys()))
	projects, err := a.store.Project().GetProject(opt)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, errors.NewError(http.StatusInternalServerError, "无法获取项目信息").WithLog(err.Error())
	}

	var list []*common.ProjectWithUserRole
	for _, v := range projects {
		userPermission, exist := projectsMap.Get(v.ID)
		if !exist {
			continue
		}
		if userPermission.Role == "" {
			userPermission.Role = common.PERMISSION_USER
		}
		list = append(list, &common.ProjectWithUserRole{
			Project: v,
			Role:    userPermission.Role,
		})
	}

	return list, nil
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

func (a *app) GetTaskLogDetail(pid int64, tid string, tmpID string) (*common.TaskLog, error) {
	res, err := a.store.TaskLog().GetOne(pid, tid, tmpID)
	if err != nil && err != gorm.ErrRecordNotFound {
		errObj := errors.ErrInternalError
		errObj.Msg = "获取日志列表失败"
		errObj.Log = err.Error()
		return nil, errObj
	}

	return res, nil
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

func (a *app) GetErrorLogs(pids []int64, page, pagesize int) ([]*common.TaskLog, int, error) {
	opt := selection.NewSelector(selection.NewRequirement("with_error", selection.Equals, common.ErrorLog))
	if len(pids) > 0 {
		opt.AddQuery(selection.NewRequirement("project_id", selection.In, pids))
	}
	opt.Page = page
	opt.Pagesize = pagesize
	opt.OrderBy = "id DESC"

	list, err := a.store.TaskLog().GetList(opt)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, 0, errors.NewError(http.StatusInternalServerError, "获取日志列表失败").WithLog(err.Error())
	}

	total, err := a.store.TaskLog().GetTotal(opt)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, 0, errors.NewError(http.StatusInternalServerError, "获取日志列表总数失败").WithLog(err.Error())
	}

	return list, total, nil
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

func (a *app) CheckProjectExist(oid, title string) (*common.Project, error) {
	opt := selection.NewSelector(selection.NewRequirement("oid", selection.Equals, oid),
		selection.NewRequirement("title", selection.Equals, title))

	p, err := a.store.Project().GetProject(opt)
	if err != nil && err != common.ErrNoRows {
		return nil, errors.NewError(http.StatusInternalServerError, "项目重名检测失败").WithLog(err.Error())
	}

	if len(p) == 0 {
		return nil, nil
	}

	return p[0], nil
}

func (a *app) ReGenProjectToken(uid, pid int64) (string, error) {
	if err := a.CheckPermissions(pid, uid, PermissionAll); err != nil {
		return "", err
	}

	newToken := utils.RandomStr(32)

	if err := a.store.Project().UpdateToken(pid, newToken); err != nil {
		return "", errors.NewError(http.StatusInternalServerError, "重置项目token失败").WithLog(err.Error())
	}
	return newToken, nil
}

func (a *app) CreateProject(tx *gorm.DB, p common.Project) (int64, error) {
	isAdmin, err := a.IsAdmin(p.UID)
	if err != nil {
		return 0, err
	}
	if !isAdmin {
		exist, err := a.store.OrgRelevance().GetUserOrg(p.OID, p.UID)
		if err != nil && err != common.ErrNoRows {
			return 0, errors.NewError(http.StatusInternalServerError, "创建项目失败，获取用户组织信息失败").WithLog(err.Error())
		}

		if exist == nil {
			return 0, errors.NewError(http.StatusForbidden, "无权限")
		}
	}

	id, err := a.store.Project().CreateProject(tx, p)
	if err != nil {
		return 0, errors.NewError(http.StatusInternalServerError, "创建项目失败").WithLog(err.Error())
	}

	if err = a.CreateProjectRelevance(tx, id, p.UID, RoleChief.IDStr); err != nil {
		return 0, err
	}

	return id, nil
}

func (a *app) DeleteProject(tx *gorm.DB, pid, uid int64) error {
	opt := selection.NewSelector(selection.NewRequirement("id", selection.Equals, pid))
	isAdmin, err := a.IsAdmin(uid)
	if err != nil {
		return err
	}
	if !isAdmin {
		opt.AddQuery(selection.NewRequirement("uid", selection.Equals, uid))
	}
	if err := a.store.Project().DeleteProject(tx, opt); err != nil {
		return errors.NewError(http.StatusInternalServerError, "删除项目失败").WithLog(err.Error())
	}

	return nil
}

func (a *app) UpdateProject(pid int64, title, remark string) error {
	if err := a.store.Project().UpdateProject(pid, title, remark); err != nil {
		return errors.NewError(http.StatusInternalServerError, "更新项目失败").WithLog(err.Error())
	}
	return nil
}

func (a *app) CreateProjectRelevance(tx *gorm.DB, pid, uid int64, roleStr string) error {
	var err error
	if tx == nil {
		tx = a.BeginTx()
		defer func() {
			if r := recover(); r != nil || err != nil {
				if err == nil {
					err = fmt.Errorf("panic: %s", r)
				}
				tx.Rollback()
			} else {
				tx.Commit()
			}
		}()
	}

	// 检测用户是否存在项目组中
	userProjectRel, err := a.CheckUserIsInProject(pid, uid)
	if err != nil {
		return errors.NewError(http.StatusInternalServerError, "获取用户项目关联信息失败").WithLog(err.Error())
	}
	if userProjectRel != nil {
		return nil
	}

	project, err := a.store.Project().GetProjectByID(tx, pid)
	if err != nil && err != common.ErrNoRows {
		return errors.NewError(http.StatusInternalServerError, "获取项目信息失败").WithLog(err.Error())
	}

	if project == nil {
		return errors.NewError(http.StatusForbidden, "项目不存在")
	}

	role, exist := a.rbacSrv.GetRole(roleStr)
	if !exist {
		return errors.NewError(http.StatusBadRequest, "未定义的权限")
	}

	if err := a.store.ProjectRelevance().Create(tx, common.ProjectRelevance{
		ProjectID:  pid,
		UID:        uid,
		Role:       role.ID(),
		CreateTime: time.Now().Unix(),
	}); err != nil {
		return errors.NewError(http.StatusInternalServerError, "创建项目关联关系失败").WithLog(err.Error())
	}

	rel, err := a.store.OrgRelevance().GetUserOrg(project.OID, uid)
	if err != nil && err != common.ErrNoRows {
		return errors.NewError(http.StatusInternalServerError, "获取用户组织信息失败").WithLog(err.Error())
	}

	if rel == nil {
		err = a.store.OrgRelevance().Create(tx, common.OrgRelevance{
			OID:        project.OID,
			UID:        uid,
			Role:       RoleUser.IDStr,
			CreateTime: time.Now().Unix(),
		})
		if err != nil {
			return errors.NewError(http.StatusInternalServerError, "创建用户组织关系失败").WithLog(err.Error())
		}
	}

	return nil
}

func (a *app) DeleteProjectRelevance(tx *gorm.DB, pid, uid int64) error {
	if err := a.store.ProjectRelevance().Delete(tx, pid, uid); err != nil {
		return errors.NewError(http.StatusInternalServerError, "删除项目关联关系失败").WithLog(err.Error())
	}
	return nil
}

func (a *app) GetUserByAccount(account string) (*common.User, error) {
	opt := selection.NewSelector(selection.NewRequirement("account", selection.Equals, account))
	opt.Pagesize = 1

	res, err := a.store.User().GetUsers(opt)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, errors.NewError(http.StatusInternalServerError, "获取用户信息失败").WithLog(err.Error())
	}

	if len(res) == 0 {
		return nil, nil
	}

	return res[0], nil
}

func (a *app) CheckPermissions(projectID, uid int64, permission gorbac.Permission) error {
	// 首先确认操作的用户是否为该项目的管理员
	isAdmin, err := a.IsAdmin(uid)
	if err != nil {
		return err
	}

	if !isAdmin {
		exist, err := a.CheckUserIsInProject(projectID, uid)
		if err != nil {
			return err
		} else if exist == nil {
			return errors.ErrProjectNotExist
		}

		if !a.rbacSrv.IsGranted(exist.Role, permission) {
			return errors.ErrInsufficientPermissions
		}
	}

	return nil
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
	opts.OrderBy = "id DESC"
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
	ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
	defer cancel()

	cc, err := grpc.DialContext(ctx, host, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return errors.NewError(http.StatusInternalServerError, "获取agent连接失败").WithLog(err.Error())
	}

	defer cc.Close()

	client := cronpb.NewAgentClient(cc)
	if _, err = client.Command(ctx, &cronpb.CommandRequest{
		Command: common.AGENT_COMMAND_RELOAD_CONFIG,
	}); err != nil {
		return errors.NewError(http.StatusInternalServerError, "命令执行失败:"+err.Error()).WithLog(err.Error())
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
		wlog.Error("failed to clean logs by auto clean", zap.Error(err))
	}
}
