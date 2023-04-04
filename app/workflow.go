package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/config"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/pkg/cronpb"
	"github.com/holdno/gopherCron/pkg/warning"
	"github.com/holdno/gopherCron/protocol"
	"github.com/holdno/gopherCron/utils"

	"github.com/gorhill/cronexpr"
	"github.com/holdno/gocommons/selection"
	"github.com/holdno/rego"
	"github.com/jinzhu/gorm"
	"github.com/spacegrower/watermelon/infra/wlog"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"go.uber.org/zap"
)

func (a *app) GetWorkflowRelevanceUsers(workflowID int64) ([]common.UserWorkflowRelevance, error) {
	list, err := a.store.UserWorkflowRelevance().GetWorkflowUsers(workflowID)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, errors.NewError(http.StatusInternalServerError, "获取workflow用户列表失败").WithLog(err.Error())
	}
	return list, nil
}

func (a *app) WorkflowRemoveUser(workflowID int64, userID int64) error {
	if userID == 1 {
		return errors.NewError(http.StatusBadRequest, "无法移除超级管理员的权限")
	}
	workflow, err := a.store.Workflow().GetOne(workflowID)
	if err != nil && err != gorm.ErrRecordNotFound {
		return errors.NewError(http.StatusInternalServerError, "获取workflow信息失败").WithLog(err.Error())
	}
	if workflow == nil {
		return errors.NewError(http.StatusNotFound, "workflow不存在")
	}

	l := a.etcd.GetLocker(common.BuildWorkflowAddUserLockKey(workflowID, userID))
	if err = l.TryLock(); err != nil {
		return errors.NewError(http.StatusBadRequest, "请勿频繁操作").WithLog(err.Error())
	}
	defer l.Unlock()

	exist, err := a.store.UserWorkflowRelevance().GetUserWorkflowRelevance(userID, workflowID)
	if err != nil && err != gorm.ErrRecordNotFound {
		return errors.NewError(http.StatusInternalServerError, "检测用户权限失败").WithLog(err.Error())
	}
	if exist == nil {
		return errors.NewError(http.StatusBadRequest, "用户不存在")
	}

	err = a.store.UserWorkflowRelevance().DeleteUserWorkflowRelevance(nil, workflowID, userID)
	if err != nil {
		return errors.NewError(http.StatusInternalServerError, "移除用户失败").WithLog(err.Error())
	}
	return nil
}

func (a *app) WorkflowAddUser(workflowID int64, userID int64) error {
	workflow, err := a.store.Workflow().GetOne(workflowID)
	if err != nil && err != gorm.ErrRecordNotFound {
		return errors.NewError(http.StatusInternalServerError, "获取workflow信息失败").WithLog(err.Error())
	}
	if workflow == nil {
		return errors.NewError(http.StatusNotFound, "workflow不存在")
	}

	l := a.etcd.GetLocker(common.BuildWorkflowAddUserLockKey(workflowID, userID))
	if err = l.TryLock(); err != nil {
		return errors.NewError(http.StatusBadRequest, "请勿频繁操作").WithLog(err.Error())
	}
	defer l.Unlock()

	exist, err := a.store.UserWorkflowRelevance().GetUserWorkflowRelevance(userID, workflowID)
	if err != nil && err != gorm.ErrRecordNotFound {
		return errors.NewError(http.StatusInternalServerError, "检测用户权限失败").WithLog(err.Error())
	}

	if exist != nil {
		return errors.NewError(http.StatusBadRequest, "用户已存在")
	}

	err = a.store.UserWorkflowRelevance().Create(nil, &common.UserWorkflowRelevance{
		WorkflowID: workflowID,
		UserID:     userID,
		CreateTime: time.Now().Unix(),
	})
	if err != nil {
		return errors.NewError(http.StatusInternalServerError, "增加用户失败").WithLog(err.Error())
	}
	return nil
}

