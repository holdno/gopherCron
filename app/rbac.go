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

var (
	PermissionAll    = gorbac.NewStdPermission("all")
	PermissionDelete = gorbac.NewStdPermission("delete")
	PermissionEdit   = gorbac.NewStdPermission("edit")
	PermissionView   = gorbac.NewStdPermission("view")

	RoleChief   = gorbac.NewStdRole("admin")
	RoleManager = gorbac.NewStdRole("manager")
	RoleUser    = gorbac.NewStdRole("user")
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
