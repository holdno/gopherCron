package common

const (
	TASK_EVENT_SAVE              = 1
	TASK_EVENT_DELETE            = 2
	TASK_EVENT_KILL              = 3
	TASK_EVENT_TEMPORARY         = 4
	TASK_EVENT_WORKFLOW_SCHEDULE = 5

	TASK_STATUS_START = 1
	TASK_STATUS_STOP  = 2

	TASK_STATUS_UNDEFINED   = -1
	TASK_STATUS_RUNNING     = 1
	TASK_STATUS_NOT_RUNNING = 0

	TASK_EXECUTE_NOSEIZE = 1

	TASK_STATUS_STARTING_V2    = "starting"
	TASK_STATUS_RUNNING_V2     = "running"
	TASK_STATUS_FINISHED_V2    = "finished"
	TASK_STATUS_NOT_RUNNING_V2 = ""
	TASK_STATUS_DONE_V2        = "done"
	TASK_STATUS_FAIL_V2        = "fail"

	WORKFLOW_SCHEDULE_LIMIT int = 3

	APP_KEY = "app_impl"
	USER_ID = "user_id"

	CLUSTER_AUTO_INDEX = "/gopherCron_cluster_key"

	MonitorFrequency = 5

	AGENT_COMMAND_RELOAD_CONFIG = "reload_config"

	// Database
	ADMIN_USER_ACCOUNT    = "admin"
	ADMIN_USER_PASSWORD   = "123456"
	ADMIN_USER_PERMISSION = "admin,user"
	ADMIN_USER_NAME       = "administrator"
	ADMIN_PROJECT         = "admin_project"

	WEBHOOK_TYPE_TASK_START    = "task_start"
	WEBHOOK_TYPE_TASK_FINISHED = "task_finished"

	ACK_RESPONSE_V1 = "v1"

	// Log
	ErrorLog   = 1
	SuccessLog = 0

	TEMPORARY_TASK_SCHEDULE_STATUS_WAITING   = 1
	TEMPORARY_TASK_SCHEDULE_STATUS_SCHEDULED = 2
)

var (
	// common
	LocalIP = ""
)