func (a *app) CreateWorkflow(userID int64, data common.Workflow) error {

	var (
		tx  = a.store.BeginTx()
		err error
	)

	if _, err = cronexpr.Parse(data.Cron); err != nil {
		return errors.NewError(errors.CodeInvalidArgument, "cron表达式校验失败: "+err.Error()).WithLog(err.Error())
	}

	defer func() {
		if r := recover(); r != nil && err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()
	if err = a.store.Workflow().Create(tx, &data); err != nil {
		return errors.NewError(errors.CodeInternalError, "创建workflow失败").WithLog(err.Error())
	}

	if err = a.store.UserWorkflowRelevance().Create(tx, &common.UserWorkflowRelevance{
		UserID:     userID,
		WorkflowID: data.ID,
		CreateTime: time.Now().Unix(),
	}); err != nil {
		return errors.NewError(http.StatusInternalServerError, "创建workflow用户关联关系失败").WithLog(err.Error())
	}

	err = a.workflowRunner.SetPlan(data)
	return err
}

func checkUserWorkflowPermission(checkFunc interface {
	GetUserWorkflowRelevance(userID int64, workflowID int64) (*common.UserWorkflowRelevance, error)
}, userID, workflowID int64) error {
	if userID == 1 {
		return nil
	}
	exist, err := checkFunc.GetUserWorkflowRelevance(userID, workflowID)
	if err != nil && err != gorm.ErrRecordNotFound {
		return errors.NewError(http.StatusInternalServerError, "检测用户权限失败").WithLog(err.Error())
	}
	if exist == nil {
		return errors.NewError(http.StatusForbidden, "无权编辑该workflow")
	}
	return nil
}

func (a *app) CreateWorkflowTask(userID int64, data common.WorkflowTask) error {
	var err error
	if err = a.CheckPermissions(data.ProjectID, userID); err != nil {
		return err
	}

	if data.TaskID == "" {
		data.TaskID = utils.GetStrID()
	}

	if err = a.store.WorkflowTask().Create(nil, &data); err != nil {
		return errors.NewError(http.StatusInternalServerError, "创建workflow任务失败").WithLog(err.Error())
	}

	return nil
}

func (a *app) UpdateWorkflowTask(userID int64, data common.WorkflowTask) error {
	if err := a.CheckPermissions(data.ProjectID, userID); err != nil {
		return err
	}

	if err := a.store.WorkflowTask().Save(nil, &data); err != nil {
		return errors.NewError(http.StatusInternalServerError, "更新workflow任务失败").WithLog(err.Error())
	}
	return nil
}

func (a *app) DeleteWorkflowTask(userID, projectID int64, taskID string) error {
	if err := a.CheckPermissions(projectID, userID); err != nil {
		return err
	}

	if taskID == "" {
		return errors.NewError(http.StatusBadRequest, "任务id不能为空")
	}

	// 检测任务是否被workflow依赖
	plans, err := a.store.WorkflowSchedulePlan().GetTaskWorkflowIDs([]string{common.BuildWorkflowTaskIndex(projectID, taskID)})
	if err != nil && err != gorm.ErrRecordNotFound {
		return errors.NewError(http.StatusInternalServerError, "检查workflow任务依赖失败").WithLog(err.Error())
	}
	if len(plans) > 0 {
		return errors.NewError(http.StatusBadRequest, "该任务已被workflow引用，请先移除引用关系")
	}

	if err = a.store.WorkflowTask().Delete(nil, projectID, taskID); err != nil {
		return errors.NewError(http.StatusInternalServerError, "删除workflow任务失败").WithLog(err.Error())
	}

	return nil
}

type CreateWorkflowSchedulePlanArgs struct {
	WorkflowTaskInfo
	Dependencies []WorkflowTaskInfo
}

func (a *app) CreateWorkflowSchedulePlan(userID, workflowID int64, taskList []CreateWorkflowSchedulePlanArgs) error {
	err := checkUserWorkflowPermission(a.store.UserWorkflowRelevance(), userID, workflowID)
	if err != nil {
		return err
	}

	plan := a.workflowRunner.GetPlan(workflowID)
	if plan == nil {
		// 分布式场景下每台服务器缓存的workflow可能会不一致，如果在服务启动后新建workflow，则会导致其他服务器没有该plan的信息
		return nil
	}
	running, err := plan.IsRunning()
	if err != nil {
		return err
	}
	if running {
		return errors.NewError(http.StatusBadRequest, "当前workflow正在运行中，请稍后再试")
	}

	workflowTaskList, err := a.store.WorkflowSchedulePlan().GetList(workflowID)
	if err != nil && err != gorm.ErrRecordNotFound {
		return errors.NewError(errors.CodeInternalError, "创建workflow 任务信息失败").WithLog(err.Error())
	}

	var needToDelete []int64
	for _, v := range workflowTaskList {
		needToDelete = append(needToDelete, v.ID)
	}
	var needToCreate []common.WorkflowSchedulePlan
	for _, v := range taskList {
		if len(v.Dependencies) > 0 {
			for _, vv := range v.Dependencies {
				needToCreate = append(needToCreate, common.WorkflowSchedulePlan{
					WorkflowID:          workflowID,
					TaskID:              v.TaskID,
					ProjectID:           v.ProjectID,
					DependencyTaskID:    vv.TaskID,
					DependencyProjectID: vv.ProjectID,
					CreateTime:          time.Now().Unix(),
				})
			}
		} else {
			needToCreate = append(needToCreate, common.WorkflowSchedulePlan{
				WorkflowID:          workflowID,
				TaskID:              v.TaskID,
				ProjectID:           v.ProjectID,
				DependencyTaskID:    "",
				DependencyProjectID: 0,
				CreateTime:          time.Now().Unix(),
			})
		}
	}

	tasks := make(map[string]map[string]bool)
	for _, v := range needToCreate {
		checkKey := common.BuildWorkflowTaskIndex(v.ProjectID, v.TaskID)
		sons, exist := tasks[checkKey]
		if !exist {
			tasks[checkKey] = make(map[string]bool)
			sons = tasks[checkKey]
		}
		sonkey := common.BuildWorkflowTaskIndex(v.DependencyProjectID, v.DependencyTaskID)
		if (v.DependencyProjectID == 0 && len(sons) > 0) || (sons[sonkey] || sons["0_"]) {
			return errors.NewError(http.StatusBadRequest, fmt.Sprintf("workflow出现重复执行任务, projectid: %d, taskid: %s", v.ProjectID, v.TaskID))
		}
		sons[sonkey] = true
	}

	for _, v := range needToCreate {
		if v.DependencyTaskID == "" {
			continue
		}
		if _, exist := tasks[common.BuildWorkflowTaskIndex(v.DependencyProjectID, v.DependencyTaskID)]; !exist {
			return errors.NewError(http.StatusBadRequest, fmt.Sprintf("缺少依赖任务, projectid: %d, taskid: %s", v.DependencyProjectID, v.DependencyTaskID))
		}

		exist, err := a.GetWorkflowTask(v.ProjectID, v.TaskID)
		if err != nil {
			return err
		}
		if exist == nil {
			return errors.NewError(http.StatusBadRequest, fmt.Sprintf("任务不存在, projectid: %d, taskid: %s", v.ProjectID, v.TaskID))
		}
	}

	tx := a.store.BeginTx()
	defer func() {
		if r := recover(); r != nil || err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()
	if err = a.store.WorkflowSchedulePlan().DeleteList(tx, needToDelete); err != nil {
		return errors.NewError(errors.CodeInternalError, "创建workflow 任务信息失败, 解除任务关联失败").WithLog(err.Error())
	}

	for _, v := range needToCreate {
		if err = a.store.WorkflowSchedulePlan().Create(tx, &v); err != nil {
			return errors.NewError(errors.CodeInternalError, "创建workflow 任务信息失败, 创建任务关联关系失败").WithLog(err.Error())
		}
	}

	err = a.workflowRunner.SetPlan(plan.Workflow)
	return err
}

func disposeWorkflowTaskData(workflowTaskList []common.WorkflowSchedulePlan, task WorkflowTaskInfo, dependencies []WorkflowTaskInfo) ([]int64, []common.WorkflowSchedulePlan) {
	dependMap := make(map[WorkflowTaskInfo]bool)
	for _, v := range dependencies {
		dependMap[v] = true
	}

	var needToDelete []int64
	var workflowID int64
	for _, v := range workflowTaskList {
		workflowID = v.WorkflowID
		key := WorkflowTaskInfo{
			TaskID:    v.DependencyTaskID,
			ProjectID: v.DependencyProjectID,
		}
		if dependMap[key] {
			// 删除已经存在的key
			delete(dependMap, key)
			continue
		}
		needToDelete = append(needToDelete, v.ID)
	}

	var needToCreate []common.WorkflowSchedulePlan
	for k := range dependMap {
		needToCreate = append(needToCreate, common.WorkflowSchedulePlan{
			WorkflowID:          workflowID,
			TaskID:              task.TaskID,
			ProjectID:           task.ProjectID,
			DependencyTaskID:    k.TaskID,
			DependencyProjectID: k.ProjectID,
			CreateTime:          time.Now().Unix(),
		})
	}

	if len(needToCreate) == 0 && len(workflowTaskList) == 0 {
		needToCreate = append(needToCreate, common.WorkflowSchedulePlan{
			WorkflowID:          workflowID,
			TaskID:              task.TaskID,
			ProjectID:           task.ProjectID,
			DependencyTaskID:    "",
			DependencyProjectID: 0,
			CreateTime:          time.Now().Unix(),
		})
	}

	return needToDelete, needToCreate
}

func (a *app) CreateWorkflowLog(workflowID int64, startTime, endTime int64, result string) error {
	err := a.store.WorkflowLog().Create(nil, &common.WorkflowLog{
		WorkflowID: workflowID,
		StartTime:  startTime,
		EndTime:    endTime,
		Result:     result,
		CreateTime: time.Now().Unix(),
	})
	if err != nil {
		return errors.NewError(http.StatusInternalServerError, "workflow任务日志入库失败").WithLog(err.Error())
	}
	return nil
}

func (a *app) GetWorkflowLogList(workflowID int64, page, pagesize uint64) ([]common.WorkflowLog, int, error) {
	opts := selection.NewSelector(selection.NewRequirement("workflow_id", selection.Equals, workflowID))
	list, err := a.store.WorkflowLog().GetList(opts,
		page, pagesize)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, 0, errors.NewError(http.StatusInternalServerError, "获取workflow任务执行结果失败").WithLog(err.Error())
	}

	total, err := a.store.WorkflowLog().GetTotal(opts)
	if err != nil {
		return nil, 0, errors.NewError(http.StatusInternalServerError, "获取workflow日志总记录数失败").WithLog(err.Error())
	}
	return list, total, nil
}

func (a *app) GetWorkflow(id int64) (*common.Workflow, error) {
	data, err := a.store.Workflow().GetOne(id)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, errors.NewError(http.StatusInternalServerError, "获取workflow信息失败").WithLog(err.Error())
	}

	return data, nil
}

