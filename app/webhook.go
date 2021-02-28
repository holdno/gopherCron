package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

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

func (a *app) HandleWebHook(projectID int64, taskID string, types string) error {
	wh, err := a.GetWebHook(projectID, types)
	if err != nil {
		return err
	}

	if wh == nil {
		return nil
	}

	var (
		retryTimes = [3]time.Duration{1, 3, 5}
		index      = 0
		logDetail  *common.TaskLog
	)

	// 获取项目创建者信息
	project, err := a.GetProject(projectID)
	if err != nil {
		return err
	}

	for {
		if index > 2 {
			errObj := errors.ErrDataNotFound
			errObj.Msg = "无法获取项目日志"
			errObj.Log = fmt.Sprintf("project_id: %d, task_id: %s, failed to get logs", projectID, taskID)
			return errObj
		}
		logDetail, err = a.GetTaskLogDetail(projectID, taskID)
		if err != nil {
			return err
		}

		if logDetail == nil {
			time.Sleep(retryTimes[index] * time.Second)
			index++
			continue
		}
		break
	}

	token := jwt.Build(project.UID)

	var resultLog common.TaskResultLog
	json.Unmarshal([]byte(logDetail.Result), &resultLog)

	params := map[string]interface{}{
		"task_id":    taskID,
		"project_id": projectID,
		"command":    logDetail.Command,
		"start_time": logDetail.StartTime,
		"end_time":   logDetail.EndTime,
		"result":     logDetail.Result,
		"client_ip":  logDetail.ClientIP,
	}

	http.NewRequest(http.MethodPost, wh.CallbackUrl)
	a.httpClient.Post(wh.CallbackUrl, "content-type:application/json")
}
