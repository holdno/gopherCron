package app

import (
	"github.com/mikespook/gorbac"
)

type RBACSrv struct {
	rbac *gorbac.RBAC
}

type RBACImpl interface {
	IsGranted(id string, p gorbac.Permission) (rslt bool)
	GetRole(id string) (gorbac.Role, bool)
}

const (
	ROLE_NAME_CHIEF   = "admin"
	ROLE_NAME_MANAGER = "manager"
	ROLE_NAME_USER    = "user"
)

var (
	PermissionAll    = gorbac.NewStdPermission("all")
	PermissionDelete = gorbac.NewStdPermission("delete")
	PermissionEdit   = gorbac.NewStdPermission("edit")
	PermissionView   = gorbac.NewStdPermission("view")

	RoleChief            = gorbac.NewStdRole(ROLE_NAME_CHIEF)
	RoleManager          = gorbac.NewStdRole(ROLE_NAME_MANAGER)
	RoleUser             = gorbac.NewStdRole(ROLE_NAME_USER)
	roleLevelNumberIndex = map[string]int{ // 该map表示几个权限之间的等级关系，越小权限越大，这个数字可以随意改变，不会被存储，仅用于程序内比较
		ROLE_NAME_CHIEF:   100,
		ROLE_NAME_MANAGER: 1000,
		ROLE_NAME_USER:    10000,
	}
)

func NewRBACSrv() *RBACSrv {
	rbac := gorbac.New()

	assignChief(RoleChief)
	assignManager(RoleManager)
	assignUser(RoleUser)

	rbac.Add(RoleChief)
	rbac.Add(RoleManager)
	rbac.Add(RoleUser)

	rbac.SetParents(RoleChief.ID(), []string{RoleManager.ID()})
	rbac.SetParents(RoleManager.ID(), []string{RoleUser.ID()})

	return &RBACSrv{
		rbac: rbac,
	}
}

func assignChief(chief *gorbac.StdRole) {
	chief.Assign(PermissionAll)
}

func assignManager(manager *gorbac.StdRole) {
	manager.Assign(PermissionDelete)
	manager.Assign(PermissionEdit)
}

func assignUser(viewer *gorbac.StdRole) {
	viewer.Assign(PermissionView)
}

func (s *RBACSrv) IsGranted(id string, p gorbac.Permission) (rslt bool) {
	return s.rbac.IsGranted(id, p, nil)
}

func (s *RBACSrv) GetRole(id string) (gorbac.Role, bool) {
	role, _, err := s.rbac.Get(id)
	if err == gorbac.ErrRoleNotExist {
		return nil, false
	}
	return role, true
}

func (s *RBACSrv) GetParents(id string) ([]string, error) {
	return s.rbac.GetParents(id)
}

func CompareRole(a, b gorbac.Role) int {
	numberA := roleLevelNumberIndex[a.ID()]
	numberB := roleLevelNumberIndex[b.ID()]

	if numberA < numberB {
		return 1
	} else if numberA == numberB {
		return 0
	} else {
		return -1
	}
}
