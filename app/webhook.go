package app

import (
	"bytes"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/jinzhu/gorm"
	"github.com/spacegrower/watermelon/infra/wlog"
	"go.uber.org/zap"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/pkg/warning"
	"github.com/holdno/gopherCron/utils"
)

func (a *app) CreateWebHook(projectID int64, types, callbackUrl string) error {
	hook, err := a.GetWebHook(projectID, types)
	if err != nil {
		return err
	}

	if hook != nil {
		return errors.NewError(http.StatusForbidden, "当前webhook类型已存在")
	}

	err = a.store.WebHook().Create(common.WebHook{
		CallbackURL: callbackUrl,
		ProjectID:   projectID,
		Type:        types,
		CreateTime:  time.Now().Unix(),
	})

	if err != nil {
		errObj := errors.ErrInternalError
		errObj.Log = "[WebHook - CreateWebHook] failed to create webhook: " + err.Error()
		return errObj
	}

	return nil
}

func (a *app) GetWebHookList(projectID int64) ([]*common.WebHook, error) {
	list, err := a.store.WebHook().GetList(projectID)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, errors.NewError(http.StatusInternalServerError, "failed to get project webhooks")
	}

	return list, nil
}

func (a *app) GetWebHook(projectID int64, types string) (*common.WebHook, error) {
	res, err := a.store.WebHook().GetOne(projectID, types)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		errObj := errors.ErrInternalError
		errObj.Log = "[WebHook - GetWebHook] failed to get webhook: " + err.Error()
		return nil, errObj
	}

	return res, nil
}

func (a *app) DeleteWebHook(tx *gorm.DB, projectID int64, types string) error {
	err := a.store.WebHook().Delete(tx, projectID, types)
	if err != nil {
		errObj := errors.ErrInternalError
		errObj.Log = "[WebHook - DeleteWebHook] failed to delete webhook: " + err.Error()
		return errObj
	}

	return nil
}

func (a *app) DeleteAllWebHook(tx *gorm.DB, projectID int64) error {
	err := a.store.WebHook().DeleteAll(tx, projectID)
	if err != nil {
		errObj := errors.ErrInternalError
		errObj.Log = "[WebHook - DeleteAllWebHook] failed to delete all webhook by project id: " + err.Error()
		return errObj
	}

	return nil
}

func (a *app) DeleteAllWorkflowTask(tx *gorm.DB, projectID int64) error {
	err := a.store.WorkflowTask().DeleteAll(tx, projectID)
	if err != nil {
		errObj := errors.ErrInternalError
		errObj.Log = "[WebHook - DeleteAllWorkflowTask] failed to delete all workflow task by project id: " + err.Error()
		return errObj
	}

	return nil
}

func (a *app) HandleWebHook(agentIP string, res *common.TaskFinishedV2) error {
	hooks, err := a.GetWebHookList(res.ProjectID)
	if err != nil {
		return err
	}

	if len(hooks) == 0 {
		return nil
	}

	hookMaps := make(map[string]*common.WebHook)
	hookURLMaps := make(map[string]*sync.Once)
	for _, v := range hooks {
		hookMaps[v.Type] = v
		if _, exist := hookURLMaps[v.CallbackURL]; !exist {
			hookURLMaps[v.CallbackURL] = &sync.Once{}
		}
	}
	// 任务没报错，也没有任务结束钩子的话，提前终止运行
	if res.Error == "" && hookMaps[common.WEBHOOK_TYPE_TASK_RESULT] == nil {
		return nil
	}

	wlog.Debug("handle webhook", zap.String("type", "finished"), zap.Int64("project_id", res.ProjectID), zap.String("task_id", res.TaskID))

	p, err := a.GetProject(res.ProjectID)
	if err != nil {
		return err
	}

	if p == nil {
		return nil
	}

	body := common.WebHookBody{
		TaskID:      res.TaskID,
		TaskName:    res.TaskName,
		ProjectID:   res.ProjectID,
		ProjectName: p.Title,
		Command:     res.Command,
		StartTime:   res.StartTime,
		EndTime:     res.EndTime,
		ClientIP:    agentIP,
		Result:      res.Result,
		Error:       res.Error,
		TmpID:       res.TmpID,
		Operator:    res.Operator,
	}

	var eventType = "succeeded"
	if res.Error != "" {
		eventType = "failure"
	}
	event := cloudevents.NewEvent()
	event.SetID(utils.GetStrID())
	event.SetSubject(common.WEBHOOK_TYPE_TASK_RESULT)
	event.SetData(cloudevents.ApplicationJSON, body)
	event.SetSource(fmt.Sprintf("%s-%d", common.GOPHERCRON_CENTER_NAME, a.ClusterID()))
	event.SetType(eventType)
	event.SetTime(time.Unix(res.EndTime, 0))
	reqData, _ := event.MarshalJSON()

	handleFunc := func(hook *common.WebHook) error {
		var err error
		hookURLMaps[hook.CallbackURL].Do(func() {
			err = retry.Do(func() error {
				req, _ := http.NewRequest(http.MethodPost, hook.CallbackURL, bytes.NewReader(reqData))
				req.Header.Add("Authorization", p.Token)
				resp, err := a.httpClient.Do(req)
				if err != nil {
					return errors.NewError(http.StatusInternalServerError, err.Error())
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					return errors.NewError(resp.StatusCode, "回调响应失败，"+resp.Status)
				}
				return nil
			}, retry.Attempts(5), retry.DelayType(retry.BackOffDelay),
				retry.MaxJitter(time.Minute), retry.LastErrorOnly(true))
		})

		if err != nil {
			a.Metrics().CustomInc("handle_webhook", a.GetIP(), fmt.Sprintf("%d", hook.ProjectID))
			wlog.Error("failed to handle webhook", zap.String("type", hook.Type),
				zap.Int64("project_id", res.ProjectID), zap.String("task_id", res.TaskID), zap.Error(err))
			a.Warning(warning.NewTaskWarningData(warning.TaskWarning{
				AgentIP:   a.localip,
				TaskName:  res.TaskName,
				TaskID:    res.TaskID,
				ProjectID: res.ProjectID,
				Message:   fmt.Sprintf("webhook request error %s, callback-url: %s", err.Error(), hook.CallbackURL),
			}))
		}
		return err
	}

	if eventType == "failure" && hookMaps[common.WEBHOOK_TYPE_TASK_FAILURE] != nil {
		handleFunc(hookMaps[common.WEBHOOK_TYPE_TASK_FAILURE])
	}

	if hook := hookMaps[common.WEBHOOK_TYPE_TASK_RESULT]; hook != nil {
		handleFunc(hook)
	}

	return nil
}