func (a *app) GetWorkflowList(opts common.GetWorkflowListOptions, page, pagesize uint64) ([]common.Workflow, int, error) {
	// TODO get user workflow
	selector := selection.NewSelector()
	if len(opts.IDs) > 0 {
		selector.AddQuery(selection.NewRequirement("id", selection.In, opts.IDs))
	}
	if opts.Title != "" {
		selector.AddQuery(selection.NewRequirement("title", selection.Like, opts.Title))
	}
	list, err := a.store.Workflow().GetList(selector, page, pagesize)
	if err != nil {
		return nil, 0, errors.NewError(http.StatusInternalServerError, "获取workflow列表失败").WithLog(err.Error())
	}

	total, err := a.store.Workflow().GetTotal(selector)
	if err != nil {
		return nil, 0, errors.NewError(http.StatusInternalServerError, "获取workflow总记录数失败").WithLog(err.Error())
	}

	return list, total, nil
}

func (a *app) GetUserWorkflowPermission(userID, workflowID int64) error {
	ok, err := a.IsAdmin(userID)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	err = checkUserWorkflowPermission(a.store.UserWorkflowRelevance(), userID, workflowID)
	if err != nil {
		return err
	}
	return nil
}

func (a *app) GetWorkflowScheduleTasks(workflowID int64) ([]common.WorkflowSchedulePlan, error) {
	list, err := a.store.WorkflowSchedulePlan().GetList(workflowID)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, errors.NewError(http.StatusInternalServerError, "获取workflow任务列表失败").WithLog(err.Error())
	}

	return list, nil
}

func (a *app) ClearWorkflowLog(workflowID int64) error {
	err := a.store.WorkflowLog().Clear(nil, selection.NewSelector(selection.NewRequirement("workflow_id", selection.Equals, workflowID)))
	if err != nil {
		return errors.NewError(http.StatusInternalServerError, "清理workflow日志失败").WithLog(err.Error())
	}
	return nil
}

