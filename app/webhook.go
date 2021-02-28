package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/holdno/gopherCron/utils"

	"github.com/holdno/gopherCron/jwt"

	"github.com/jinzhu/gorm"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
)

func (a *app) CreateWebHook(projectID int64, types, callbackUrl string) error {
	err := a.store.WebHook().Create(common.WebHook{
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

func (a *app) GetWebHookList(projectID int64) ([]common.WebHook, error) {
	list, err := a.store.WebHook().GetList(projectID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		errObj := errors.ErrInternalError
		errObj.Log = "[WebHook - GetWebHookList] failed to get webhook list: " + err.Error()
		return nil, errObj
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
	}

	return nil
}

func (a *app) HandleWebHook(projectID int64, taskID string, types string, logDetail *common.TaskLog) error {
	wh, err := a.GetWebHook(projectID, types)
	if err != nil {
		return err
	}

	if wh == nil {
		return nil
	}

	var (
		retryTimes = [5]time.Duration{1, 3, 5, 7, 9}
		index      = 0
	)

	// 获取项目创建者信息
	project, err := a.GetProject(projectID)
	if err != nil {
		return err
	}

	token := jwt.Build(project.UID)

	params := map[string]interface{}{
		"task_id":    taskID,
		"project_id": projectID,
		"command":    logDetail.Command,
		"start_time": logDetail.StartTime,
		"end_time":   logDetail.EndTime,
		"result":     logDetail.Result,
		"client_ip":  logDetail.ClientIP,
	}

	reqData, _ := json.Marshal(params)
	req, _ := http.NewRequest(http.MethodPost, wh.CallbackUrl, bytes.NewReader(reqData))
	req.Header.Set("token", token)

	err = utils.RetryFunc(5, func() error {
		resp, err := a.httpClient.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			if index >= 5 {
				return err
			}
			time.Sleep(retryTimes[index] * time.Second)
			index++
			if err != nil {
				return err
			}
			return errors.NewError(resp.StatusCode, "回调响应失败", "the hook is faild")
		}
		return nil
	})

	if err != nil {
		task, getTaskErr := a.GetTask(projectID, taskID)
		if getTaskErr != nil {
			return getTaskErr
		}
		_ = a.Warning(WarningData{
			Data:      err.Error(),
			Type:      WarningTypeTask,
			AgentIP:   a.localip,
			TaskName:  task.Name,
			ProjectID: projectID,
		})
	}
	return nil
}
