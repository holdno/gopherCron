package common

import (
	"github.com/mongodb/mongo-go-driver/bson/primitive"
)

type User struct {
	ID         primitive.ObjectID `json:"id" bson:"_id"`
	Name       string             `json:"name" bson:"name"`
	Permission string             `json:"permission" bson:"permission"`
	Account    string             `json:"account" bson:"account"`
	Password   string             `json:"-" bson:"password"`
	Salt       string             `json:"-" bson:"salt"`
	CreateTime int64              `json:"create_time" bson:"create_time"`
}

type Project struct {
	ProjectID primitive.ObjectID   `json:"project_id" bson:"_id"`
	UID       primitive.ObjectID   `json:"uid" bson:"uid"`
	Title     string               `json:"title" bson:"title"`
	Remark    string               `json:"remark" bson:"remark"`
	Relation  []primitive.ObjectID `json:"-" bson:"relation"`
}

type TaskLog struct {
	ID        primitive.ObjectID `json:"id" bson:"_id"`
	ProjectID primitive.ObjectID `json:"project_id" bson:"project_id"`
	TaskID    primitive.ObjectID `json:"task_id" bson:"task_id"`
	Project   string             `json:"project" bson:"project"`

	Name      string `json:"name" bson:"name"`
	Result    string `json:"result" bson:"result"`
	StartTime int64  `json:"start_time" bson:"start_time"`
	EndTime   int64  `json:"end_time" bson:"end_time"`
	Command   string `json:"command" bson:"command"`
	WithError int    `json:"with_error" bson:"with_error"`
	ClientIP  string `json:"client_ip" bson:"client_ip"`
}

// MonitorInfo 监控信息
type MonitorInfo struct {
	IP            string `json:"ip"`
	CpuPercent    string `json:"cpu_percent"`
	MemoryPercent string `json:"memory_percent"`
	MemoryTotal   uint64 `json:"memory_total"`
	MemoryFree    uint64 `json:"memory_free"`
}
