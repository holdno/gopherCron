package store

import (
	"github.com/holdno/gopherCron/common"

	"github.com/holdno/gocommons/selection"
	"github.com/jinzhu/gorm"
)

// Commons defined func which can be used by other stores
type Commons interface {
	GetMap(selector selection.Selector) ([]map[string]interface{}, error)
	GetTable() string
	GetTotal(selector selection.Selector) (int, error)
	CheckSelf()
	AutoMigrate()
}

type ProjectStore interface {
	Commons
	CreateProject(tx *gorm.DB, obj common.Project) (int64, error)
	UpdateProject(id int64, title, remark string) error
	GetProject(selector selection.Selector) ([]*common.Project, error)
	DeleteProject(tx *gorm.DB, selector selection.Selector) error
	UpdateRelation(projectID int64, relation string) error
}

type ProjectRelevanceStore interface {
	Commons
	Create(tx *gorm.DB, r common.ProjectRelevance) error
	Delete(tx *gorm.DB, pid, uid int64) error
	GetList(selector selection.Selector) ([]*common.ProjectRelevance, error)
}

type UserStore interface {
	Commons
	CreateAdminUser() error
	DeleteUser(id int64) error
	GetAdminUser() (*common.User, error)
	CreateUser(user common.User) error
	ChangePassword(uid int64, password, salt string) error
	GetUsers(selector selection.Selector) ([]*common.User, error)
}

type TaskLogStore interface {
	Commons
	CreateTaskLog(data common.TaskLog) error
	GetList(selector selection.Selector) ([]*common.TaskLog, error)
	GetOne(projectID int64, taskID, tmpID string) (*common.TaskLog, error)
	Clean(tx *gorm.DB, selector selection.Selector) error
}

type TemporaryTaskStore interface {
	Commons
	Create(data common.TemporaryTask) error
	GetList(selector selection.Selector) ([]*common.TemporaryTask, error)
	UpdateTaskScheduleStatus(tx *gorm.DB, projectID int64, taskID string, scheduleStatus int32) error
	Clean(tx *gorm.DB, selector selection.Selector) error
}

type UserWorkflowRelevanceStore interface {
	Commons
	Create(tx *gorm.DB, data *common.UserWorkflowRelevance) error
	GetUserWorkflows(userID int64) ([]common.UserWorkflowRelevance, error)
	GetUserWorkflowRelevance(userID int64, workflowID int64) (*common.UserWorkflowRelevance, error)
	DeleteWorkflowAllUserRelevance(tx *gorm.DB, workflowID int64) error
	DeleteUserWorkflowRelevance(tx *gorm.DB, workflowID, userID int64) error
	GetWorkflowUsers(workflowID int64) ([]common.UserWorkflowRelevance, error)
}

type WorkflowSchedulePlanStore interface {
	Commons
	Create(tx *gorm.DB, data *common.WorkflowSchedulePlan) error
	GetList(workflowID int64) ([]common.WorkflowSchedulePlan, error)
	GetLatestTaskCreateTime(workflowID int64) (*common.WorkflowSchedulePlan, error)
	GetTaskWorkflowIDs(index []string) ([]common.WorkflowSchedulePlan, error)
	Delete(tx *gorm.DB, id int64) error
	DeleteAllWorkflowSchedulePlan(tx *gorm.DB, workflowID int64) error
	DeleteList(tx *gorm.DB, ids []int64) error
}

type WorkflowTaskStore interface {
	Commons
	Create(tx *gorm.DB, data *common.WorkflowTask) error
	GetList(projectID int64) ([]common.WorkflowTask, error)
	GetOne(projectID int64, taskID string) (*common.WorkflowTask, error)
	Save(tx *gorm.DB, data *common.WorkflowTask) error
	Delete(tx *gorm.DB, projectID int64, taskID string) error
	GetMultiList(taskIDs []string) ([]common.WorkflowTask, error)
}

type WorkflowStore interface {
	Commons
	Create(tx *gorm.DB, data *common.Workflow) error
	GetList(selector selection.Selector) ([]common.Workflow, error)
	Update(tx *gorm.DB, data common.Workflow) error
	GetOne(id int64) (*common.Workflow, error)
	Delete(tx *gorm.DB, id int64) error
}

type TaskWebHookStore interface {
	Commons
	Create(data common.WebHook) error
	GetList(projectID int64) ([]common.WebHook, error)
	GetOne(projectID int64, types string) (*common.WebHook, error)
	Delete(tx *gorm.DB, projectID int64, types string) error
	DeleteAll(tx *gorm.DB, projectID int64) error
}

type WorkflowLogStore interface {
	Commons
	Create(tx *gorm.DB, data *common.WorkflowLog) error
	GetList(selector selection.Selector, page, pagesize uint64) ([]common.WorkflowLog, error)
	Clear(tx *gorm.DB, selector selection.Selector) error
}
