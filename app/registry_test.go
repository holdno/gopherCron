package app

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/holdno/gopherCron/pkg/cronpb"
	"github.com/holdno/gopherCron/pkg/infra"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/spacegrower/watermelon/infra/register"
)

func TestStream(t *testing.T) {
	store := &streamManager[*cronpb.ServiceEvent]{
		isV2:      true,
		aliveSrv:  make(map[string]map[string]*Stream[*cronpb.ServiceEvent]),
		hostIndex: make(map[string]streamHostIndex),
		responseMap: &responseMap{
			m: cmap.New[*streamResponse](),
		},
	}

	meta := infra.NodeMeta{
		NodeMeta: register.NodeMeta{
			ServiceName: "test-service",
			GrpcMethods: make([]register.GrpcMethodInfo, 0),
			Host:        "127.0.0.1",
			Port:        6306,
			Runtime:     "go1.20.0",
			Version:     "v2.4.3",
		},
		CenterServiceEndpoint: "127.0.0.1:6306",
		CenterServiceRegion:   "center",
		OrgID:                 "gophercron",
		Region:                "center",
		Weight:                100,
		System:                3,
		Tags:                  nil,
		RegisterTime:          time.Now().UnixNano(),
	}
	_, cancel := context.WithCancel(context.Background())
	store.SaveStream(meta, nil, cancel)

	// store.RemoveStream(meta)

	for _, v := range store.GetStreams(37, "test-service") {

		fmt.Println(v.Host)

	}
}