func (a *app) GetUserWorkflows(userID int64) ([]int64, error) {
	list, err := a.store.UserWorkflowRelevance().GetUserWorkflows(userID)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, errors.NewError(http.StatusInternalServerError, "获取角色下关联的workflow失败").WithLog(err.Error())
	}
	var result []int64
	for _, v := range list {
		result = append(result, v.WorkflowID)
	}
	return result, nil
}

func (a *app) UpdateWorkflow(userID int64, data common.Workflow) error {
	err := checkUserWorkflowPermission(a.store.UserWorkflowRelevance(), userID, data.ID)
	if err != nil {
		return err
	}

	tx := a.store.BeginTx()
	defer func() {
		if r := recover(); r != nil || err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()
	if err = a.store.Workflow().Update(tx, data); err != nil {
		return errors.NewError(http.StatusInternalServerError, "更新workflow失败").WithLog(err.Error())
	}

	// if data.Status == common.TASK_STATUS_START {

	// } else {
	// 	a.workflowRunner.DelPlan(data.ID)
	// }
	err = a.workflowRunner.SetPlan(data)
	return err
}

func (a *app) GetWorkflowTask(projectID int64, taskID string) (*common.WorkflowTask, error) {
	task, err := a.store.WorkflowTask().GetOne(projectID, taskID)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, errors.NewError(http.StatusInternalServerError, "获取workflow任务详情失败").WithLog(err.Error())
	}
	return task, nil
}

func (a *app) GetProjectWorkflowTask(projectID int64) ([]common.WorkflowTask, error) {
	list, err := a.store.WorkflowTask().GetList(projectID)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, errors.NewError(http.StatusInternalServerError, "获取项目下workflow任务列表失败").WithLog(err.Error())
	}
	return list, nil
}

func (a *app) GetMultiWorkflowTaskList(taskIDs []string) ([]common.WorkflowTask, error) {
	list, err := a.store.WorkflowTask().GetMultiList(taskIDs)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, errors.NewError(http.StatusInternalServerError, "获取workflow全部任务列表失败").WithLog(err.Error())
	}
	return list, nil
}

func (a *app) GetWorkflowAllTaskStates(workflowID int64) ([]*WorkflowTaskStates, error) {
	states, err := getWorkflowAllTaskStates(a.GetEtcdClient().KV, workflowID)
	if err != nil {
		return nil, errors.NewError(http.StatusInternalServerError, "获取任务详情失败")
	}
	return states, nil
}

