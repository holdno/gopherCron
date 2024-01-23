package common

import (
	"fmt"
)

type ClientInfo struct {
	ClientIP string `json:"client_ip"`
	Version  string `json:"version"`
	Region   string `json:"region"`
}

type User struct {
	ID         int64  `json:"id" gorm:"column:id;primary_key;auto_increment"`
	Name       string `json:"name" gorm:"column:name;index:name;type:varchar(100);not null;comment:'用户名称'"`
	Permission string `json:"permission" gorm:"column:permission;type:varchar(100);not null;comment:'用户权限'"`
	Account    string `json:"account" gorm:"column:account;index:account;type:varchar(100);not null;comment:'用户账号'"`
	Password   string `json:"-" gorm:"password;type:varchar(255);not null;comment:'用户密码'"`
	Salt       string `json:"-" gorm:"salt;type:varchar(6);not null;comment:'密码盐'"`
	CreateTime int64  `json:"create_time" gorm:"column:create_time;type:bigint(20);not null;comment:'创建时间'"`
}

type Project struct {
	ID     int64  `json:"id" gorm:"column:id;primary_key;auto_increment"`
	UID    int64  `json:"uid" gorm:"column:uid;index:uid;type:bigint(20);not null;comment:'关联用户id'"`
	OID    string `json:"oid" gorm:"column:oid;index:oid;type:varchar(32);not null;comment:'关联组织id'"`
	Title  string `json:"title" gorm:"column:title;index:title;type:varchar(100);not null;comment:'项目名称'"`
	Remark string `json:"remark" gorm:"column:remark;type:varchar(255);not null;comment:'项目备注'"`
	Token  string `json:"token" gorm:"column:token;type:varchar(32);not null;default:'';comment:'项目token,用于请求中心接口'"`
}

type Org struct {
	ID         string `json:"id" gorm:"column:id;type:varchar(32);not null;comment:'组织id'"`
	Title      string `json:"title" gorm:"column:title;type:varchar(32);not null;comment:'组织名称'"`
	CreateTime int64  `json:"create_time" gorm:"column:create_time;type:bigint(20);not null;comment:'创建时间'"`
}

type OrgRelevance struct {
	ID         int64  `json:"id" gorm:"column:id;primary_key;auto_increment"`
	UID        int64  `json:"uid" gorm:"column:uid;index:uid;type:bigint(20);not null;comment:'关联用户id'"`
	OID        string `json:"oid" gorm:"column:oid;index:oid;type:varchar(32);not null;comment:'关联组织id'"`
	Role       string `json:"role" gorm:"column:role;type:varchar(32);default:'user';not null;comment:'项目角色'"`
	CreateTime int64  `json:"create_time" gorm:"column:create_time;type:bigint(20);not null;comment:'创建时间'"`
}

type ProjectWithUserRole struct {
	*Project
	Role string `json:"role"`
}

type ProjectRelevance struct {
	ID         int64  `json:"id" gorm:"column:id;primary_key;auto_increment"`
	UID        int64  `json:"uid" gorm:"column:uid;index:uid;type:bigint(20);not null;comment:'关联用户id'"`
	ProjectID  int64  `json:"project_id" gorm:"column:project_id;index:project_id;type:bigint(20);not null;comment:'关联项目id'"`
	Role       string `json:"role" gorm:"column:role;type:varchar(32);default:'user';not null;comment:'项目角色'"`
	CreateTime int64  `json:"create_time" gorm:"column:create_time;type:bigint(20);not null;comment:'创建时间'"`
}

type TaskLog struct {
	ID        int64  `json:"id" gorm:"column:id;primary_key;auto_increment"`
	ProjectID int64  `json:"project_id" gorm:"column:project_id;index:project_id;type:bigint(20);not null;comment:'关联项目id'"`
	TaskID    string `json:"task_id" gorm:"column:task_id;index:task_id;type:varchar(32);not null;comment:'关联任务id'"`
	Project   string `json:"project" gorm:"column:project;type:varchar(100);not null;comment:'项目名称'"`

	Name      string `json:"name" gorm:"column:name;index:name;type:varchar(100);not null;comment:'任务名称'"`
	Result    string `json:"result" gorm:"column:result;type:text;not null;comment:'任务执行结果'"`
	StartTime int64  `json:"start_time" gorm:"column:start_time;index:start_time;type:bigint(20);not null;comment:'任务开始时间'"`
	EndTime   int64  `json:"end_time" gorm:"column:end_time;index:end_time;type:bigint(20);not null;comment:'任务结束时间'"`
	Command   string `json:"command" gorm:"column:command;type:varchar(255);not null;comment:'任务指令'"`
	WithError int    `json:"with_error" gorm:"column:with_error;type:int(11);not null;comment:'是否发生错误'"`
	ClientIP  string `json:"client_ip" gorm:"client_ip;index:client_ip;type:varchar(20);not null;comment:'节点ip'"`
	TmpID     string `json:"tmp_id" gorm:"column:tmp_id;type:varchar(50);not null;comment:'任务执行id'"`
}

