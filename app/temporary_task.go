package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/holdno/gocommons/selection"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/pkg/warning"
	"github.com/holdno/gopherCron/utils"

	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

func (a *app) CreateTemporaryTask(data common.TemporaryTask) error {
	if data.ScheduleTime < time.Now().Unix() {
		return errors.NewError(http.StatusBadRequest, "任务调度时间已过，请重新设置调度时间")
	}
	data.CreateTime = time.Now().Unix()
	data.TmpID = utils.GetStrID()
	if err := a.store.TemporaryTask().Create(data); err != nil {
		return errors.NewError(http.StatusInternalServerError, "创建临时调度任务失败").WithLog(err.Error())
	}
	return nil
}

func (a *app) GetNeedToScheduleTemporaryTask(t time.Time) ([]*common.TemporaryTask, error) {
	truncateTime := t.Truncate(time.Minute)
	selector := selection.NewSelector(
		selection.NewRequirement("schedule_time", selection.LessThanEqual, truncateTime.Add(time.Minute).Unix()),
		selection.NewRequirement("schedule_status", selection.Equals, common.TEMPORARY_TASK_SCHEDULE_STATUS_WAITING),
	)
	list, err := a.store.TemporaryTask().GetList(selector)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, errors.NewError(http.StatusInternalServerError, "获取临时调度任务列表失败").WithLog(err.Error())
	}
	return list, nil
}

func (a *app) GetTemporaryTaskList(projectID int64) ([]*common.TemporaryTask, error) {
	selector := selection.NewSelector(
		selection.NewRequirement("project_id", selection.Equals, projectID))
	list, err := a.store.TemporaryTask().GetList(selector)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, errors.NewError(http.StatusInternalServerError, "获取临时调度任务列表失败").WithLog(err.Error())
	}
	return list, nil
}

type TemporaryTaskListWithUser struct {
	*common.TemporaryTask
	UserName  string `json:"user_name"`
	IsRunning int    `json:"is_running"`
}

func (a *app) GetTemporaryTaskListWithUser(projectID int64) ([]TemporaryTaskListWithUser, error) {
	list, err := a.GetTemporaryTaskList(projectID)
	if err != nil {
		return nil, err
	}

	var userIDs []int64
	for _, v := range list {
		userIDs = append(userIDs, v.UserID)
	}
	userList, err := a.store.User().GetUsers(selection.NewSelector(selection.NewRequirement("id", selection.In, userIDs)))
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, errors.NewError(http.StatusInternalServerError, "获取任务创建人信息失败").WithLog(err.Error())
	}

	userMap := make(map[int64]*common.User)
	for _, v := range userList {
		userMap[v.ID] = v
	}

	// build etcd pre key
	preKey := common.BuildKey(projectID, "")
	ctx, _ := utils.GetContextWithTimeout()
	getResp, err := a.etcd.KV().Get(ctx, preKey, clientv3.WithPrefix())
	if err != nil {
		return nil, errors.NewError(http.StatusInternalServerError, "获取任务状态失败").WithLog(err.Error())
	}

	taskStatus := make(map[string]string)
	for _, kvPair := range getResp.Kvs {
		if common.IsStatusKey(string(kvPair.Key)) {
			pid, tid := common.PatchProjectIDTaskIDFromStatusKey(string(kvPair.Key))
			taskStatus[fmt.Sprintf("%s_%s", pid, tid)] = string(kvPair.Value)
		}
	}

	var result []TemporaryTaskListWithUser
	for _, v := range list {

		item := TemporaryTaskListWithUser{
			TemporaryTask: v,
			UserName:      "-",
		}

		if u, exist := userMap[v.UserID]; exist {
			item.UserName = u.Name
		}

		status, exist := taskStatus[fmt.Sprintf("%d_%s", v.ProjectID, v.TaskID)]
		if exist {
			var taskRuningInfo common.TaskRunningInfo
			if err = json.Unmarshal([]byte(status), &taskRuningInfo); err == nil && taskRuningInfo.TmpID == v.TmpID {
				switch taskRuningInfo.Status {
				case common.TASK_STATUS_RUNNING_V2:
					item.IsRunning = common.TASK_STATUS_RUNNING
				default:
					item.IsRunning = common.TASK_STATUS_NOT_RUNNING
				}
			}
		}

		result = append(result, item)
	}

	return result, nil
}

func (a *app) TemporaryTaskSchedule(tmpTask common.TemporaryTask) error {
	task, err := a.GetTask(tmpTask.ProjectID, tmpTask.TaskID)
	if err != nil {
		return err
	}

	if tmpTask.Command != "" {
		task.Command = tmpTask.Command
	}
	task.TmpID = tmpTask.TmpID

	tx := a.store.BeginTx()
	defer func() {
		if r := recover(); r != nil || err != nil {
			if err == nil {
				err = fmt.Errorf("panic: %s", r)
			}
			a.Warning(warning.WarningData{
				Type:      warning.WarningTypeSystem,
				Data:      fmt.Sprintf("临时任务调度/执行失败, %s, DB Rollback", err.Error()),
				TaskName:  task.Name,
				ProjectID: tmpTask.ProjectID,
			})
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()
	err = a.store.TemporaryTask().UpdateTaskScheduleStatus(tx, tmpTask.ProjectID, tmpTask.TaskID, common.TEMPORARY_TASK_SCHEDULE_STATUS_SCHEDULED)
	if err != nil {
		return errors.NewError(http.StatusInternalServerError, "更新临时任务调度状态失败").WithLog(err.Error())
	}

	if err = a.TemporarySchedulerTask(task); err != nil {
		return err
	}
	return nil
}

func (a *app) AutoCleanScheduledTemporaryTask() {
	opt := selection.NewSelector(selection.NewRequirement("schedule_time", selection.LessThan, time.Now().Unix()-86400*7))
	if err := a.store.TemporaryTask().Clean(nil, opt); err != nil {
		a.logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Error("failed to clean scheduled temporary task by auto clean")
	}
}
