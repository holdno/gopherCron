package common

import "github.com/mongodb/mongo-go-driver/bson/primitive"

type User struct {
	ID         string   `json:"id" bson:"_id"`
	Name       string   `json:"name" bson:"name"`
	Permission string   `json:"permission" bson:"permission"`
	Account    string   `json:"account" bson:"account"`
	Password   string   `json:"-" bson:"password"`
	Salt       string   `json:"-" bson:"salt"`
	CreateTime int64    `json:"create_time" bson:"create_time"`
	Project    []string `json:"project" bson:"project"`
}

type Project struct {
	UID     string `json:"uid" bson:"uid"`
	Title   string `json:"title" bson:"title"`
	Project string `json:"project" bson:"project"`
	Remark  string `json:"remark" bson:"remark"`
}

type TaskLog struct {
	ObjID     primitive.ObjectID `bson:"_id,omitempty"`
	ID        string             `json:"id"`
	Project   string             `json:"project" bson:"project"`
	Name      string             `json:"name" bson:"name"`
	Result    string             `json:"result" bson:"result"`
	StartTime int64              `json:"start_time" bson:"start_time"`
	EndTime   int64              `json:"end_time" bson:"end_time"`
	Command   string             `json:"command" bson:"command"`
}
