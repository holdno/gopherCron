package common

const (
	TASK_EVENT_SAVE      = 1
	TASK_EVENT_DELETE    = 2
	TASK_EVENT_KILL      = 3
	TASK_EVENT_TEMPORARY = 4

	TASK_STATUS_START = 1

	APP_KEY = "app_impl"
	USER_ID = "user_id"

	CLUSTER_AUTO_INDEX = "gopherCron_cluster_key"

	MonitorFrequency = 5

	// Database
	ADMIN_USER_ACCOUNT    = "admin"
	ADMIN_USER_PASSWORD   = "123456"
	ADMIN_USER_PERMISSION = "admin,user"
	ADMIN_USER_NAME       = "administrator"
	ADMIN_PROJECT         = "admin_project"

	// Log
	ErrorLog   = 1
	SuccessLog = 0
)

var (
	// common
	LocalIP = ""
)