func (a *app) DeleteWorkflow(userID int64, workflowID int64) error {
	err := checkUserWorkflowPermission(a.store.UserWorkflowRelevance(), userID, workflowID)
	if err != nil {
		return err
	}

	tx := a.store.BeginTx()
	defer func() {
		if r := recover(); r != nil || err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()
	if err = a.store.Workflow().Delete(tx, workflowID); err != nil {
		return errors.NewError(http.StatusInternalServerError, "删除workflow失败").WithLog(err.Error())
	}
	if err = a.store.WorkflowSchedulePlan().DeleteAllWorkflowSchedulePlan(tx, workflowID); err != nil {
		return errors.NewError(http.StatusInternalServerError, "删除workflow调度任务列表失败").WithLog(err.Error())
	}
	if err = a.store.UserWorkflowRelevance().DeleteWorkflowAllUserRelevance(tx, workflowID); err != nil {
		return errors.NewError(http.StatusInternalServerError, "删除workflow用户关联列表失败").WithLog(err.Error())
	}

	plan := a.workflowRunner.GetPlan(workflowID)
	if plan != nil {
		running, err := plan.IsRunning()
		if err != nil {
			return err
		}
		if running {
			if err = plan.Finished(ErrWorkflowKilled); err != nil {
				return err
			}
		}
	}

	a.workflowRunner.DelPlan(workflowID)
	return err
}

func (a *app) StartWorkflow(workflowID int64) error {
	plan := a.workflowRunner.GetPlan(workflowID)
	if plan == nil {
		return errors.NewError(http.StatusBadRequest, "该workflow不存在")
	}
	if err := a.workflowRunner.TryStartPlan(plan); err != nil {
		return err
	}
	return nil
}

func (a *app) KillWorkflow(workflowID int64) error {
	plan := a.workflowRunner.GetPlan(workflowID)
	if plan == nil {
		return errors.NewError(http.StatusBadRequest, "workflow不存在")
	}

	if err := plan.Finished(ErrWorkflowKilled); err != nil {
		return err
	}
	return nil
}

func (a *app) killWorkflowTasks(region string, killList []WorkflowTaskInfo) error {
	ctx, _ := utils.GetContextWithTimeout()

	for _, v := range killList {
		agents, err := a.FindAgents(region, v.ProjectID)
		if err != nil {
			return err
		}
		for _, a := range agents {
			if _, err = a.KillTask(ctx, &cronpb.KillTaskRequest{
				ProjectId: v.ProjectID,
				TaskId:    v.TaskID,
			}); err != nil {
				a.Close()
				return errors.NewError(http.StatusInternalServerError, "停止任务请求失败, agent: "+a.addr).WithLog(err.Error())
			}
			a.Close()
		}
	}
	return nil
}

type workflowRunner struct {
	etcd              *clientv3.Client
	app               *app
	plans             sync.Map
	planCounter       int64
	nextWorkflow      common.Workflow
	scheduleEventChan chan *common.TaskEvent

	processRecord sync.Map

	ctx        context.Context
	cancelFunc context.CancelFunc
	isClose    bool
}

func (r *workflowRunner) IsInProcess(taskID string) bool {
	_, ok := r.processRecord.Load(taskID)
	return ok
}

func (r *workflowRunner) InProcess(taskID string) bool {
	_, ok := r.processRecord.LoadOrStore(taskID, struct{}{})
	return ok
}

func (r *workflowRunner) ProcessDone(taskID string) {
	r.processRecord.Delete(taskID)
}

func NewWorkflowRunner(app *app, cli *clientv3.Client) (*workflowRunner, error) {
	ctx, cancel := context.WithCancel(context.Background())
	runner := &workflowRunner{
		app:               app,
		etcd:              app.GetEtcdClient(),
		ctx:               ctx,
		cancelFunc:        cancel,
		scheduleEventChan: make(chan *common.TaskEvent, 100),
	}

	list, _, err := app.GetWorkflowList(common.GetWorkflowListOptions{}, 1, 100000)
	if err != nil {
		return nil, err
	}

	for _, v := range list {
		runner.SetPlan(v)
	}

	return runner, nil
}

func (r *workflowRunner) Close() {
	if r.isClose {
		return
	}
	r.isClose = true
	r.cancelFunc()
}

type WorkflowPlan struct {
	runner    *workflowRunner
	Workflow  common.Workflow
	Expr      *cronexpr.Expression // 解析后的cron表达式
	NextTime  time.Time
	Tasks     map[WorkflowTaskInfo]*common.WorkflowTask
	TaskFlow  map[WorkflowTaskInfo][]WorkflowTaskInfo // map[任务][]依赖
	planState *PlanState

	LatestScheduleTime time.Time

	locker sync.Mutex
}

func (p *WorkflowPlan) Finished(withError error) error {
	p.locker.Lock()
	defer p.locker.Unlock()
	if err := p.RefreshStates(); err != nil {
		return err
	}
	if p.planState == nil {
		return nil
	}

	p.planState.Status = common.TASK_STATUS_DONE_V2
	if withError != nil {
		p.planState.Status = common.TASK_STATUS_FAIL_V2
	}

	states, err := getWorkflowAllTaskStates(p.runner.etcd.KV, p.Workflow.ID)
	if err != nil {
		return err
	}

	failedReason := strings.Builder{}
	var killList []WorkflowTaskInfo
	for _, v := range states {
		if v.CurrentStatus == common.TASK_STATUS_FAIL_V2 {
			p.planState.Status = common.TASK_STATUS_FAIL_V2
			// get task info
			taskDetail, err := p.runner.app.GetWorkflowTask(v.ProjectID, v.TaskID)
			if err != nil {
				// log
				return err
			}
			failedReason.WriteString(taskDetail.TaskName)
			failedReason.WriteString(" 任务执行失败")
		} else if v.CurrentStatus == common.TASK_STATUS_STARTING_V2 {
			killList = append(killList, WorkflowTaskInfo{
				ProjectID: v.ProjectID,
				TaskID:    v.TaskID,
			})

			failedReason.WriteString("任务启动失败")
		} else if v.CurrentStatus == common.TASK_STATUS_RUNNING_V2 {
			killList = append(killList, WorkflowTaskInfo{
				ProjectID: v.ProjectID,
				TaskID:    v.TaskID,
			})

			failedReason.WriteString("任务执行失败")
		}
	}
	if withError != nil {
		if failedReason.Len() > 0 {
			failedReason.WriteString("，")
		}
		failedReason.WriteString(withError.Error())
	}

	finalState := *p.planState
	finalState.Reason = failedReason.String()
	finalState.Records = states
	finalState.EndTime = time.Now().Unix()

	p.runner.app.PublishMessage(messageWorkflowStatusChanged(p.Workflow.ID, finalState.Status))

	result, err := json.Marshal(finalState)
	if err != nil {
		return err
	}

	// 一定要先清理workflow相关的key，这样在kill过程中，任务就不会写入新的状态
	err = rego.Retry(func() error {
		return clearWorkflowKeys(p.runner.etcd.KV, p.Workflow.ID)
	}, rego.WithPeriod(time.Second), rego.WithTimes(3), rego.WithLatestError())
	if err != nil {
		p.runner.app.Warning(warning.WarningData{
			Type:     warning.WarningTypeSystem,
			TaskName: p.Workflow.Title,
			Data:     fmt.Sprintf("workflow: %s, 运行结束时清除运行状态失败, 失败原因: %s", p.Workflow.Title, err.Error()),
		})
		return err
	}

	if withError != nil {
		err = rego.Retry(func() error {
			return p.runner.app.killWorkflowTasks(p.runner.app.GetConfig().Micro.Region, killList)
		}, rego.WithPeriod(time.Second), rego.WithTimes(3), rego.WithLatestError())
		if err != nil {
			p.runner.app.Warning(warning.WarningData{
				Type:     warning.WarningTypeSystem,
				TaskName: p.Workflow.Title,
				Data:     fmt.Sprintf("workflow: %s, 运行结束时强杀任务失败, 失败原因: %s", p.Workflow.Title, err.Error()),
			})
			return err
		}
	}

	if err = p.runner.app.CreateWorkflowLog(finalState.WorkflowID, finalState.StartTime, finalState.EndTime, string(result)); err != nil {
		p.runner.app.Warning(warning.WarningData{
			Type:     warning.WarningTypeSystem,
			TaskName: p.Workflow.Title,
			Data:     fmt.Sprintf("workflow: %s, 执行结果入库失败, 失败原因: %s", p.Workflow.Title, err.Error()),
		})
		return err
	}

	return nil
}

type taskFlowItem struct {
	Task WorkflowTaskInfo
	Deps []WorkflowTaskInfo
}

func (a *workflowRunner) scheduleWorkflowPlan(plan *WorkflowPlan) error {
	needToScheduleTasks, finished, err := plan.CanSchedule(a)
	if err != nil && err != ErrWorkflowFailed {
		return err
	}

	if finished {
		plan.Finished(err)
		return nil
	}

	for _, v := range needToScheduleTasks {
		task := plan.Tasks[v]
		a.scheduleEventChan <- common.BuildTaskEvent(common.TASK_EVENT_WORKFLOW_SCHEDULE, &common.TaskInfo{
			TaskID:    task.TaskID,
			Name:      task.TaskName,
			ProjectID: task.ProjectID,
			Command:   task.Command,
			Remark:    task.Remark,
			Timeout:   task.Timeout,
			Noseize:   task.Noseize,
			FlowInfo: &common.WorkflowInfo{
				WorkflowID: plan.Workflow.ID,
			},
		})
	}
	return nil
}

func (a *workflowRunner) TryStartPlan(plan *WorkflowPlan) error {
	// 获取当前plan是否在运行中
	// TODO lock
	running, err := plan.IsRunning()
	if err != nil || running {
		// TODO latest workflow not compalete
		return err
	}

	// 重置任务依赖
	if err := plan.RefreshPlanTasks(); err != nil {
		return err
	}

	if err = plan.SetRunning(); err != nil {
		return err
	}

	return a.scheduleWorkflowPlan(plan)
}

func (s *WorkflowPlan) RefreshPlanTasks() error {
	s.locker.Lock()
	defer s.locker.Unlock()

	tasks, err := s.runner.app.GetWorkflowScheduleTasks(s.Workflow.ID)
	if err != nil {
		return err
	}

	depsMap := make(map[WorkflowTaskInfo][]WorkflowTaskInfo)
	tasksMap := make(map[WorkflowTaskInfo]*common.WorkflowTask)
	for _, v := range tasks {
		key := WorkflowTaskInfo{
			TaskID:    v.TaskID,
			ProjectID: v.ProjectID,
		}
		depsMap[key] = append(depsMap[key], WorkflowTaskInfo{
			TaskID:    v.DependencyTaskID,
			ProjectID: v.DependencyProjectID,
		})

		if _, exist := tasksMap[key]; !exist {
			tasksMap[key], err = s.runner.app.GetWorkflowTask(key.ProjectID, key.TaskID)
			if err != nil {
				return err
			}
		}
	}

	s.TaskFlow = depsMap
	s.Tasks = tasksMap
	return nil
}

var (
	ErrWorkflowFailed    = fmt.Errorf("workflow任务失败")
	ErrWorkflowKilled    = fmt.Errorf("人工停止workflow")
	ErrWorkflowInProcess = fmt.Errorf("workflow in process")
)

// CanSchedule 判断下一步可调度的任务
func (s *WorkflowPlan) CanSchedule(runner *workflowRunner) ([]WorkflowTaskInfo, bool, error) {
	if !s.locker.TryLock() {
		return nil, false, ErrWorkflowInProcess
	}
	defer s.locker.Unlock()

	var (
		readys        []WorkflowTaskInfo
		taskStatesMap      = make(map[WorkflowTaskInfo]*WorkflowTaskStates)
		finished      bool = true
	)

	states, err := getWorkflowTasksStates(s.runner.etcd.KV, common.BuildWorkflowTaskStatusKeyPrefix(s.Workflow.ID))
	if err != nil {
		return nil, false, err
	}

	for _, v := range states {
		taskStatesMap[WorkflowTaskInfo{v.ProjectID, v.TaskID}] = v
	}

	for task, deps := range s.TaskFlow {
		taskStates, exist := taskStatesMap[WorkflowTaskInfo{task.ProjectID, task.TaskID}]
		if exist && taskStates.CurrentStatus == common.TASK_STATUS_DONE_V2 {
			continue
		}

		// 检查依赖的任务是否都已结束
		ok := true
		for _, check := range deps {
			if check.TaskID != "" {
				states := taskStatesMap[check]
				if states == nil || states.CurrentStatus != common.TASK_STATUS_DONE_V2 {
					ok = false
					break
				}
			}
		}
		if !ok { // 上游还未跑完
			finished = false
			continue
		}

		if taskStates == nil {
			taskStates = &WorkflowTaskStates{
				CurrentStatus: common.TASK_STATUS_NOT_RUNNING_V2,
			}
		}

		afterDebounce := func() bool {
			if len(taskStates.ScheduleRecords) > 0 {
				return taskStates.ScheduleRecords[len(taskStates.ScheduleRecords)-1].EventTime <= time.Now().Add(-time.Second*(time.Duration(config.GetServiceConfig().Deploy.Timeout)+2)).Unix()
			}
			return true
		}

		switch taskStates.CurrentStatus {
		case common.TASK_STATUS_RUNNING_V2:
			finished = false

			if !afterDebounce() {
				continue
			}
			// 1.可以通过注册中心感知节点下线，如果节点有下线记录，则在此处调用agent check任务是否运行中
			// 2.看锁是否存在，若存在则任务执行中
			locker := s.runner.app.GetTaskLocker(&common.TaskInfo{TaskID: task.TaskID, ProjectID: task.ProjectID})
			exist, err := locker.LockExist()
			if err != nil {
				wlog.Error("failed to get lock status", zap.String("task_id", task.TaskID), zap.Int64("project_id", task.ProjectID), zap.String("method", "locker.LockExist"))
				continue
			}
			if exist {
				continue
			}

			// if the task lock detection fails, perform a kill operation as a fallback
			err = s.runner.app.killWorkflowTasks(s.runner.app.GetConfig().Micro.Region, []WorkflowTaskInfo{
				{
					ProjectID: task.ProjectID,
					TaskID:    task.TaskID,
				},
			})
			if err != nil {
				wlog.Error("workflow kill fallback failed", zap.String("task_id", task.TaskID), zap.Int64("project_id", task.ProjectID), zap.String("method", "killWorkflowTasks"))
				continue
			}

			// running状态 但是没有ack key
			// 有两种情况，一种是done状态还在队列中没有被workflow消费，另一种是agent直接断电，导致agent任务异常终止，没有上报任何状态
			// 添加re-run标记，待下一次尝试调度时重新拉起任务
			_, err = concurrency.NewSTM(s.runner.etcd, func(stm concurrency.STM) error {
				task := s.Tasks[task]
				if err := setWorkflowTaskNotRunning(stm, WorkflowRunningTaskInfo{
					TaskID:     task.TaskID,
					ProjectID:  task.ProjectID,
					TaskName:   task.TaskName,
					TmpID:      taskStates.GetLatestScheduleRecord().TmpID,
					WorkflowID: s.Workflow.ID,
				}, "waiting re-run"); err != nil {
					wlog.Error("mark the task as failed and in need of retry when it fails", zap.Int64("workflow_id", s.Workflow.ID),
						zap.String("task_id", task.TaskID), zap.Int64("project_id", task.ProjectID), zap.String("task_name", task.TaskName))
					return err
				}
				return nil
			})

		case common.TASK_STATUS_FAIL_V2:
			// 判断是否已经重复跑3次
			if taskStates.ScheduleCount >= common.WORKFLOW_SCHEDULE_LIMIT {
				return nil, true, ErrWorkflowFailed
			}
			fallthrough
		case common.TASK_STATUS_NOT_RUNNING_V2:
			finished = false
			if len(taskStates.ScheduleRecords) > 1 && !afterDebounce() {
				continue
			}

			readys = append(readys, task)
		case common.TASK_STATUS_STARTING_V2: // 异常补救
			if taskStates.ScheduleCount >= common.WORKFLOW_SCHEDULE_LIMIT {
				return nil, true, ErrWorkflowFailed
			}
			// 任务启动的超时间隔内也先不处理
			if runner.IsInProcess(task.TaskID) || (len(taskStates.ScheduleRecords) > 0 &&
				taskStates.ScheduleRecords[len(taskStates.ScheduleRecords)-1].Status == common.TASK_STATUS_STARTING_V2 &&
				!afterDebounce()) {
				finished = false
				continue
			}

			finished = false
			readys = append(readys, task)
		default:
		}
	}

	if finished {
		return nil, finished, nil
	}

	return readys, finished, nil
}

type WorkflowTaskInfo struct {
	ProjectID int64  `json:"project_id"`
	TaskID    string `json:"task_id"`
}

func inverseGraph(graph map[WorkflowTaskInfo][]WorkflowTaskInfo) (igraph map[WorkflowTaskInfo][]WorkflowTaskInfo) {
	igraph = make(map[WorkflowTaskInfo][]WorkflowTaskInfo)
	for node, outcomes := range graph {
		for _, outcome := range outcomes {
			igraph[outcome] = append(igraph[outcome], node)
		}
		if _, existed := igraph[node]; !existed {
			igraph[node] = make([]WorkflowTaskInfo, 0)
		}
	}
	return igraph
}

func (a *workflowRunner) GetPlan(id int64) *WorkflowPlan {
	data, exist := a.plans.Load(id)
	if !exist {
		return nil
	}
	return data.(*WorkflowPlan)
}

func (a *workflowRunner) SetPlan(data common.Workflow) error {
	plan := a.GetPlan(data.ID)
	if plan == nil {
		plan = &WorkflowPlan{
			runner:   a,
			Workflow: data,
			Tasks:    make(map[WorkflowTaskInfo]*common.WorkflowTask),
			TaskFlow: make(map[WorkflowTaskInfo][]WorkflowTaskInfo),
		}
		atomic.AddInt64(&a.planCounter, 1)
	} else {
		plan.Workflow = data
	}

	if err := plan.RefreshStates(); err != nil {
		return err
	}

	if err := plan.RefreshPlanTasks(); err != nil {
		return err
	}

	expr, err := cronexpr.Parse(data.Cron)
	if err != nil {
		return err
	}

	plan.Expr = expr
	plan.NextTime = expr.Next(time.Now())
	a.plans.Store(data.ID, plan)
	return nil
}

func (a *workflowRunner) DelPlan(id int64) {
	atomic.AddInt64(&a.planCounter, -1)
	a.plans.Delete(id)
}

func (a *workflowRunner) PlanCount() int64 {
	return atomic.LoadInt64(&a.planCounter)
}

func (a *workflowRunner) PlanRange(f func(key int64, value *WorkflowPlan) bool) {
	a.plans.Range(func(key, value interface{}) bool {
		f(key.(int64), value.(*WorkflowPlan))
		return true
	})
}

func (a *workflowRunner) TrySchedule() time.Duration {
	var (
		now      = time.Now()
		nearTime *time.Time
	)

	// 遍历所有任务
	a.PlanRange(func(workflowID int64, plan *WorkflowPlan) bool {
		// 如果plan没有开启运行的话，直接跳过
		if plan.Workflow.Status != common.TASK_STATUS_RUNNING {
			return true
		}
		// 如果调度时间是在现在或之前再或者为临时调度任务
		if plan.NextTime.Before(now) || plan.NextTime.Equal(now) {
			// 尝试执行任务
			// 因为可能上一次任务还没执行结束
			if err := a.TryStartPlan(plan); err != nil {
				fmt.Println("执行workflow失败", err.Error())
			}
			plan.NextTime = plan.Expr.Next(now) // 更新下一次执行时间
		}

		// 获取下一个要执行任务的时间
		if nearTime == nil || plan.NextTime.Before(*nearTime) {
			nearTime = &plan.NextTime
		}

		return true
	})

	if nearTime == nil {
		return time.Second
	}
	// 下次调度时间 (最近要执行的任务调度时间 - 当前时间)
	return (*nearTime).Sub(now)
}

// GetWorkflowState 获取workflow当前状态，未运行时状态为空
func (a *app) GetWorkflowState(workflowID int64) (*PlanState, error) {
	plan := a.workflowRunner.GetPlan(workflowID)
	if plan == nil {
		return nil, nil
	}
	err := plan.RefreshStates()
	if err != nil {
		return nil, errors.NewError(http.StatusInternalServerError, "获取workflow状态失败").WithLog(err.Error())
	}
	return plan.planState, nil
}

func (a *workflowRunner) Loop() {
	var (
		taskEvent     *common.TaskEvent
		scheduleAfter time.Duration
		scheduleTimer *time.Timer
		daemonTimer   *time.Ticker
	)

	scheduleAfter = a.TrySchedule()

	// 调度定时器
	scheduleTimer = time.NewTimer(scheduleAfter)
	daemonTimer = time.NewTicker(time.Second * 10)

	fmt.Printf("start workflow, next schedule after %d second\n", scheduleAfter/time.Second)

	for {
		select {
		case taskEvent = <-a.scheduleEventChan:
			// 对内存中的任务进行增删改查
			go a.handleTaskEvent(taskEvent)
		case <-daemonTimer.C:
			// 每10s
			a.PlanRange(func(key int64, value *WorkflowPlan) bool {
				if ok, _ := value.IsRunning(); ok {
					if value.LatestScheduleTime.Before(time.Now().Add(-time.Second * 10)) {
						a.scheduleWorkflowPlan(value)
					}
				}
				return true
			})
		case <-scheduleTimer.C: // 最近的一个调度任务到期执行
		}

		if a.isClose {
			scheduleTimer.Stop()
			continue
		}
		// 每次触发事件后 重新计算下次调度任务时间
		scheduleAfter = a.TrySchedule()
		scheduleTimer.Reset(scheduleAfter)
	}
}

func (a *workflowRunner) handleTaskResultV1(agentIP string, data protocol.TaskFinishedV1) error {
	var (
		next         = true
		planFinished bool
		err          error
	)
	err = rego.Retry(func() error {
		_, err := concurrency.NewSTM(a.etcd, func(s concurrency.STM) error {
			planFinished, err = setWorkFlowTaskFinished(s, agentIP, data)
			if err != nil {
				return err
			}
			return nil
		})
		return err
	}, rego.WithTimes(3), rego.WithPeriod(time.Second), rego.WithLatestError())
	if err != nil {
		return err
	}

	a.app.PublishMessage(messageWorkflowTaskStatusChanged(data.WorkflowID, data.ProjectID, data.TaskID, data.Status))

	// 任务如果失败三次，则终止整个workflow
	if planFinished {
		next = false
		plan := a.GetPlan(data.WorkflowID)
		if plan != nil {
			plan.Finished(nil)
		}
	}

	if !next {
		return nil
	}
	plan := a.GetPlan(data.WorkflowID)
	if plan == nil {
		return nil
	}

	err = rego.Retry(func() error {
		return a.scheduleWorkflowPlan(plan)
	}, rego.WithLatestError())
	if err != nil {
		return err
	}
	return nil
}

func (a *workflowRunner) handleTaskEvent(event *common.TaskEvent) {
	defer func() {
		if r := recover(); r != nil {
			a.app.Warning(warning.WarningData{
				Data: fmt.Sprintf("workflow任务调度失败，workflow_id: %d\n panic: %v",
					event.Task.FlowInfo.WorkflowID, r),
				Type:      warning.WarningTypeSystem,
				TaskName:  event.Task.Name,
				ProjectID: event.Task.ProjectID,
			})
		}
	}()
	switch event.EventType {
	case common.TASK_EVENT_WORKFLOW_SCHEDULE:
		plan := a.GetPlan(event.Task.FlowInfo.WorkflowID)
		if plan == nil {
			return
		}

		err := a.scheduleTask(event.Task)
		if err != nil {
			// if err := a.GetPlan(event.Task.FlowInfo.WorkflowID).Finished(fmt.Errorf("workflow任务(%s)调度失败, %w", event.Task.Name, err)); err != nil {
			// 	// todo log
			// 	fmt.Println("finished error", err.Error())
			// }
			wlog.Error("failed to schedule workflow task", zap.Error(err))
			a.app.Warning(warning.WarningData{
				Data: fmt.Sprintf("workflow任务调度失败，workflow_id: %d\n%s",
					event.Task.FlowInfo.WorkflowID, err.Error()),
				Type:      warning.WarningTypeSystem,
				TaskName:  event.Task.Name,
				ProjectID: event.Task.ProjectID,
			})
		}
	}
}

func (p *WorkflowPlan) RefreshStates() error {
	states, err := getWorkflowPlanState(p.runner.etcd.KV, p.Workflow.ID)
	if err != nil {
		return err
	}

	p.planState = states
	return nil
}

// TODO
func (p *WorkflowPlan) IsRunning() (bool, error) {
	if err := p.RefreshStates(); err != nil {
		return false, err
	}
	if p.planState == nil {
		return false, nil
	}

	now := time.Now()

	if now.Unix()-p.planState.LatestTryTime > p.Expr.Next(now).Unix()-now.Unix() {
		return false, nil
	}
	return p.planState.Status == common.TASK_STATUS_RUNNING_V2, nil
}

func (p *WorkflowPlan) SetRunning() error {
	newState, err := setWorkflowPlanRunning(p.runner.etcd, p.Workflow.ID)
	if err != nil {
		return err
	}
	p.planState = newState
	p.runner.app.PublishMessage(messageWorkflowStatusChanged(p.Workflow.ID, common.TASK_STATUS_RUNNING_V2))
	return nil
}
