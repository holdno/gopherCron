package common

type ClientInfo struct {
	ClientIP string `json:"client_ip"`
	Version  string `json:"version"`
}

type User struct {
	ID         int64  `json:"id" gorm:"column:id;pirmary_key;auto_increment"`
	Name       string `json:"name" gorm:"column:name;index:name;type:varchar(100);not null;comment:'用户名称'"`
	Permission string `json:"permission" gorm:"column:permission;type:varchar(100);not null;comment:'用户权限'"`
	Account    string `json:"account" gorm:"column:account;index:account;type:varchar(100);not null;comment:'用户账号'"`
	Password   string `json:"-" gorm:"password;type:varchar(255);not null;comment:'用户密码'"`
	Salt       string `json:"-" gorm:"salt;type:varchar(6);not null;comment:'密码盐'"`
	CreateTime int64  `json:"create_time" gorm:"column:create_time;type:bigint(20);not null;comment:'创建时间'"`
}

type Project struct {
	ID     int64  `json:"id" gorm:"column:id;pirmary_key;auto_increment"`
	UID    int64  `json:"uid" gorm:"column:uid;index:uid;type:bigint(20);not null;comment:'关联用户id'"`
	Title  string `json:"title" gorm:"column:title;index:title;type:varchar(100);not null;comment:'项目名称'"`
	Remark string `json:"remark" gorm:"column:remark;type:varchar(255);not null;comment:'项目备注'"`
}

type ProjectRelevance struct {
	ID         int64 `json:"id" gorm:"column:id;pirmary_key;auto_increment"`
	UID        int64 `json:"uid" gorm:"column:uid;index:uid;type:bigint(20);not null;comment:'关联用户id'"`
	ProjectID  int64 `json:"project_id" gorm:"column:project_id;index:project_id;type:bigint(20);not null;comment:'关联项目id'"`
	CreateTime int64 `json:"create_time" gorm:"column:create_time;type:bigint(20);not null;comment:'创建时间'"`
}

type TaskLog struct {
	ID        int64  `json:"id" gorm:"column:id;pirmary_key;auto_increment"`
	ProjectID int64  `json:"project_id" gorm:"column:project_id;index:project_id;type:bigint(20);not null;comment:'关联项目id'"`
	TaskID    string `json:"task_id" gorm:"column:task_id;index:task_id;type:varchar(32);not null;comment:'关联任务id'"`
	Project   string `json:"project" gorm:"column:project;type:varchar(100);not null;comment:'项目名称'"`

	Name      string `json:"name" gorm:"column:name;index:name;type:varchar(100);not null;comment:'任务名称'"`
	Result    string `json:"result" gorm:"column:result;type:varchar(20);not null;comment:'任务执行结果'"`
	StartTime int64  `json:"start_time" gorm:"column:start_time;type:bigint(20);not null;comment:'任务开始时间'"`
	EndTime   int64  `json:"end_time" gorm:"column:end_time;type:bigint(20);not null;comment:'任务结束时间'"`
	Command   string `json:"command" gorm:"column:command;type:varchar(255);not null;comment:'任务指令'"`
	WithError int    `json:"with_error" gorm:"column:with_error;type:int(11);not null;comment:'是否发生错误'"`
	ClientIP  string `json:"client_ip" gorm:"client_ip;index:client_ip;type:varchar(20);not null;comment:'节点ip'"`
	TmpID     string `json:"tmp_id" gorm:"column:tmp_id;type:varchar(50);not null;comment:'任务执行id'"`
}

// MonitorInfo 监控信息
type MonitorInfo struct {
	IP            string `json:"ip"`
	CpuPercent    string `json:"cpu_percent"`
	MemoryPercent string `json:"memory_percent"`
	MemoryTotal   uint64 `json:"memory_total"`
	MemoryFree    uint64 `json:"memory_free"`
}

type WebHook struct {
	CallbackURL string `json:"callback_url" gorm:"column:callback_url;type:varchar(255);not null;comment:'回调地址'"`
	ProjectID   int64  `json:"project_id" gorm:"column:project_id;type:int(11);index:project_id;not null;comment:'关联项目id'"`
	Type        string `json:"type" gorm:"column:type;type:varchar(30);not null;index:type;comment:'webhook类型'"`
	Secret      string `json:"secret" gorm:"column:secret;type:varchar(32);not null;comment:'secret'"`
	CreateTime  int64  `json:"create_time" gorm:"column:create_time;type:int(11);not null;comment:'创建时间'"`
}

type WebHookBody struct {
	TaskID      string `json:"task_id" form:"task_id"`
	ProjectID   int64  `json:"project_id" form:"project_id"`
	Command     string `json:"command" form:"command"`
	StartTime   int64  `json:"start_time" form:"start_time"`
	EndTime     int64  `json:"end_time" form:"end_time"`
	Result      string `json:"result" form:"result"`
	SystemError string `json:"system_error" form:"system_error"`
	Error       string `json:"error" form:"error"`
	ClientIP    string `json:"client_ip" form:"client_ip"`
	RequestTime int64  `json:"request_time" form:"request_time"`
	Sign        string `json:"sign,omitempty" form:"sign"`
}

type Workflow struct {
	ID         int64  `json:"id" gorm:"column:id;pirmary_key;auto_increment"`
	Title      string `json:"title" gorm:"column:title;type:varchar(100);not null;comment:'flow标题'"`
	Remark     string `json:"remark" gorm:"column:remark;type:text;not null;comment:'flow详细介绍'"`
	Cron       string `json:"cron" gorm:"column:cron;type:varchar(20);not null;comment:'cron表达式'"`
	Status     int    `json:"status" gorm:"column:status;type:tinyint(1);not null;default:2;comment:'workflow状态，1启用2暂停'"`
	CreateTime int64  `json:"create_time" gorm:"column:create_time;type:int(11);not null;comment:'创建时间'"`
}

type GetWorkflowListOptions struct {
	Title string
	IDs   []int64
}

type WorkflowTask struct {
	ID                  int64  `json:"id" gorm:"column:id;pirmary_key;auto_increment"`
	WorkflowID          int64  `json:"workflow_id" gorm:"column:workflow_id;type:int(11);not null;index:workflow_id;comment:'关联workflow id'"`
	TaskID              string `json:"task_id" gorm:"column:task_id;type:varchar(50);not null;index:task_id;comment:'task id'"`
	ProjectID           int64  `json:"project_id" gorm:"column:project_id;type:int(11);not null;index:project_id;comment:'project id'"`
	DependencyTaskID    string `json:"dependency_task_id" gorm:"column:dependency_task_id;not null;type:varchar(50);default:'';index:dependency_task_id;"`
	DependencyProjectID int64  `json:"dependency_project_id" gorm:"column:dependency_project_id;not null;type:int(11);default:'';index:dependency_project_id;comment:'依赖任务的项目id'"`
	CreateTime          int64  `json:"create_time" gorm:"column:create_time;type:int(11);not null;comment:'创建时间'"`
}

type UserWorkflowRelevance struct {
	ID         int64 `json:"id" gorm:"column:id;pirmary_key;auto_increment"`
	UserID     int64 `json:"user_id" gorm:"column:user_id;type:int(11);not null;index:user_id;comment:'关联用户id'"`
	WorkflowID int64 `json:"workflow_id" gorm:"column:workflow_id;type:int(11);not null;index:workflow_id;comment:'关联workflow id'"`
	CreateTime int64 `json:"create_time" gorm:"column:create_time;type:int(11);not null;comment:'创建时间'"`
}
