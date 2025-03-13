package app

import (
	"net/http"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/jinzhu/gorm"
)

func (a *app) UpsertAgentActiveTime(projectID int64, clientIP string) error {
	err := a.store.AgentActivity().Create(nil, common.AgentActivity{
		ProjectID:  projectID,
		ClientIP:   clientIP,
		ActiveTime: time.Now().Unix(),
	})
	if err != nil {
		return errors.NewError(http.StatusInternalServerError, "更新Agent活跃时间出错").WithLog(err.Error())
	}

	return nil
}

func (a *app) GetAgentLatestActiveTime(projectID int64, clientIP string) (time.Time, error) {
	activeInfo, err := a.store.AgentActivity().GetOne(projectID, clientIP)
	if err != nil && err != gorm.ErrRecordNotFound {
		return time.Time{}, errors.NewError(http.StatusInternalServerError, "更新Agent活跃时间出错").WithLog(err.Error())
	}

	if activeInfo == nil {
		return time.Time{}, nil
	}

	return time.Unix(activeInfo.ActiveTime, 0), nil
}
