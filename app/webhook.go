package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/pkg/warning"
	"github.com/holdno/gopherCron/protocol"
	"github.com/holdno/gopherCron/utils"

	"github.com/jinzhu/gorm"
)

func (a *app) CreateWebHook(projectID int64, types, callbackUrl string) error {
	err := a.store.WebHook().Create(common.WebHook{
		CallbackURL: callbackUrl,
		ProjectID:   projectID,
		Type:        types,
		Secret:      utils.RandomStr(32),
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

func (a *app) HandleWebHook(agentIP string, res *protocol.TaskFinishedV1) error {
	wh, err := a.GetWebHook(res.ProjectID, "finished")
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

	body := common.WebHookBody{
		TaskID:    res.TaskID,
		ProjectID: res.ProjectID,
		Command:   res.Command,
		StartTime: res.StartTime,
		EndTime:   res.EndTime,
		ClientIP:  agentIP,
		Result:    res.Result,
		Error:     res.Error,
		TmpID:     res.TmpID,
	}

	p, err := a.GetProject(res.ProjectID)
	if err != nil {
		return err
	}

	err = utils.RetryFunc(5, func() error {
		reqData, _ := json.Marshal(body)
		req, _ := http.NewRequest(http.MethodPost, wh.CallbackURL, bytes.NewReader(reqData))
		req.Header.Add("Authorization", p.Token)
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
			return errors.NewError(resp.StatusCode, "回调响应失败")
		}
		return nil
	})

	if err != nil {
		a.Warning(warning.NewTaskWarningData(warning.TaskWarning{
			AgentIP:   a.localip,
			TaskName:  res.TaskName,
			TaskID:    res.TaskID,
			ProjectID: res.ProjectID,
			Message:   fmt.Sprintf("webhook request error %s, callback-url: %s", err.Error(), wh.CallbackURL),
		}))
	}
	return nil
}
