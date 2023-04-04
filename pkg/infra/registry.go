package infra

import (
	"net/url"

	"github.com/spacegrower/watermelon/infra"
	wb "github.com/spacegrower/watermelon/infra/balancer"
	"github.com/spacegrower/watermelon/infra/register"
	eregister "github.com/spacegrower/watermelon/infra/register/etcd"
	wresolver "github.com/spacegrower/watermelon/infra/resolver"
	eresolve "github.com/spacegrower/watermelon/infra/resolver/etcd"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/resolver"
)

func MustSetupEtcdRegister() register.ServiceRegister[NodeMeta] {
	return eregister.NewEtcdRegister[NodeMeta](infra.ResolveEtcdClient())
}

func DefaultAllowFunc(query url.Values, attr NodeMeta, addr *resolver.Address) bool {
	region := query.Get("region")

	if region == "" {
		return true
	}

	if attr.Region != region {
		proxy := infra.ResolveProxy(attr.Region)
		if proxy == "" {
			return false
		}

		addr.Addr = proxy
		addr.ServerName = proxy
	}
	return true
}

func MustSetupEtcdResolver() wresolver.Resolver {
	return eresolve.NewEtcdResolver(infra.ResolveEtcdClient(), DefaultAllowFunc)
}

func init() {
	balancer.Register(base.NewBalancerBuilder(wb.WeightRobinName,
		new(wb.WeightRobinBalancer[NodeMeta]),
		base.Config{HealthCheck: true}))
}
