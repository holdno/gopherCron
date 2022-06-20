package app

import (
	"fmt"
	"net/http"
	"time"

	"github.com/holdno/gocommons/selection"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/pkg/warning"
	"github.com/sirupsen/logrus"

	"github.com/jinzhu/gorm"
)

func (a *app) CreateTemporaryTask(data common.TemporaryTask) error {
	if data.ScheduleTime < time.Now().Unix() {
		return errors.NewError(http.StatusBadRequest, "任务调度时间已过，请重新设置调度时间")
	}
	data.CreateTime = time.Now().Unix()
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
	UserName string `json:"user_name"`
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

	var result []TemporaryTaskListWithUser
	for _, v := range list {
		var (
			u    = userMap[v.UserID]
			name = "-"
		)
		if u != nil {
			name = u.Name
		}
		result = append(result, TemporaryTaskListWithUser{
			TemporaryTask: v,
			UserName:      name,
		})
	}

	return result, nil
}

func (a *app) TemporaryTaskSchedule(projectID int64, taskID string, realCommand string) error {
	task, err := a.GetTask(projectID, taskID)
	if err != nil {
		return err
	}

	if realCommand != "" {
		task.Command = realCommand
	}

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
				ProjectID: projectID,
			})
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()
	err = a.store.TemporaryTask().UpdateTaskScheduleStatus(tx, projectID, taskID, common.TEMPORARY_TASK_SCHEDULE_STATUS_SCHEDULED)
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
