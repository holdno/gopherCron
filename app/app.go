package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/config"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/jwt"
	"github.com/holdno/gopherCron/pkg/cronpb"
	"github.com/holdno/gopherCron/pkg/etcd"
	"github.com/holdno/gopherCron/pkg/infra"
	"github.com/holdno/gopherCron/pkg/metrics"
	"github.com/holdno/gopherCron/pkg/panicgroup"
	"github.com/holdno/gopherCron/pkg/store/sqlStore"
	"github.com/holdno/gopherCron/pkg/warning"
	"github.com/holdno/gopherCron/protocol"
	"github.com/holdno/gopherCron/utils"
	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
	"github.com/holdno/gocommons/selection"
	"github.com/holdno/keypool"
	"github.com/jinzhu/gorm"
	"github.com/spacegrower/watermelon/infra/wlog"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

type App interface {
	Log() wlog.Logger
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
	KillTask(pid int64, tid string) error
	IsAdmin(uid int64) (bool, error)
	GetWorkerList(projectID int64) ([]common.ClientInfo, error)
	CheckProjectWorkerExist(projectID int64, host string) (bool, error)
	ReloadWorkerConfig(host string) error
	GetProjectTaskCount(projectID int64) (int64, error)
	GetTaskList(projectID int64) ([]*common.TaskListItemWithWorkflows, error)
	GetTask(projectID int64, taskID string) (*common.TaskInfo, error)
	GetMonitor(ip string) (*common.MonitorInfo, error)
	TemporarySchedulerTask(task *common.TaskInfo) error
	GetTaskLogList(pid int64, tid string, page, pagesize int) ([]*common.TaskLog, error)
	GetTaskLogDetail(pid int64, tid, tmpID string) (*common.TaskLog, error)
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
	CreateWebHook(projectID int64, types, CallBackURL string) error
	GetWebHook(projectID int64, types string) (*common.WebHook, error)
	GetWebHookList(projectID int64) ([]common.WebHook, error)
	DeleteWebHook(tx *gorm.DB, projectID int64, types string) error
	DeleteAllWebHook(tx *gorm.DB, projectID int64) error
	CheckPermissions(projectID, uid int64) error
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
	// web sockets
	PublishMessage(data PublishData)
	// temporary task
	CreateTemporaryTask(data common.TemporaryTask) error
	GetTemporaryTaskListWithUser(projectID int64) ([]TemporaryTaskListWithUser, error)
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
	StreamManager() *streamManager
	DispatchAgentJob(region string, projectID int64, withStream ...*Stream) error
	RemoveClientRegister(client string) error

	// task status
	SetTaskRunning(agentIP string, execInfo *common.TaskExecutingInfo) error
	CheckTaskIsRunning(projectID int64, taskID string) ([]common.TaskRunningInfo, error)
	HandlerTaskFinished(agentIP string, result protocol.TaskFinishedV1) error
}

func GetApp(c *gin.Context) App {
	return c.MustGet(common.APP_KEY).(App)
}

type app struct {
	connPool   keypool.Pool[*grpc.ClientConn]
	clusterID  int64
	httpClient *http.Client
	store      sqlStore.SqlStore
	etcd       protocol.EtcdManager
	closeCh    chan struct{}
	isClose    bool
	localip    string
	metrics    *metrics.Metrics

	workflowRunner *workflowRunner
	messageChan    chan PublishData
	messageCounter int64
	taskResultChan chan string

	cfg *config.ServiceConfig

	panicgroup.PanicGroup
	protocol.CommonInterface
	warning.Warner

	pusher *SystemPusher

	streamManager *streamManager
}

func (a *app) GetAgentClient(region string, projectID int64) (*AgentClient, error) {
	key := fmt.Sprintf("client_%s_%d", region, projectID)
	conn, err := a.connPool.Get(key)
	if err != nil {
		return nil, err
	}

	client := &AgentClient{
		AgentClient: cronpb.NewAgentClient(conn.Conn()),
		addr:        key,
		cancel: func() {
			a.connPool.Put(key, conn)
		},
	}

	return client, nil
}

