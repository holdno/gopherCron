package common

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
	TaskID    string `json:"task_id" gorm:"column:task_id;index:task_id;type:bigint(20);not null;comment:'关联任务id'"`
	Project   string `json:"project" gorm:"column:project;type:varchar(100);not null;comment:'项目名称'"`

	Name      string `json:"name" gorm:"column:name;index:name;type:varchar(100);not null;comment:'任务名称'"`
	Result    string `json:"result" gorm:"column:result;type:varchar(20);not null;comment:'任务执行结果'"`
	StartTime int64  `json:"start_time" gorm:"column:start_time;type:bigint(20);not null;comment:'任务开始时间'"`
	EndTime   int64  `json:"end_time" gorm:"column:end_time;type:bigint(20);not null;comment:'任务结束时间'"`
	Command   string `json:"command" gorm:"column:command;type:varchar(255);not null;comment:'任务指令'"`
	WithError int    `json:"with_error" gorm:"column:with_error;type:int(11);not null;comment:'是否发生错误'"`
	ClientIP  string `json:"client_ip" gorm:"client_ip;index:client_ip;type:varchar(20);not null;comment:'节点ip'"`
}

// MonitorInfo 监控信息
type MonitorInfo struct {
	IP            string `json:"ip"`
	CpuPercent    string `json:"cpu_percent"`
	MemoryPercent string `json:"memory_percent"`
	MemoryTotal   uint64 `json:"memory_total"`
	MemoryFree    uint64 `json:"memory_free"`
}
