package db

import (
	"context"

	"github.com/mongodb/mongo-go-driver/bson"
	"github.com/mongodb/mongo-go-driver/mongo"

	"ojbk.io/gopherCron/errors"

	"ojbk.io/gopherCron/common"
)

const (
	ProjectTable = "project"
)

func CreateProject(obj *common.Project) error {
	var (
		err    error
		errObj errors.Error
	)

	if _, err = Database.Collection(ProjectTable).InsertOne(context.TODO(), obj); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Project - CreateProject] InsertOne error:" + err.Error()
		return err
	}

	return nil
}

func CheckProjectExist(project, uid string) (*common.Project, error) {
	var (
		err         error
		errObj      errors.Error
		res         *mongo.SingleResult
		projectResp common.Project
	)

	res = Database.Collection(ProjectTable).FindOne(context.TODO(), bson.M{"project": project, "uid": uid})

	if err = res.Decode(&projectResp); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.ErrProjectNotExist
		}
		errObj = errors.ErrInternalError
		errObj.Log = "[Project - GetOneProject] FindOne error:" + err.Error()
		return nil, errObj
	}
	return &projectResp, nil
}

func CheckProjectExistByName(project string) (*common.Project, error) {
	var (
		err         error
		errObj      errors.Error
		res         *mongo.SingleResult
		projectResp common.Project
	)

	res = Database.Collection(ProjectTable).FindOne(context.TODO(), bson.M{"project": project})

	if err = res.Decode(&projectResp); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.ErrProjectNotExist
		}
		errObj = errors.ErrInternalError
		errObj.Log = "[Project - GetOneProject] FindOne error:" + err.Error()
		return nil, errObj
	}
	return &projectResp, nil
}

func GetUserProjects(uid string) ([]*common.Project, error) {
	var (
		err    error
		errObj errors.Error
		list   []*common.Project
		res    mongo.Cursor
	)

	if res, err = Database.Collection(ProjectTable).Find(context.TODO(), bson.M{"uid": uid}); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		errObj = errors.ErrInternalError
		errObj.Log = "[Project - GetUserProjects] Find error:" + err.Error()
		return nil, errObj
	}

	for res.Next(context.TODO()) {
		var item common.Project
		if err = res.Decode(&item); err != nil {
			errObj = errors.ErrInternalError
			errObj.Log = "[Project - GetUserProjects] convert error:" + err.Error()
			return nil, errObj
		}

		list = append(list, &item)
	}

	return list, nil
}

func DeleteProject(project, uid string) error {
	var (
		err    error
		errObj errors.Error
	)

	if _, err = Database.Collection(ProjectTable).DeleteOne(context.TODO(), bson.M{"project": project, "uid": uid}); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Project - DeleteProject] DeleteOne error:" + err.Error()
		return errObj
	}

	return nil
}