type WebClientPusher interface {
	Publish(messageId, source, topic string, data json.RawMessage) error
}

func (a *app) GetEtcdClient() *clientv3.Client {
	return a.etcd.Client()
}

type AppOptions func(a *app)

func WithWarning(w warning.Warner) AppOptions {
	return func(a *app) {
		a.Warner = w
	}
}

// func WithFiretower() AppOptions {
// 	return func(a *app) {
// 		if !a.cfg.Firetower.Enable {
// 			return
// 		}
// 		mode := towerconfig.SingleMode
// 		if a.cfg.Firetower.Redis.Addr != "" || a.cfg.Firetower.Nats.Addr != "" {
// 			mode = towerconfig.ClusterMode
// 		}
// 		var err error
// 		a.firetower, err = tower.Setup(towerconfig.FireTowerConfig{
// 			ChanLens:    1000,
// 			Heartbeat:   30,
// 			ServiceMode: mode,
// 			Bucket: towerconfig.BucketConfig{
// 				Num:              4,
// 				CentralChanCount: 1000,
// 				BuffChanCount:    1000,
// 				ConsumerNum:      1,
// 			},
// 			Cluster: towerconfig.Cluster{
// 				RedisOption: towerconfig.Redis{
// 					KeyPrefix: a.cfg.Firetower.Redis.KeyPrefix,
// 					Addr:      a.cfg.Firetower.Redis.Addr,
// 					Password:  a.cfg.Firetower.Redis.Password,
// 					DB:        a.cfg.Firetower.Redis.DB,
// 				},
// 				NatsOption: towerconfig.Nats{
// 					Addr:       a.cfg.Firetower.Nats.Addr,
// 					ServerName: a.cfg.Firetower.Nats.ServerName,
// 					UserName:   a.cfg.Firetower.Nats.UserName,
// 					Password:   a.cfg.Firetower.Nats.Password,
// 				},
// 			},
// 		})
// 		if err != nil {
// 			panic(err)
// 		}

// 		a.pusher = &SystemPusher{
// 			clientID: "system",
// 		}

// 		a.messageChan = make(chan PublishData, 1000)
// 		a.Go(func() {
// 			for {
// 				res := <-a.messageChan
// 				a.publishEventToWebClient(res)
// 			}
// 		})
// 	}
// }

var (
	POOL_ERROR_FACTORY_NOT_FOUND = fmt.Errorf("factory not found")
)

