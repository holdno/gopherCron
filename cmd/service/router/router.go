package router

import (
	"fmt"
	"net/http"

	"github.com/holdno/gopherCron/cmd/service/controller"
	"github.com/holdno/gopherCron/cmd/service/controller/etcd_func"
	"github.com/holdno/gopherCron/cmd/service/controller/log_func"
	"github.com/holdno/gopherCron/cmd/service/controller/project_func"
	"github.com/holdno/gopherCron/cmd/service/controller/system"
	"github.com/holdno/gopherCron/cmd/service/controller/user_func"
	"github.com/holdno/gopherCron/cmd/service/middleware"
	"github.com/holdno/gopherCron/config"
	"github.com/holdno/gopherCron/utils"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/holdno/firetower/protocol"
	"github.com/holdno/firetower/service/tower"
)

func SetupRoute(r *gin.Engine, conf *config.DeployConf) {
	r.Use(gin.Recovery())
	r.Use(middleware.CrossDomain())
	r.Use(middleware.BuildResponse())

	api := r.Group("/api/v1")
	{
		api.GET("/ws", Websocket)
		api.GET("/version", system.GetVersion)
		user := api.Group("/user")
		{
			user.POST("/login", user_func.Login)
			user.Use(middleware.TokenVerify())
			user.GET("/info", user_func.GetUserInfo)
			user.POST("/change_password", user_func.ChangePassword)
			user.POST("/create", user_func.CreateUser)
			user.POST("/delete", user_func.DeleteUser)
			user.GET("/list", user_func.GetUserList)
		}

		webhook := api.Group("/webhook")
		{
			webhook.Use(middleware.TokenVerify())
			webhook.POST("/create", controller.CreateWebHook)
			webhook.POST("/delete", controller.DeleteWebHook)
			webhook.GET("/list", controller.GetWebHookList)
			webhook.GET("/info", controller.GetWebHook)
		}

		cron := api.Group("/crontab")
		{
			cron.Use(middleware.TokenVerify())
			cron.POST("/save", etcd_func.SaveTask)
			cron.POST("/delete", etcd_func.DeleteTask)
			cron.GET("/list", etcd_func.GetTaskList)
			cron.POST("/kill", etcd_func.KillTask)
			cron.POST("/execute", etcd_func.ExecuteTask)
			cron.GET("/client_list", etcd_func.GetClientList)
			cron.POST("/monitor", etcd_func.GetWorkerListInfo)
			service := cron.Group("/tmp")
			{
				service.POST("/execute", etcd_func.TmpExecute)
			}
		}

		temporaryTask := api.Group("/temporary_task")
		{
			temporaryTask.Use(middleware.TokenVerify())
			temporaryTask.POST("/create", controller.CreateTemporaryTask)
			temporaryTask.GET("/list", controller.GetTemporaryTaskList)
		}

		workflow := api.Group("/workflow")
		{
			workflow.Use(middleware.TokenVerify())
			workflow.POST("/create", controller.CreateWorkflow)
			workflow.POST("/delete", controller.DeleteWorkflow)
			workflow.POST("/update", controller.UpdateWorkflow)
			workflow.GET("/list", controller.GetWorkflowList)
			workflow.GET("/detail", controller.GetWorkflow)
			workflow.POST("/start", controller.StartWorkflow)
			workflow.POST("/kill", controller.KillWorkflow)
			manage := workflow.Group("/manage")
			{
				manage.POST("/add_user", controller.WorkflowAddUser)
				manage.POST("/remove_user", controller.WorkflowRemoveUser)
				manage.GET("/users", controller.GetUsersByWorkflow)
			}
			task := workflow.Group("/task")
			{
				task.POST("/schedule/create", controller.CreateWorkflowSchedulePlan)
				task.GET("/list", controller.GetWorkflowTaskList)
			}
			log := workflow.Group("/log")
			{
				log.GET("/list", controller.GetWorkflowLogList)
				log.POST("/clear", controller.ClearWorkflowLog)
			}
		}

		worker := api.Group("/client")
		{
			worker.Use(middleware.TokenVerify())
			worker.GET("/list", etcd_func.GetWorkerListInfo)
			worker.POST("/reload/config", etcd_func.ReloadConfig)
		}

		project := api.Group("/project")
		{
			project.Use(middleware.TokenVerify())
			project.POST("/create", project_func.Create)
			project.GET("/list", project_func.GetUserProjects)
			project.POST("/update", project_func.Update)
			project.POST("/delete", project_func.DeleteOne)
			project.GET("/users", user_func.GetUsersUnderTheProject)
			project.POST("/remove_user", project_func.RemoveUser)
			project.POST("/add_user", project_func.AddUser)
			workflow := project.Group("/workflow")
			{
				workflow.GET("/task/list", project_func.GetProjectWorkflowTasks)
				workflow.POST("/task/create", project_func.CreateProjectWorkflowTask)
				workflow.POST("/task/delete", project_func.DeleteProjectWorkflowTask)
				workflow.POST("/task/update", project_func.UpdateProjectWorkflowTask)
			}
		}

		log := api.Group("/log")
		{
			log.Use(middleware.TokenVerify())
			log.GET("/list", log_func.GetList)
			log.GET("/detail", log_func.GetLogDetail)
			log.POST("/clean", log_func.CleanLogs)
			log.GET("/recent", log_func.GetRecentLogCount)
			log.GET("/errors", log_func.GetErrorLogs)
		}

		r.NoRoute(func(c *gin.Context) {
			c.String(http.StatusOK, "no route found")
		})
	}

	if conf.ViewPath == "" {
		conf.ViewPath = "./view"
	}
	r.StaticFS("/admin", http.Dir(conf.ViewPath))
	r.StaticFile("/favicon.ico", conf.ViewPath+"/favicon.ico")

}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Websocket http转websocket连接 并实例化firetower
func Websocket(c *gin.Context) {
	// 做用户身份验证

	// 验证成功才升级连接
	ws, _ := upgrader.Upgrade(c.Writer, c.Request, nil)

	towerSvc := tower.BuildTower(ws, utils.GetStrID())

	towerSvc.SetReadHandler(func(fire *protocol.FireInfo) bool {
		return false // 禁止客户端与socket通信，仅做接收方
		// 做发送验证
		// 判断发送方是否有权限向到达方发送内容
		towerSvc.Publish(fire)
		return true
	})

	towerSvc.SetSubscribeHandler(func(context protocol.FireLife, topic []string) bool {
		for _, v := range topic {
			num := towerSvc.GetConnectNum(v)
			// 继承订阅消息的context
			var pushmsg = tower.NewFire("system", towerSvc)
			pushmsg.Message.Topic = v
			pushmsg.Message.Data = []byte(fmt.Sprintf("{\"type\":\"onSubscribe\",\"data\":%d}", num))
			towerSvc.Publish(pushmsg)
		}
		return true
	})

	towerSvc.SetUnSubscribeHandler(func(context protocol.FireLife, topic []string) bool {
		for _, v := range topic {
			num := towerSvc.GetConnectNum(v)
			var pushmsg = tower.NewFire("system", towerSvc)
			pushmsg.Message.Topic = v
			pushmsg.Message.Data = []byte(fmt.Sprintf("{\"type\":\"onUnsubscribe\",\"data\":%d}", num))
			towerSvc.Publish(pushmsg)
		}
		return true
	})

	towerSvc.Run()
}
