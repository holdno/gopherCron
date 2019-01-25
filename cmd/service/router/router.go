package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"ojbk.io/gopherCron/cmd/service/controller/etcd_func"
	"ojbk.io/gopherCron/cmd/service/controller/log_func"
	"ojbk.io/gopherCron/cmd/service/controller/project_func"
	"ojbk.io/gopherCron/cmd/service/controller/user_func"
	"ojbk.io/gopherCron/cmd/service/middleware"
)

func SetupRoute(r *gin.Engine) {
	r.Use(gin.Recovery())
	r.Use(middleware.CrossDomain())
	r.Use(middleware.BuildResponse())

	api := r.Group("/api/v1")
	{
		user := api.Group("/user")
		{
			user.POST("/login", user_func.Login)
			user.Use(middleware.TokenVerify())
			user.GET("/info", user_func.GetUserInfo)
			user.POST("/change_password", user_func.ChangePassword)
			user.POST("/create", user_func.CreateUser)
		}

		cron := api.Group("/crontab")
		{
			cron.Use(middleware.TokenVerify())
			cron.POST("/save", etcd_func.SaveTask)
			cron.POST("/delete", etcd_func.DeleteTask)
			cron.GET("/list", etcd_func.GetTaskList)
			cron.POST("/kill", etcd_func.KillTask)
			cron.POST("/execute", etcd_func.ExecuteTask)
			cron.GET("/worker_list", etcd_func.GetWorkerList)
			cron.POST("/monitor", etcd_func.GetWorkerListInfo)
		}

		project := api.Group("/project")
		{
			project.Use(middleware.TokenVerify())
			project.POST("/create", project_func.Create)
			project.GET("/list", project_func.GetUserProjects)
			project.POST("/update", project_func.Update)
			project.POST("/delete", project_func.DeleteOne)
			project.GET("/users", user_func.GetUsersByProject)
			project.POST("/remove_user", project_func.RemoveUser)
			project.POST("/add_user", project_func.AddUser)
		}

		log := api.Group("/log")
		{
			log.Use(middleware.TokenVerify())
			log.GET("/list", log_func.GetList)
			log.POST("/clean", log_func.CleanLogs)
			log.GET("/recent", log_func.GetRecentLogCount)
		}

		r.NoRoute(func(c *gin.Context) {
			c.String(http.StatusOK, "no route found")
		})
	}

	r.StaticFS("/admin", http.Dir("./view"))
	r.StaticFile("/favicon.ico", "./view/favicon.ico")
}
