package context

import (
	"fmt"

	"ojbk.io/gopherCron/jwt"

	"ojbk.io/gopherCron/utils"

	"ojbk.io/gopherCron/pkg/db"

	"ojbk.io/gopherCron/config"
	"ojbk.io/gopherCron/pkg/etcd"
)

func InitMasterContext(conf *config.ServiceConfig) {
	var err error
	jwt.InitJWT(conf.JWT)

	err = etcd.Connect(conf.Etcd)
	if err != nil {
		panic(err)
	}

	utils.InitIDWorker(1)

	db.Connect(conf.MongoDB)
}

func InitWorkerContext(conf *config.ServiceConfig) {
	var err error

	err = etcd.Connect(conf.Etcd)
	if err != nil {
		panic(err)
	}

	db.Connect(conf.MongoDB)

	clusterID, err := etcd.Manager.Inc("gopherCron_cluster_key")
	if err != nil {
		panic(err)
	}
	fmt.Println("ClusterID", clusterID)
	utils.InitIDWorker(1)

	// 执行器
	etcd.InitExecuter()
	fmt.Println("InitExecuter")
	// 调度器
	etcd.InitScheduler()
	fmt.Println("InitScheduler")
	// 开始watch etcd中任务的变化
	etcd.Manager.TaskWatcher(conf.Etcd.Projects)
	fmt.Println("TaskWatcher")
	// 开始watch etcd中任务的变化
	etcd.Manager.TaskKiller(conf.Etcd.Projects)
	fmt.Println("TaskKiller")
	go etcd.Manager.Register(conf.Etcd)
	fmt.Println("Register")
}
