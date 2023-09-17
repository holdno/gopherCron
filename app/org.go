package app

import (
	"fmt"
	"net/http"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/utils"

	"github.com/holdno/gocommons/selection"
	"github.com/jinzhu/gorm"
	"github.com/spacegrower/watermelon/infra/wlog"
	"go.uber.org/zap"
)

func (a *app) GetUserOrgs(userID int64) ([]*common.Org, error) {
	isAdmin, err := a.IsAdmin(userID)
	if err != nil {
		return nil, err
	}
	selector := selection.NewSelector()
	selector.AddOrder("create_time ASC")
	var orgIDs []string
	if !isAdmin {
		orgRelevance, err := a.store.OrgRelevance().ListUserOrg(userID)
		if err != nil && err != common.ErrNoRows {
			return nil, errors.NewError(http.StatusInternalServerError, "获取用户关联组织信息失败").WithLog(err.Error())
		}

		for _, v := range orgRelevance {
			orgIDs = append(orgIDs, v.OID)
		}

		if len(orgIDs) == 0 {
			return nil, nil
		}

		selector.AddQuery(selection.NewRequirement("id", selection.In, orgIDs))
	}

	list, err := a.store.Org().List(selector)
	if err != nil && err != common.ErrNoRows {
		return nil, errors.NewError(http.StatusInternalServerError, "获取用户组织列表失败").WithLog(err.Error())
	}

	list = append(list, &common.Org{
		ID:    "baseorg",
		Title: "通用",
	})

	return list, nil
}

func (a *app) CleanProject(tx *gorm.DB, pid int64) error {
	var err error
	if err = a.store.Project().DeleteProjectV2(tx, pid); err != nil {
		return errors.NewError(http.StatusInternalServerError, fmt.Sprintf("删除项目%d失败", pid)).WithLog(err.Error())
	}

	if err = a.CleanProjectLog(tx, pid); err != nil {
		return err
	}

	if err = a.DeleteAllWebHook(tx, pid); err != nil {
		return err
	}

	// warn: no trans
	if err = a.DeleteProjectAllTasks(pid); err != nil {
		return err
	}
	return nil
}

func (a *app) DeleteOrg(orgID string, userID int64) error {
	role, err := a.store.OrgRelevance().GetUserOrg(orgID, userID)
	if err != nil && err != common.ErrNoRows {
		return errors.NewError(http.StatusInternalServerError, "删除组织失败").WithLog(err.Error())
	}

	if role == nil || !a.rbacSrv.IsGranted(role.Role, PermissionAll) {
		return errors.NewError(http.StatusForbidden, "权限不足")
	}

	plist, err := a.GetProjects(orgID)
	if err != nil {
		return err
	}

	tx := a.BeginTx()
	defer func() {
		if r := recover(); r != nil || err != nil {
			wlog.Error("failed to delete org, panic", zap.Any("recover", r), zap.Error(err))
			tx.Rollback()
		}
	}()
	for _, v := range plist {
		if err := a.CleanProject(tx, v.ID); err != nil {
			return errors.NewError(http.StatusInternalServerError, fmt.Sprintf("删除项目%d失败", v.ID)).WithLog(err.Error())
		}
	}

	if err = tx.Commit().Error; err != nil {
		return errors.NewError(http.StatusInternalServerError, "删除组织事务提交失败").WithLog(err.Error())
	}
	return nil
}

func (a *app) CreateOrg(userID int64, title string) (string, error) {
	// 当前管理员才可以创建权限
	isAdmin, err := a.IsAdmin(userID)
	if err != nil {
		return "", err
	}

	if !isAdmin {
		return "", errors.NewError(http.StatusForbidden, "权限不足")
	}

	// todo limit

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

	err = a.store.OrgRelevance().Create(tx, common.OrgRelevance{
		UID:        userID,
		OID:        oid,
		Role:       RoleChief.IDStr,
		CreateTime: time.Now().Unix(),
	})
	if err != nil {
		return "", errors.NewError(http.StatusInternalServerError, "创建组织关联用户失败").WithLog(err.Error())
	}

	if err = tx.Commit().Error; err != nil {
		return "", errors.NewError(http.StatusInternalServerError, "创建组织事务提交失败").WithLog(err.Error())
	}
	return oid, nil
}