type WebHook struct {
	CallbackURL string `json:"callback_url" gorm:"column:callback_url;type:varchar(255);not null;comment:'回调地址'"`
	ProjectID   int64  `json:"project_id" gorm:"column:project_id;type:int(11);index:project_id;not null;comment:'关联项目id'"`
	Type        string `json:"type" gorm:"column:type;type:varchar(30);not null;index:type;comment:'webhook类型'"`
	CreateTime  int64  `json:"create_time" gorm:"column:create_time;type:int(11);not null;comment:'创建时间'"`
}

type WebHookBody struct {
	TaskID      string `json:"task_id" form:"task_id"`
	TaskName    string `json:"task_name" form:"task_name"`
	ProjectID   int64  `json:"project_id" form:"project_id"`
	ProjectName string `json:"project_name" form:"project_name"`
	Command     string `json:"command" form:"command"`
	StartTime   int64  `json:"start_time" form:"start_time"`
	EndTime     int64  `json:"end_time" form:"end_time"`
	Result      string `json:"result" form:"result"`
	Error       string `json:"error" form:"error"`
	ClientIP    string `json:"client_ip" form:"client_ip"`
	TmpID       string `json:"tmp_id" form:"tmp_id"`
	Operator    string `json:"operator" form:"operator"`
}

type Workflow struct {
	ID         int64  `json:"id" gorm:"column:id;primary_key;auto_increment"`
	OID        string `json:"oid" gorm:"column:oid;index:oid;type:varchar(32);not null;comment:'关联组织id'"`
	Title      string `json:"title" gorm:"column:title;type:varchar(100);not null;comment:'flow标题'"`
	Remark     string `json:"remark" gorm:"column:remark;type:text;not null;comment:'flow详细介绍'"`
	Cron       string `json:"cron" gorm:"column:cron;type:varchar(20);not null;comment:'cron表达式'"`
	Status     int    `json:"status" gorm:"column:status;type:tinyint(1);not null;default:2;comment:'workflow状态，1启用2暂停'"`
	CreateTime int64  `json:"create_time" gorm:"column:create_time;type:int(11);not null;comment:'创建时间'"`
}

type GetWorkflowListOptions struct {
	OID   string
	Title string
	IDs   []int64
}

type WorkflowSchedulePlan struct {
	ID                  int64  `json:"id" gorm:"column:id;primary_key;auto_increment"`
	WorkflowID          int64  `json:"workflow_id" gorm:"column:workflow_id;type:int(11);not null;index:workflow_id;comment:'关联workflow id'"`
	TaskID              string `json:"task_id" gorm:"column:task_id;type:varchar(50);not null;index:task_id;comment:'task id'"`
	ProjectID           int64  `json:"project_id" gorm:"column:project_id;type:int(11);not null;index:project_id;comment:'project id'"`
	DependencyTaskID    string `json:"dependency_task_id" gorm:"column:dependency_task_id;not null;type:varchar(50);default:'';index:dependency_task_id;"`
	DependencyProjectID int64  `json:"dependency_project_id" gorm:"column:dependency_project_id;not null;type:int(11);default:0;index:dependency_project_id;comment:'依赖任务的项目id'"`
	ProjectTaskIndex    string `json:"project_task_index" gorm:"column:project_task_index;not null;type:varchar(100);default:'';index:project_task_index;comment:'项目+任务索引'"`
	CreateTime          int64  `json:"create_time" gorm:"column:create_time;type:int(11);not null;comment:'创建时间'"`
}

func (w *WorkflowSchedulePlan) BuildIndex() {
	w.ProjectTaskIndex = BuildWorkflowTaskIndex(w.ProjectID, w.TaskID)
}

