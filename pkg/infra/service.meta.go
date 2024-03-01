package infra

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/spacegrower/watermelon/infra/definition"
	"github.com/spacegrower/watermelon/infra/register"
	"github.com/spacegrower/watermelon/infra/resolver/etcd"
	"github.com/spacegrower/watermelon/infra/utils"
	"google.golang.org/grpc/resolver"
)

// 基于 github.com/spacegrower/watermelon 实现自定义微服务框架
type NodeMetaRemote struct {
	OrgID        string            `json:"org_id"`
	Systems      []int64           `json:"systems"`
	Region       string            `json:"region"`
	Weight       int32             `json:"weight"`
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
	NodeWeight            int32             `json:"weight"`
	RegisterTime          int64             `json:"register_time"`
	Tags                  map[string]string `json:"tags"`
	CenterServiceEndpoint string            `json:"cse"`
	CenterServiceRegion   string            `json:"csr"`
	register.NodeMeta
}

func (n NodeMeta) Equal(o any) bool {
	ov, ok := o.(NodeMeta)
	if !ok {
		return false
	}

	return n.Host == ov.Host &&
		n.OrgID == ov.OrgID &&
		n.Port == ov.Port &&
		n.Region == ov.Region &&
		n.System == ov.System &&
		n.NodeWeight == ov.NodeWeight &&
		n.RegisterTime == ov.RegisterTime &&
		n.CenterServiceEndpoint == ov.CenterServiceEndpoint &&
		n.CenterServiceRegion == ov.CenterServiceRegion &&
		n.ServiceName == ov.ServiceName
}

func (n NodeMeta) Weight() int32 {
	return n.NodeWeight
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
	n.NodeWeight = utils.GetEnvWithDefault(definition.NodeWeightENVKey, n.NodeWeight, func(val string) (int32, error) {
		res, err := strconv.Atoi(val)
		if err != nil {
			return 0, err
		}
		return int32(res), nil
	})

	if n.RegisterTime == 0 {
		n.RegisterTime = time.Now().Unix()
	}

	raw, _ := json.Marshal(n)
	return string(raw)
}

func (n NodeMeta) RegisterKey() string {
	return fmt.Sprintf("%s/%d/%s/node/%s:%d", n.OrgID, n.System, n.ServiceName, n.Host, n.Port)
}

func GetNodeMetaAttribute(addr resolver.Address) (NodeMeta, bool) {
	attr, ok := etcd.GetMetaAttributes[NodeMeta](addr)
	if !ok {
		return NodeMeta{}, false
	}
	return attr, true
}
