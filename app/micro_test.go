package app

import (
	"testing"

	"github.com/holdno/gopherCron/pkg/infra"
	"google.golang.org/grpc/resolver"
)

func TestChooseNode(t *testing.T) {
	var result []*FinderResult

	result = append(result, &FinderResult{
		addr: resolver.Address{
			Addr: "1",
		},
		attr: infra.NodeMeta{
			Weight: 10,
		},
	}, &FinderResult{
		addr: resolver.Address{
			Addr: "2",
		},
		attr: infra.NodeMeta{
			Weight: 20,
		},
	}, &FinderResult{
		addr: resolver.Address{
			Addr: "3",
		},
		attr: infra.NodeMeta{
			Weight: 30,
		},
	}, &FinderResult{
		addr: resolver.Address{
			Addr: "4",
		},
		attr: infra.NodeMeta{
			Weight: 40,
		},
	}, &FinderResult{
		addr: resolver.Address{
			Addr: "5",
		},
		attr: infra.NodeMeta{
			Weight: 50,
		},
	})

	for i := 0; i < 10; i++ {
		r := ChooseNode(result)
		t.Log(r.addr.Addr)
	}
}