type WorkflowTask struct {
	TaskID     string `json:"task_id" gorm:"column:task_id;type:varchar(50);primary_key;not null;comment:'task id'"`
	ProjectID  int64  `json:"project_id" gorm:"column:project_id;type:int(11);not null;index:project_id;comment:'project id'"`
	TaskName   string `json:"task_name" gorm:"column:task_name;type:varchar(100);not null;index:task_name;comment:'任务名称'"`
	Command    string `json:"command" gorm:"column:command;type:text;comment:'执行命令'"`
	Remark     string `json:"remark" gorm:"column:remark;type:text;comment:'任务备注'"`
	Timeout    int    `json:"timeout" gorm:"column:timeout;not null;default:0;comment:'超时时间(s)'"`
	Noseize    int    `json:"noseize" gorm:"column:noseize;not null;default:0;comment:'不抢占，设为1后多个agent并行执行'"`
	WorkflowID int64  `json:"workflow_id" gorm:"column:workflow_id;type:int(11);not null;index:workflow_id;comment:'关联workflow id'"`
	CreateTime int64  `json:"create_time" gorm:"column:create_time;type:int(11);not null;comment:'创建时间'"`
}

func BuildWorkflowTaskIndex(pid int64, tid string) string {
	return fmt.Sprintf("%d_%s", pid, tid)
}

type UserWorkflowRelevance struct {
	ID         int64 `json:"id" gorm:"column:id;primary_key;auto_increment"`
	UserID     int64 `json:"user_id" gorm:"column:user_id;type:int(11);not null;index:user_id;comment:'关联用户id'"`
	WorkflowID int64 `json:"workflow_id" gorm:"column:workflow_id;type:int(11);not null;index:workflow_id;comment:'关联workflow id'"`
	CreateTime int64 `json:"create_time" gorm:"column:create_time;type:int(11);not null;comment:'创建时间'"`
}

type WorkflowLog struct {
	ID         int64  `json:"id" gorm:"column:id;primary_key;auto_increment"`
	WorkflowID int64  `json:"workflow_id" gorm:"column:workflow_id;type:int(11);not null;index:workflow_id;comment:'关联workflow id'"`
	StartTime  int64  `json:"start_time" gorm:"column:start_time;type:int(11);not null;comment:'开始时间'"`
	EndTime    int64  `json:"end_time" gorm:"column:end_time;type:int(11);not null;comment:'结束时间'"`
	Result     string `json:"result" gorm:"column:result;type:text;not null;comment:'任务执行结果'"`
	CreateTime int64  `json:"create_time" gorm:"column:create_time;type:int(11);not null;comment:'创建时间'"`
}

type TemporaryTask struct {
	ID             int64  `json:"id" gorm:"column:id;primary_key;auto_increment"`
	ProjectID      int64  `json:"project_id" gorm:"column:project_id;type:int(11);index:project_id;not null;index:project_id;comment:'project id'"`
	TaskID         string `json:"task_id" gorm:"column:task_id;type:varchar(50);primary_key;not null;comment:'task id'"`
	ScheduleTime   int64  `json:"schedule_time" gorm:"column:schedule_time;type:int(11);index:schedule_time;not null;comment:'调度时间'"`
	ScheduleStatus int32  `json:"schedule_status" gorm:"column:schedule_status;type:int(1);index:schedule_status;not null;comment:'调度状态'"`
	UserID         int64  `json:"user_id" gorm:"column:user_id;type:int(11);not null;index:user_id;comment:'关联用户id'"`
	Command        string `json:"command" gorm:"column:command;type:varchar(255);not null;comment:'任务指令'"`
	Noseize        int    `json:"noseize" gorm:"column:noseize;type:tinyint(1);not null;comment:'不抢占'"`
	TmpID          string `json:"tmp_id" gorm:"column:tmp_id;type:varchar(50);not null;comment:'临时任务id'"` // 每次任务执行的唯一标识
	Timeout        int    `json:"timeout" gorm:"column:timeout;type:int(11);not null;comment:'超时时间'"`     // 每次任务执行的唯一标识 // 任务超时时间 单位 秒(s)
	Remark         string `json:"remark" gorm:"column:remark;type:varchar(255);not null;comment:'任务备注'"`
	Host           string `json:"host" gorm:"column:host;type:varchar(255);not null;comment:'指定agent进行调度'"`
	CreateTime     int64  `json:"create_time" gorm:"column:create_time;type:int(11);not null;comment:'创建时间'"`
}