func NewApp(configPath string, opts ...AppOptions) App {
	var err error

	conf := config.InitServiceConfig(configPath)
	wlog.SetGlobalLogger(wlog.NewLogger(&wlog.Config{
		Level: wlog.ParseLevel(conf.LogLevel),
		RotateConfig: &wlog.RotateConfig{
			MaxAge:  24,
			MaxSize: 1000,
		},
		File: conf.LogPath, // print to sedout if config.File undefined
	}))
	app := new(app)
	app.cfg = conf
	if conf.Mysql != nil && conf.Mysql.Service != "" {
		app.store = sqlStore.MustSetup(conf.Mysql, wlog.With(zap.String("component", "sqlprovider")), true)
	}

	if app.localip, err = utils.GetLocalIP(); err != nil {
		wlog.Error("failed to get local ip")
	}

	if app.cfg.Publish.Enable {
		wlog.Info("enable publish service", zap.String("endpoint", app.cfg.Publish.Endpoint))
		app.pusher = &SystemPusher{
			clientID: app.localip,
			Endpoint: app.cfg.Publish.Endpoint,
			client:   &http.Client{Timeout: time.Duration(app.cfg.Deploy.Timeout) * time.Second},
		}
	}

	app.metrics = metrics.NewMetrics("center", app.GetIP())
	app.connPool, err = keypool.NewChannelPool(&keypool.Config[*grpc.ClientConn]{
		MaxCap: 10000,
		//最大空闲连接
		MaxIdle: 10,
		//生成连接的方法
		Factory: func(key string) (*grpc.ClientConn, error) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(conf.Deploy.Timeout))
			defer cancel()
			addr := key
			if strings.Contains(key, "agent_") {
				addr = strings.TrimPrefix(key, "agent_")
			} else if strings.Contains(key, "client_") {
				// client 的连接对象由调用时提供初始化
				keys := strings.Split(key, "_")
				if len(keys) != 3 {
					return nil, POOL_ERROR_FACTORY_NOT_FOUND
				}
				projectID, err := strconv.ParseInt(keys[2], 10, 64)
				if err != nil {
					return nil, err
				}
				newConn := infra.NewClientConn()
				cc, err := newConn(cronpb.Agent_ServiceDesc.ServiceName,
					newConn.WithRegion(keys[1]),
					newConn.WithSystem(projectID),
					newConn.WithOrg("gophercron"),
					newConn.WithGrpcDialOptions(grpc.WithTransportCredentials(insecure.NewCredentials())),
					newConn.WithServiceResolver(infra.MustSetupEtcdResolver()))
				if err != nil {
					return nil, errors.NewError(http.StatusInternalServerError, fmt.Sprintf("连接agent失败，project_id: %d", projectID)).WithLog(err.Error())
				}
				return cc, nil
			}
			return grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		},
		//关闭连接的方法
		Close: func(cc *grpc.ClientConn) error {
			return cc.Close()
		},
		//检查连接是否有效的方法
		Ping: func(cc *grpc.ClientConn) error {
			state := cc.GetState().String()
			switch state {
			case connectivity.Shutdown.String():
				fallthrough
			case connectivity.TransientFailure.String():
				fallthrough
			case "INVALID_STATE":
				return fmt.Errorf("error state %s", cc.GetState().String())
			default:
				wlog.Debug("ping conn status", zap.String("status", state), zap.String("component", "connPool"))
				return nil
			}
		},
		//连接最大空闲时间，超过该事件则将失效
		IdleTimeout: time.Minute * 10,
	})
	if err != nil {
		panic(err)
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

		app.streamManager = &streamManager{
			aliveSrv:  make(map[string]map[string]*Stream),
			hostIndex: make(map[string]streamHostIndex),
		}
	}

	wlog.Info("start to connect etcd ...")
	if app.etcd, err = etcd.Connect(conf.Etcd); err != nil {
		panic(err)
	}

	{
		app.taskResultChan = make(chan string, 1000)
	}

	wlog.Info("connected to etcd")
	app.CommonInterface = protocol.NewComm(app.etcd)

	clusterID, err := app.etcd.Inc(conf.Etcd.Prefix + common.CLUSTER_AUTO_INDEX)
	if err != nil {
		panic(err)
	}

	app.clusterID = clusterID % 1024
	app.PanicGroup = panicgroup.NewPanicGroup(func(err error) {
		reserr := app.Warning(warning.WarningData{
			Data:    err.Error(),
			Type:    warning.WarningTypeSystem,
			AgentIP: app.localip,
		})
		if reserr != nil {
			wlog.With(zap.Any("fields", map[string]interface{}{
				"desc":         reserr,
				"source_error": err,
			})).Error("panicgroup: failed to warning panic error")
		}
	})
	for _, opt := range opts {
		opt(app)
	}

	if app.Warner == nil {
		app.Warner = warning.NewDefaultWarner(wlog.With(zap.String("component", "warner")))
	}

	jwt.InitJWT(conf.JWT)

	// why 1024. view https://github.com/holdno/snowFlakeByGo
	utils.InitIDWorker(app.clusterID)
	app.Go(func() {
		for {
			select {
			case <-app.closeCh:
				return
			default:
				if err = app.WebHookWorker(); err != nil {
					wlog.Error(err.Error())
				}
			}
		}
	})

	// 自动清理任务
	app.Go(func() {
		t := time.NewTicker(time.Hour * 12)
		for {
			select {
			case <-t.C:
				app.AutoCleanLogs()
				app.AutoCleanScheduledTemporaryTask()
			case <-app.closeCh:
				t.Stop()
				// app.etcd.Lock(nil).CloseAll()
				return
			}
		}
	})

	workflow, err := NewWorkflowRunner(app, app.etcd.Client())
	if err != nil {
		panic(err)
	}

	app.Go(func() {
		for {
			// 同一时间只需要有一个service的Loop运行即可
			s, err := concurrency.NewSession(app.etcd.Client(), concurrency.WithTTL(9))
			if err != nil {
				fmt.Println(err)
				continue
			}

			e := concurrency.NewElection(s, common.BuildWorkflowMasterKey())
			ctx, _ := context.WithTimeout(context.TODO(), time.Duration(app.GetConfig().Deploy.Timeout)*time.Second)
			err = e.Campaign(ctx, app.GetIP())
			if err != nil {
				switch {
				case err == context.Canceled:
					return
				default:
					time.Sleep(time.Second * 5)
					continue
				}
			}
			fmt.Println("new workflow leader")
			list, _, err := app.GetWorkflowList(common.GetWorkflowListOptions{}, 1, 100000)
			if err != nil {
				wlog.Error("failed to refresh workflow list", zap.Error(err))
				continue
			}

			for _, v := range list {
				workflow.SetPlan(v)
			}
			workflow.Loop()
		}
	})
	app.workflowRunner = workflow

	startTemporaryTaskWorker(app)

	startCalaDataConsistency(app)
	return app
}

