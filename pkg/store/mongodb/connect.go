package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/config"

	"github.com/mongodb/mongo-go-driver/mongo"
	"github.com/mongodb/mongo-go-driver/mongo/options"
)

var Database *mongo.Database

func MustSetup(_conf *config.ServiceConfig) {
	var (
		client        *mongo.Client
		err           error
		timeoutCtx    context.Context
		opt           *options.ClientOptions
		databaseExist bool
	)

	conf := _conf.MongoDB

	timeoutCtx, _ = context.WithTimeout(context.TODO(), time.Duration(10)*time.Second)
	opt = &options.ClientOptions{}
	opt.SetAuth(options.Credential{
		Username:      conf.Username,
		Password:      conf.Password,
		AuthSource:    conf.Table,
		AuthMechanism: conf.AuthMechanism,
	})
	if client, err = mongo.Connect(timeoutCtx, conf.Service[0], opt); err != nil {
		panic("[MongoDB - Connect] Connect error:" + err.Error())
	}

	//if dbTables, err = client.ListDatabaseNames(timeoutCtx, nil); err != nil {
	//	panic("[MongoDB - Connect] ListDatabaseNames error:" + err.Error())
	//}
	//
	//for _, table = range dbTables {
	//	if table == conf.Table {
	Database = client.Database(conf.Table)

	user, err := GetUserWithAccount(common.ADMIN_USER_ACCOUNT)
	if err != nil {
		panic(err)
	}

	if user != nil {
		databaseExist = true
	}
	//	}
	//}

	if !databaseExist {
		fmt.Println("Start init database")
		// 执行安装
		if err = createAdminUser(); err != nil {
			panic("[GoperCron - createAdminUser] createAdminUser error:" + err.Error())
		}

		fmt.Println("Init database successful")
	}
}
