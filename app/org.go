package app

import (
	"net/http"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/utils"

	"github.com/holdno/gocommons/selection"
)

func (a *app) GetUserOrgs(userID int64) ([]*common.Org, error) {
	orgRelevance, err := a.store.OrgRelevanceStore().ListUserOrg(userID)
	if err != nil && err != common.ErrNoRows {
		return nil, errors.NewError(http.StatusInternalServerError, "获取用户关联组织信息失败").WithLog(err.Error())
	}

	var orgIDs []string
	for _, v := range orgRelevance {
		orgIDs = append(orgIDs, v.OID)
	}
	list, err := a.store.Org().List(selection.NewSelector(selection.NewRequirement("id", selection.In, orgIDs)))
	if err != nil && err != common.ErrNoRows {
		return nil, errors.NewError(http.StatusInternalServerError, "获取用户组织列表失败").WithLog(err.Error())
	}

	return list, nil
}

func (a *app) CreateOrg(userID int64, title string) (string, error) {
	// todo 用户有没有创建组织的权限？
	// 当前管理员才可以创建权限
	isAdmin, err := a.IsAdmin(userID)
	if err != nil {
		return "", err
	}

	if !isAdmin {
		return "", errors.NewError(http.StatusForbidden, "权限不足")
	}

	exists, err := a.store.Org().List(selection.NewSelector(selection.NewRequirement("title", selection.Equals, title)))
	if err != nil && err != common.ErrNoRows {
		return "", errors.NewError(http.StatusInternalServerError, "检测组织名称可用性失败").WithLog(err.Error())
	}

	if len(exists) > 0 {
		return "", errors.NewError(http.StatusForbidden, "组织名称不可用或已存在，请更换")
	}

	tx := a.store.BeginTx()
	defer func() {
		if r := recover(); r != nil && err != nil {
			tx.Rollback()
		}
	}()

	oid := utils.GetStrID()
	err = a.store.Org().CreateOrg(tx, common.Org{
		ID:         oid,
		Title:      title,
		CreateTime: time.Now().Unix(),
	})
	if err != nil {
		return "", errors.NewError(http.StatusInternalServerError, "创建组织失败").WithLog(err.Error())
	}

	err = a.store.OrgRelevanceStore().Create(tx, common.OrgRelevance{
		UID:        userID,
		OID:        oid,
		Role:       RoleChief.IDStr,
		CreateTime: time.Now().Unix(),
	})
	if err != nil {
		return "", errors.NewError(http.StatusInternalServerError, "创建组织关联用户失败").WithLog(err.Error())
	}
	return oid, nil
}
