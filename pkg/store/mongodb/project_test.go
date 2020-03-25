package mongodb

import (
	"context"
	"testing"

	"github.com/holdno/gopherCron/config"

	"github.com/mongodb/mongo-go-driver/bson"
	"github.com/mongodb/mongo-go-driver/bson/primitive"
)

func connectDB() {
	apiConf := config.InitServiceConfig("../../cmd/service/conf/config-dev.toml")
	MustSetup(apiConf)
}

func TestObjectID(t *testing.T) {
	t.Log(primitive.NewObjectID())
}

func TestGetUserProjects(t *testing.T) {
	connectDB()
	res, err := Database.Collection(ProjectTable).Aggregate(context.TODO(), bson.M{"$lookup": bson.M{
		"from":         ProjectTable,
		"localField":   "_id",
		"foreignField": "project_id",
		"as":           "projects",
	}})

	if err != nil {
		t.Error(err)
		return
	}

	for res.Next(context.TODO()) {
		bytes, err := res.DecodeBytes()
		if err != nil {
			t.Error("DecodeBytes error", err)
			return
		}
		t.Log(string(bytes))
	}
}
