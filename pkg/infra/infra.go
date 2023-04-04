package infra

import (
	"net/http"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"

	"github.com/spacegrower/watermelon/infra"
	_ "github.com/spacegrower/watermelon/infra/balancer"
	"github.com/spacegrower/watermelon/infra/register"
	"github.com/spacegrower/watermelon/infra/resolver"
)

// RegisterETCDRegisterPrefixKey a function to change default register(etcd) prefix key
func RegisterETCDRegisterPrefixKey(prefix string) {
	infra.RegisterETCDRegisterPrefixKey(prefix)
}

// ResolveEtcdClient a function to register etcd client to watermelon global
func RegisterEtcdClient(etcdConfig clientv3.Config) error {
	return infra.RegisterEtcdClient(etcdConfig)
}

// ResolveEtcdClient a function to get registed etcd client
func ResolveEtcdClient() *clientv3.Client {
	return infra.ResolveEtcdClient()
}

// RegisterRegionProxy set region's proxy endpoint
func RegisterRegionProxy(region, proxy string) {
	infra.RegisterRegionProxy(region, proxy)
}

// ResolveProxy return region's proxy, if it exist
func ResolveProxy(region string) string {
	return infra.ResolveProxy(region)
}

// Server is a function to build grpc service
type Server[T interface {
	WithMeta(register.NodeMeta) T
}] func(register func(srv *grpc.Server), opts ...infra.Option[T]) *infra.Srv[T]

// NewServer is a function to create a server instance
func NewAgentServer() Server[NodeMetaRemote] {
	return Server[NodeMetaRemote](infra.NewServer[NodeMetaRemote])
}

// copy infra options
func (*Server[T]) WithServiceRegister(r register.ServiceRegister[NodeMetaRemote]) infra.Option[NodeMetaRemote] {
	return infra.WithServiceRegister(r)
}

func (*Server[T]) WithHttpServer(srv *http.Server) infra.Option[T] {
	return infra.WithHttpServer[T](srv)
}

func (s *Server[T]) WithGrpcServerOptions(opts ...grpc.ServerOption) infra.Option[T] {
	return infra.WithGrpcServerOptions[T](opts...)
}

func (*Server[T]) WithAddress(addr string) infra.Option[T] {
	return infra.WithAddress[T](addr)
}

func (s *Server[T]) WithTags(tags map[string]string) infra.Option[NodeMetaRemote] {
	return func(s *infra.SrvInfo[NodeMetaRemote]) {
		s.CustomInfo.Tags = tags
	}
}

// customized options
func (s *Server[T]) WithRegion(region string) infra.Option[NodeMetaRemote] {
	return func(s *infra.SrvInfo[NodeMetaRemote]) {
		s.CustomInfo.Region = region
	}
}

func (s *Server[T]) WithSystems(projectsID []int64) infra.Option[NodeMetaRemote] {
	return func(s *infra.SrvInfo[NodeMetaRemote]) {
		s.CustomInfo.Systems = projectsID
	}
}

func (s *Server[T]) WithOrg(org string) infra.Option[NodeMetaRemote] {
	return func(s *infra.SrvInfo[NodeMetaRemote]) {
		s.CustomInfo.OrgID = org
	}
}

func (s *Server[T]) WithWeight(weight int32) infra.Option[NodeMetaRemote] {
	return func(s *infra.SrvInfo[NodeMetaRemote]) {
		s.CustomInfo.Weight = weight
	}
}

// ClientConn is a function to create grpc client connection
type ClientConn[T infra.ClientServiceNameGenerator] func(serviceName string, opts ...infra.ClientOptions[T]) (*grpc.ClientConn, error)

// NewClientConn is a function to create a cc instance
func NewClientConn() ClientConn[ResolveMeta] {
	return ClientConn[ResolveMeta](infra.NewClientConn[ResolveMeta])
}

func (c *ClientConn[T]) WithServiceResolver(r resolver.Resolver) infra.ClientOptions[T] {
	return infra.WithServiceResolver[T](r)
}

func (c *ClientConn[T]) WithDialTimeout(t time.Duration) infra.ClientOptions[T] {
	return infra.WithDialTimeout[T](t)
}

func (c *ClientConn[T]) WithGrpcDialOptions(opts ...grpc.DialOption) infra.ClientOptions[T] {
	return infra.WithGrpcDialOptions[T](opts...)
}

func (*ClientConn[T]) WithOrg(id string) infra.ClientOptions[ResolveMeta] {
	return func(c *infra.COptions[ResolveMeta]) {
		c.CustomizeMeta.OrgID = id
	}
}

func (*ClientConn[T]) WithSystem(projectID int64) infra.ClientOptions[ResolveMeta] {
	return func(c *infra.COptions[ResolveMeta]) {
		c.CustomizeMeta.System = projectID
	}
}

func (*ClientConn[T]) WithRegion(region string) infra.ClientOptions[ResolveMeta] {
	return func(c *infra.COptions[ResolveMeta]) {
		c.CustomizeMeta.Region = region
	}
}

type CenterServer[T interface {
	WithMeta(register.NodeMeta) T
}] func(register func(srv *grpc.Server), opts ...infra.Option[T]) *infra.Srv[T]

// NewServer is a function to create a server instance
func NewCenterServer() CenterServer[NodeMeta] {
	return CenterServer[NodeMeta](infra.NewServer[NodeMeta])
}

// copy infra options
func (*CenterServer[T]) WithServiceRegister(r register.ServiceRegister[NodeMeta]) infra.Option[NodeMeta] {
	return infra.WithServiceRegister(r)
}

func (*CenterServer[T]) WithHttpServer(srv *http.Server) infra.Option[T] {
	return infra.WithHttpServer[T](srv)
}

func (s *CenterServer[T]) WithGrpcServerOptions(opts ...grpc.ServerOption) infra.Option[T] {
	return infra.WithGrpcServerOptions[T](opts...)
}

func (*CenterServer[T]) WithAddress(addr string) infra.Option[T] {
	return infra.WithAddress[T](addr)
}

// customized options
func (s *CenterServer[T]) WithRegion(region string) infra.Option[NodeMeta] {
	return func(s *infra.SrvInfo[NodeMeta]) {
		s.CustomInfo.Region = region
	}
}

func (s *CenterServer[T]) WithSystems(projectID int64) infra.Option[NodeMeta] {
	return func(s *infra.SrvInfo[NodeMeta]) {
		s.CustomInfo.System = projectID
	}
}

func (s *CenterServer[T]) WithOrg(org string) infra.Option[NodeMeta] {
	return func(s *infra.SrvInfo[NodeMeta]) {
		s.CustomInfo.OrgID = org
	}
}

func (s *CenterServer[T]) WithWeight(weight int32) infra.Option[NodeMetaRemote] {
	return func(s *infra.SrvInfo[NodeMetaRemote]) {
		s.CustomInfo.Weight = weight
	}
}
