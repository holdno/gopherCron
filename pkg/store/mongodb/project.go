package mongodb

import (
	"context"

	"github.com/holdno/gopherCron/utils"

	"github.com/mongodb/mongo-go-driver/bson/primitive"

	"github.com/mongodb/mongo-go-driver/bson"
	"github.com/mongodb/mongo-go-driver/mongo"

	"github.com/holdno/gopherCron/errors"

	"github.com/holdno/gopherCron/common"
)

const (
	ProjectTable = "project"
)

func CreateProject(obj *common.Project) error {
	var (
		err    error
		errObj errors.Error
		ctx    context.Context
	)

	ctx, _ = utils.GetContextWithTimeout()

	if obj.ProjectID.IsZero() {
		obj.ProjectID = primitive.NewObjectID()
	}

	if _, err = Database.Collection(ProjectTable).InsertOne(ctx, obj); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Project - CreateProject] InsertOne error:" + err.Error()
		return err
	}

	return nil
}

func UpdateProject(obj *common.Project) error {
	var (
		err    error
		errObj errors.Error
		res    *mongo.UpdateResult
		ctx    context.Context
	)

	ctx, _ = utils.GetContextWithTimeout()

	if obj.ProjectID.IsZero() {
		return errors.ErrInvalidArgument
	}

	res, err = Database.Collection(ProjectTable).UpdateOne(ctx,
		bson.M{"_id": obj.ProjectID},
		bson.M{"$set": bson.M{"title": obj.Title, "remark": obj.Remark}})

	if err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[User - UpdateProject] UpdateOne error:" + err.Error()
		return errObj
	}

	if res.ModifiedCount < 1 {
		return errors.ErrDataNotFound
	}

	return nil
}

func CheckProjectExist(project, uid primitive.ObjectID) (*common.Project, error) {
	var (
		err         error
		errObj      errors.Error
		res         *mongo.SingleResult
		projectResp common.Project
	)

	res = Database.Collection(ProjectTable).FindOne(context.TODO(), bson.M{"_id": project, "relation": uid})

	if err = res.Decode(&projectResp); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.ErrProjectNotExist
		}
		errObj = errors.ErrInternalError
		errObj.Log = "[Project - CheckProjectExist] FindOne error:" + err.Error()
		return nil, errObj
	}
	return &projectResp, nil
}

func CheckUserProject(projectID, userID primitive.ObjectID) (*common.Project, error) {
	var (
		err         error
		errObj      errors.Error
		res         *mongo.SingleResult
		projectResp common.Project
	)

	res = Database.Collection(ProjectTable).FindOne(context.TODO(), bson.M{"_id": projectID, "uid": userID})

	if err = res.Decode(&projectResp); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.ErrProjectNotExist
		}
		errObj = errors.ErrInternalError
		errObj.Log = "[Project - CheckUserProject] FindOne error:" + err.Error()
		return nil, errObj
	}
	return &projectResp, nil
}

func CheckProjectExistByName(title string) (*common.Project, error) {
	var (
		err         error
		errObj      errors.Error
		res         *mongo.SingleResult
		projectResp common.Project
	)

	res = Database.Collection(ProjectTable).FindOne(context.TODO(), bson.M{"title": title})

	if err = res.Decode(&projectResp); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.ErrProjectNotExist
		}
		errObj = errors.ErrInternalError
		errObj.Log = "[Project - CheckProjectExistByName] FindOne error:" + err.Error()
		return nil, errObj
	}
	return &projectResp, nil
}

func GetUserProjects(uid primitive.ObjectID) ([]*common.Project, error) {
	var (
		err    error
		errObj errors.Error
		list   []*common.Project
		res    mongo.Cursor
	)

	if res, err = Database.Collection(ProjectTable).Find(context.TODO(), bson.M{"relation": uid}); err != nil {
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

func DeleteProject(projectID, userID primitive.ObjectID) error {
	var (
		err    error
		errObj errors.Error
	)

	if _, err = Database.Collection(ProjectTable).DeleteOne(context.TODO(), bson.M{"_id": projectID, "uid": userID}); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Project - DeleteProject] DeleteOne error:" + err.Error()
		return errObj
	}

	return nil
}

func GetProject(id primitive.ObjectID) (*common.Project, error) {
	var (
		err         error
		errObj      errors.Error
		res         *mongo.SingleResult
		projectResp common.Project
	)

	res = Database.Collection(ProjectTable).FindOne(context.TODO(), bson.M{"_id": id})

	if err = res.Decode(&projectResp); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.ErrProjectNotExist
		}
		errObj = errors.ErrInternalError
		errObj.Log = "[Project - GetProject] FindOne error:" + err.Error()
		return nil, errObj
	}
	return &projectResp, nil
}

// 将用户加入项目成员
func AddUserToProject(projectID, userID primitive.ObjectID) error {
	var (
		err    error
		errObj errors.Error
		res    *mongo.UpdateResult
		ctx    context.Context
	)

	ctx, _ = utils.GetContextWithTimeout()

	if projectID.IsZero() {
		return errors.ErrInvalidArgument
	}

	res, err = Database.Collection(ProjectTable).UpdateOne(ctx,
		bson.M{"_id": projectID},
		bson.M{"$addToSet": bson.M{"relation": userID}})

	if err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[User - RemoveUserFromProject] UpdateOne error:" + err.Error()
		return errObj
	}

	if res.ModifiedCount < 1 {
		return errors.ErrDataNotFound
	}

	return nil
}

// 将用户从项目中移出
func RemoveUserFromProject(projectID, userID primitive.ObjectID) error {
	var (
		err    error
		errObj errors.Error
		res    *mongo.UpdateResult
		ctx    context.Context
	)

	ctx, _ = utils.GetContextWithTimeout()

	if projectID.IsZero() {
		return errors.ErrInvalidArgument
	}

	res, err = Database.Collection(ProjectTable).UpdateOne(ctx,
		bson.M{"_id": projectID},
		bson.M{"$pull": bson.M{"relation": userID}})

	if err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[User - RemoveUserFromProject] UpdateOne error:" + err.Error()
		return errObj
	}

	if res.ModifiedCount < 1 {
		return errors.ErrDataNotFound
	}

	return nil
}