func startCalaDataConsistency(app *app) {
	app.Go(func() {
		for {
			// 同一时间只需要有一个service的Loop运行即可
			s, err := concurrency.NewSession(app.etcd.Client(), concurrency.WithTTL(9))
			if err != nil {
				fmt.Println(err)
				continue
			}

			e := concurrency.NewElection(s, common.BuildCalaConsistencyMasterKey())
			ctx, _ := context.WithTimeout(context.TODO(), time.Duration(app.GetConfig().Deploy.Timeout)*time.Second)
			err = e.Campaign(ctx, app.GetIP())
			if err != nil {
				switch {
				case err == context.Canceled:
					return
				default:
					time.Sleep(time.Second * 5)
					continue
				}
			}
			fmt.Println("new calc leader")
			if err := app.CalcAgentDataConsistency(); err != nil {
				wlog.Error("failed to calc agent data consistency", zap.Error(err))
			}
		}
	})
}

func startTemporaryTaskWorker(app *app) {
	app.Go(func() {
		for {
			// 同一时间只需要有一个service的Loop运行即可
			s, err := concurrency.NewSession(app.etcd.Client(), concurrency.WithTTL(9))
			if err != nil {
				fmt.Println(err)
				continue
			}
			e := concurrency.NewElection(s, common.BuildTemporaryMasterKey())
			ctx, _ := context.WithTimeout(context.TODO(), time.Duration(app.GetConfig().Deploy.Timeout)*time.Second)
			err = e.Campaign(ctx, app.GetIP())
			if err != nil {
				switch {
				case err == context.Canceled:
					return
				default:
					time.Sleep(time.Second * 5)
					continue
				}
			}
			fmt.Println("new temporary scheduler leader")

			c := time.NewTicker(time.Minute)
			for {
				select {
				case <-app.closeCh:
					return
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
					}
				}
			}
		}
	})
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

func (a *app) CheckPermissions(projectID, uid int64) error {
	// 首先确认操作的用户是否为该项目的管理员
	isAdmin, err := a.IsAdmin(uid)
	if err != nil {
		return err
	}

	if !isAdmin {
		if exist, err := a.CheckUserIsInProject(projectID, uid); err != nil {
			return err
		} else if !exist {
			return errors.ErrProjectNotExist
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

	conn, err := a.connPool.Get("agent_" + host)
	if err != nil {
		return errors.NewError(http.StatusInternalServerError, "获取agent连接失败").WithLog(err.Error())
	}
	defer func() {
		err := a.connPool.Put("agent_"+host, conn)
		fmt.Println("put back", "agent_"+host, err)
	}()

	client := cronpb.NewAgentClient(conn.Conn())
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
