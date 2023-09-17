package infra

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/spacegrower/watermelon/infra/definition"
	"github.com/spacegrower/watermelon/infra/register"
	"github.com/spacegrower/watermelon/infra/utils"
)

// 基于 github.com/spacegrower/watermelon 实现自定义微服务框架
type NodeMetaRemote struct {
	OrgID        string            `json:"org_id"`
	Systems      []int64           `json:"systems"`
	Region       string            `json:"region"`
	Weight       int32             `json:"weigth"`
	RegisterTime int64             `json:"register_time"`
	Tags         map[string]string `json:"tags"`
	register.NodeMeta
}

func (n NodeMetaRemote) WithMeta(meta register.NodeMeta) NodeMetaRemote {
	n.NodeMeta = meta
	return n
}

// register node meta
type NodeMeta struct {
	OrgID                 string            `json:"org_id"`
	Region                string            `json:"region"`
	System                int64             `json:"system"`
	Weight                int32             `json:"weigth"`
	RegisterTime          int64             `json:"register_time"`
	Tags                  map[string]string `json:"tags"`
	CenterServiceEndpoint string            `json:"cse"`
	CenterServiceRegion   string            `json:"csr"`
	register.NodeMeta
}

func (n NodeMeta) Weigth() int32 {
	return n.Weight
}

func (n NodeMeta) Service() string {
	return n.ServiceName
}

func (n NodeMeta) Methods() []string {
	var methods []string
	for _, v := range n.GrpcMethods {
		methods = append(methods, v.Name)
	}
	return methods
}

func (n NodeMeta) WithMeta(meta register.NodeMeta) NodeMeta {
	n.NodeMeta = meta
	return n
}

func (n NodeMeta) Value() string {
	// customize your register value logic
	n.Weight = utils.GetEnvWithDefault(definition.NodeWeightENVKey, n.Weight, func(val string) (int32, error) {
		res, err := strconv.Atoi(val)
		if err != nil {
			return 0, err
		}
		return int32(res), nil
	})

	n.RegisterTime = time.Now().Unix()

	raw, _ := json.Marshal(n)
	return string(raw)
}

func (n NodeMeta) RegisterKey() string {
	return fmt.Sprintf("%s/%d/%s/node/%s:%d", n.OrgID, n.System, n.ServiceName, n.Host, n.Port)
}
