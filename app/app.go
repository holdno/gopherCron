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
	"github.com/holdno/gopherCron/jwt"
	"github.com/holdno/gopherCron/pkg/etcd"
	"github.com/holdno/gopherCron/pkg/logger"
	"github.com/holdno/gopherCron/pkg/panicgroup"
	"github.com/holdno/gopherCron/pkg/store/sqlStore"
	"github.com/holdno/gopherCron/pkg/warning"
	"github.com/holdno/gopherCron/protocol"
	"github.com/holdno/gopherCron/utils"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/clientv3/concurrency"
	recipe "github.com/coreos/etcd/contrib/recipes"
	"github.com/gin-gonic/gin"
	towerconfig "github.com/holdno/firetower/config"
	"github.com/holdno/firetower/service/tower"
	"github.com/holdno/gocommons/selection"
	"github.com/holdno/rego"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

type App interface {
	Log() *logrus.Logger
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

	GetEtcdClient() *clientv3.Client
	Go(f func())
	warning.Warner
}

func GetApp(c *gin.Context) App {
	return c.MustGet(common.APP_KEY).(App)
}

type app struct {
	clusterID  int64
	httpClient *http.Client
	store      sqlStore.SqlStore
	logger     *logrus.Logger
	etcd       protocol.EtcdManager
	closeCh    chan struct{}
	isClose    bool
	localip    string

	workflowRunner  *workflowRunner
	messageChan     chan PublishData
	messageCounter  int64
	taskResultQueue *recipe.Queue
	taskResultChan  chan string

	cfg *config.ServiceConfig

	panicgroup.PanicGroup
	protocol.CommonInterface
	warning.Warner

	firetower tower.Manager
	pusher    *SystemPusher
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

func WithFiretower() AppOptions {
	return func(a *app) {
		var err error
		a.firetower, err = tower.Setup(towerconfig.FireTowerConfig{
			ChanLens:    1000,
			Heartbeat:   30,
			ServiceMode: towerconfig.SingleMode,
			Bucket: towerconfig.BucketConfig{
				Num:              4,
				CentralChanCount: 100000,
				BuffChanCount:    1000,
				ConsumerNum:      1,
			},
		})
		if err != nil {
			panic(err)
		}

		a.pusher = &SystemPusher{
			clientID: "system",
		}

		a.messageChan = make(chan PublishData, 1000)
		a.Go(func() {
			for {
				res := <-a.messageChan
				a.publishEventToWebClient(res)
			}
		})
	}
}

func NewApp(configPath string, opts ...AppOptions) App {
	var err error

	conf := config.InitServiceConfig(configPath)

	app := new(app)
	app.cfg = conf
	app.logger = logger.MustSetup(conf.LogLevel)
	app.store = sqlStore.MustSetup(conf.Mysql, app.logger, true)
	app.httpClient = &http.Client{
		Timeout: time.Second * 5,
	}

	app.logger.Info("start to connect etcd ...")
	if app.etcd, err = etcd.Connect(conf.Etcd); err != nil {
		panic(err)
	}

	{
		app.taskResultChan = make(chan string, 1000)
		app.taskResultQueue = recipe.NewQueue(app.etcd.Client(), common.BuildTaskResultQueuePrefixKey())
	}

	app.logger.Info("connected to etcd")
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
			app.logger.WithFields(logrus.Fields{
				"desc":         reserr,
				"source_error": err,
			}).Error("panicgroup: failed to warning panic error")
		}
	})
	for _, opt := range opts {
		opt(app)
	}

	if app.Warner == nil {
		app.Warner = warning.NewDefaultWarner(app.logger)
	}

	if app.localip, err = utils.GetLocalIP(); err != nil {
		app.logger.Error("failed to get local ip")
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
					app.logger.Error(err.Error())
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
			case executeResult := <-app.taskResultChan:
				var execResult protocol.TaskFinishedQueueContent
				_ = json.Unmarshal([]byte(executeResult), &execResult)
				switch execResult.Version {
				case protocol.QueueItemV1:
					var result protocol.TaskFinishedQueueItemV1
					_ = json.Unmarshal(execResult.Data, &result)
					go func(result protocol.TaskFinishedQueueItemV1) {
						defer func() {
							if r := recover(); r != nil {
								app.Warning(warning.WarningData{
									Type:      warning.WarningTypeSystem,
									Data:      fmt.Sprintf("任务结果消费出错 panic, %v", r),
									TaskName:  result.TaskID,
									ProjectID: result.ProjectID,
								})
							}
						}()

						if result.TaskType == common.WorkflowPlan {
							if err := app.workflowRunner.handleTaskResultV1(result); err != nil {
								if err = app.taskResultQueue.Enqueue(executeResult); err != nil {
									app.Warning(warning.WarningData{
										Type:      warning.WarningTypeSystem,
										Data:      fmt.Sprintf("任务结果消费出错，重新入队失败, %s", err.Error()),
										TaskName:  result.TaskID,
										ProjectID: result.ProjectID,
									})
								}
							}
						} else {
							// normal task
							app.PublishMessage(messageTaskStatusChanged(result.ProjectID, result.TaskID, result.TmpID, result.Status))
						}
					}(result)
				}
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

	// app.Go(func() {
	// 	for {
	// 		// 同一时间只需要有一个service的Loop运行即可
	// 		s, err := concurrency.NewSession(app.etcd.Client(), concurrency.WithTTL(9))
	// 		if err != nil {
	// 			fmt.Println(err)
	// 			continue
	// 		}

	// 		e := concurrency.NewElection(s, common.BuildWorkflowMasterKey())
	// 		ctx, _ := utils.GetContextWithTimeout()
	// 		err = e.Campaign(ctx, app.GetIP())
	// 		if err != nil {
	// 			switch {
	// 			case err == context.Canceled:
	// 				return
	// 			default:
	// 				time.Sleep(time.Second * 5)
	// 				continue
	// 			}
	// 		}
	// 		fmt.Println("new workflow leader")
	// 		list, _, err := app.GetWorkflowList(common.GetWorkflowListOptions{}, 1, 100000)
	// 		if err != nil {
	// 			app.logger.Error("failed to refresh workflow list", err.Error())
	// 			continue
	// 		}

	// 		for _, v := range list {
	// 			workflow.SetPlan(v)
	// 		}
	// 		workflow.Loop()
	// 	}
	// })
	app.workflowRunner = workflow

	startTemporaryTaskWorker(app)

	app.Go(func() {
		for {
			err := rego.Retry(func() error {
				result, err := app.taskResultQueue.Dequeue()
				if err != nil {
					return err
				}
				app.taskResultChan <- result
				return nil
			}, rego.WithTimes(5), rego.WithPeriod(time.Second))
			if err != nil {
				// todo warning?
			}
		}
	})

	return app
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
			ctx, _ := utils.GetContextWithTimeout()
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
					app.logger.Error("failed to refresh temporary task list", err.Error())
					continue
				}

				for _, v := range list {
					if err = app.TemporaryTaskSchedule(*v); err != nil {
						app.logger.Error("temporary task worker: failed to schedule task", err.Error())
					}
				}
			}
		}
	})
}

func (a *app) Log() *logrus.Logger {
	return a.logger
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
