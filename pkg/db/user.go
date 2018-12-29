package db

import (
	"context"
	"fmt"
	"time"

	"ojbk.io/gopherCron/errors"

	"github.com/mongodb/mongo-go-driver/bson"
	"github.com/mongodb/mongo-go-driver/mongo"

	"github.com/mongodb/mongo-go-driver/bson/primitive"

	"github.com/sirupsen/logrus"

	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/utils"
)

const (
	UserTable = "user"
)

func createAdminUser() error {
	var (
		salt string
		err  error
	)

	salt = utils.RandomStr(6)
	if _, err = Database.Collection(UserTable).InsertOne(context.TODO(), &common.User{
		ID:         primitive.NewObjectID().Hex(),
		Account:    common.ADMIN_USER_ACCOUNT,
		Password:   utils.BuildPassword(common.ADMIN_USER_PASSWORD, salt),
		Salt:       salt,
		Name:       common.ADMIN_USER_NAME,
		Permission: common.ADMIN_USER_PERMISSION,
		CreateTime: time.Now().Unix(),
		Project:    []string{common.ADMIN_PROJECT},
	}); err != nil {
		logrus.WithField("Error", err).Error("goperCron create admin user error")
		return err
	}

	return nil
}

func GetUserWithAccount(account string) (*common.User, error) {
	var (
		res    *mongo.SingleResult
		user   *common.User
		errObj errors.Error
		err    error
	)
	res = Database.Collection(UserTable).FindOne(context.TODO(), bson.M{"account": account})
	if res.Err() != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[User - GetUserWithAccount] FindOne error:" + res.Err().Error()
		return nil, errObj
	}

	if err = res.Decode(&user); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		errObj = errors.ErrInternalError
		errObj.Log = "[DB - GetUserWithAccount] get user with account error:" + err.Error()
		fmt.Println(errObj.Log)
		return nil, errObj
	}

	return user, nil
}

func GetUserInfo(uid string) (*common.User, error) {
	var (
		res    *mongo.SingleResult
		user   *common.User
		errObj errors.Error
		err    error
	)
	res = Database.Collection(UserTable).FindOne(context.TODO(), bson.M{"_id": uid})
	if res.Err() != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[User - GetUserWithAccount] FindOne error:" + res.Err().Error()
		return nil, errObj
	}

	if err = res.Decode(&user); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		errObj = errors.ErrInternalError
		errObj.Log = "[DB - GetUserWithAccount] get user with account error:" + err.Error()
		fmt.Println(errObj.Log)
		return nil, errObj
	}

	return user, nil
}
