package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/holdno/gocommons/selection"
	"github.com/holdno/gopherCron/pkg/cronpb"
	"github.com/holdno/gopherCron/pkg/infra"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/spacegrower/watermelon/infra/wlog"
	"go.uber.org/zap"
)

func (a *app) StreamManager() *streamManager[*cronpb.Event] {
	return a.streamManager
}

func (a *app) StreamManagerV2() *streamManager[*cronpb.ServiceEvent] {
	return a.streamManagerV2
}

type streamManager[T any] struct {
	aliveSrv        map[string]map[string]*Stream[T]
	hostIndex       map[string]streamHostIndex
	streamStoreLock sync.RWMutex
	isV2            bool
	*responseMap
}

type streamResponse struct {
	isClose bool
	recv    chan *cronpb.ClientEvent
	mu      *sync.Mutex
}

type streamHostIndex struct {
	one string
	two string
}

type Stream[T any] struct {
	stream interface {
		Send(T) error
	}
	cancelFunc  func()
	CreateTime  time.Time
	Host        string
	Port        int32
	ServiceName string
	Region      string
	System      int64
}

func (s *Stream[T]) Cancel() {
	if s.cancelFunc != nil {
		s.cancelFunc()
	}
}

func (s *Stream[T]) Send(e T) error {
	return s.stream.Send(e)
}

func (c *streamManager[T]) generateKey(projectID int64, srvName string) string {
	return fmt.Sprintf("%d/%s", projectID, srvName)
}

func (c *streamManager[T]) generateServiceKey(info infra.NodeMeta) string {
	return fmt.Sprintf("%s_%d_%s_%s_%d_%d", info.Region, info.System, info.ServiceName, info.Host, info.Port, info.RegisterTime)
}

func (c *streamManager[T]) SaveStreamV2(info infra.NodeMeta, stream interface {
	Send(T) error
}, cancelFunc func()) {
	c.streamStoreLock.Lock()
	defer c.streamStoreLock.Unlock()
	k := c.generateKey(info.System, info.ServiceName)
	srvList, exist := c.aliveSrv[k]
	if !exist {
		srvList = make(map[string]*Stream[T])
		c.aliveSrv[k] = srvList
	}

	kk := c.generateServiceKey(info)
	srvList[kk] = &Stream[T]{
		cancelFunc:  cancelFunc,
		stream:      stream,
		CreateTime:  time.Unix(0, info.RegisterTime),
		Host:        info.Host,
		Port:        int32(info.Port),
		ServiceName: info.ServiceName,
		Region:      info.Region,
		System:      info.System,
	}

	c.hostIndex[fmt.Sprintf("%s:%d", info.Host, info.Port)] = streamHostIndex{
		one: k,
		two: kk,
	}
}

func (c *streamManager[T]) SaveStream(info infra.NodeMeta, stream interface {
	Send(T) error
}, cancelFunc func()) {
	c.streamStoreLock.Lock()
	defer c.streamStoreLock.Unlock()
	k := c.generateKey(info.System, info.ServiceName)
	srvList, exist := c.aliveSrv[k]
	if !exist {
		srvList = make(map[string]*Stream[T])
		c.aliveSrv[k] = srvList
	}

	kk := c.generateServiceKey(info)
	srvList[kk] = &Stream[T]{
		cancelFunc:  cancelFunc,
		stream:      stream,
		CreateTime:  time.Unix(0, info.RegisterTime),
		Host:        info.Host,
		Port:        int32(info.Port),
		ServiceName: info.ServiceName,
		Region:      info.Region,
		System:      info.System,
	}

	c.hostIndex[fmt.Sprintf("%s:%d", info.Host, info.Port)] = streamHostIndex{
		one: k,
		two: kk,
	}
}

func (c *streamManager[T]) RemoveStream(info infra.NodeMeta) {
	c.streamStoreLock.Lock()
	defer c.streamStoreLock.Unlock()
	k := c.generateKey(info.System, info.ServiceName)
	srvList, exist := c.aliveSrv[k]
	if !exist {
		return
	}
	delete(srvList, c.generateServiceKey(info))
	if len(srvList) == 0 {
		delete(c.aliveSrv, k)
	}
}

func (c *streamManager[T]) GetStreamsByHost(host string) *Stream[T] {
	if index, exist := c.hostIndex[host]; exist {
		if levelOne, exist := c.aliveSrv[index.one]; exist {
			return levelOne[index.two]
		}
	}
	return nil
}

func (c *streamManager[T]) GetStreams(system int64, srvName string) map[string]*Stream[T] {
	c.streamStoreLock.RLock()
	defer c.streamStoreLock.RUnlock()
	srvList, exist := c.aliveSrv[c.generateKey(system, srvName)]
	if !exist {
		return nil
	}
	// copy map
	copiedSrvList := make(map[string]*Stream[T], len(srvList))
	for k, v := range srvList {
		copiedSrvList[k] = v
	}
	return copiedSrvList
}

type AgentProjectCompareInfo struct {
	Hash       string
	LatestTime int64
	BeforAddrs []string
}

func (a *app) CalcAgentDataConsistency(done <-chan struct{}) error {
	c := time.NewTicker(time.Minute)
	defer c.Stop()
	calc := func() error {
		mtimer := a.metrics.CustomHistogramSet("calc_agent_data_consistency")
		defer mtimer.ObserveDuration()
		list, err := a.store.Project().GetProject(selection.NewSelector())
		if err != nil {
			return err
		}
		var needToRemove []string
		checkOne := func(projectID int64) error {
			var compareData *AgentProjectCompareInfo
			agents, err := a.FindAgents(a.GetConfig().Micro.Region, projectID)
			if err != nil {
				return err
			}
			defer func() {
				for _, v := range agents {
					v.Close()
				}
			}()
			if len(agents) <= 1 {
				return nil
			}
			for _, agent := range agents {
				resp, err := agent.ProjectTaskHash(context.TODO(), &cronpb.ProjectTaskHashRequest{
					ProjectId: projectID,
				})
				if err != nil {
					return err
				}
				if compareData == nil {
					compareData = &AgentProjectCompareInfo{
						Hash:       resp.Hash,
						LatestTime: resp.LatestUpdateTime,
						BeforAddrs: []string{agent.addr},
					}
				} else if resp.Hash != compareData.Hash {
					wlog.Warn("got different task hash", zap.String("current_agent", agent.addr), zap.String("before", compareData.Hash), zap.String("after", resp.Hash))
					if resp.LatestUpdateTime <= compareData.LatestTime {
						needToRemove = append(needToRemove, agent.addr)
						compareData.BeforAddrs = append(compareData.BeforAddrs, agent.addr)
					} else {
						compareData.LatestTime = resp.LatestUpdateTime
						compareData.Hash = resp.Hash
						needToRemove = append(needToRemove, compareData.BeforAddrs...)
						compareData.BeforAddrs = nil
					}
				}
			}
			return nil
		}
		for _, v := range list {
			checkOne(v.ID)
		}

		// centerSrvs, err := a.GetCenterSrvList()
		// if err != nil {
		// 	return err
		// }

		// for _, center := range centerSrvs {
		// 	for _, addr := range needToRemove {
		// 		_, err := center.RemoveStream(context.TODO(), &cronpb.RemoveStreamRequest{
		// 			Client: addr,
		// 		})
		// 		if err != nil {
		// 			center.Close()
		// 			return err
		// 		}
		// 	}
		// 	center.Close()
		// }
		return nil
	}

	for {
		select {
		case <-c.C:
			if err := calc(); err != nil {
				a.metrics.CustomInc("calc_agent_data_consistency", a.GetIP(), err.Error())
				return err
			}
		case <-done:
			return nil
		case <-a.ctx.Done():
			return nil
		}
	}
}

type responseMap struct {
	m cmap.ConcurrentMap[string, *streamResponse]
}

func (rm *responseMap) Set(ctx context.Context, id string) chan *cronpb.ClientEvent {
	timeouter := time.NewTimer(time.Second * 5)
	responser := &streamResponse{
		recv: make(chan *cronpb.ClientEvent, 1),
		mu:   &sync.Mutex{},
	}
	rm.m.Set(id, responser)
	go func() {
		for {
			select {
			case <-ctx.Done():
			case <-timeouter.C:
			}
			timeouter.Stop()
			responser.mu.Lock()
			responser.isClose = true
			close(responser.recv)
			rm.Remove(id)
			responser.mu.Unlock()
			return
		}
	}()
	return responser.recv
}

func (rm *responseMap) Remove(id string) {
	rm.m.Remove(id)
}

func (s *streamManager[T]) RecvStreamResponse(event *cronpb.ClientEvent) {
	resp, exist := s.responseMap.m.Get(event.Id)
	if !exist {
		return
	}

	if !resp.mu.TryLock() {
		// 加锁可能是正在清理中，直接丢弃数据
		return
	}
	defer resp.mu.Unlock()
	if resp.isClose {
		return
	}

	select {
	case resp.recv <- event:
		resp.isClose = true
	default:
		wlog.Error("stream receive chan is full", zap.String("id", event.Id))
		panic("stream receive chan is full")
	}
}

func (s *streamManager[T]) SendEventWaitResponse(ctx context.Context, stream *Stream[*cronpb.ServiceEvent], event *cronpb.ServiceEvent) (*cronpb.ClientEvent, error) {
	if !s.isV2 {
		return nil, nil
	}
	resp := s.responseMap.Set(ctx, event.Id)
	defer s.responseMap.Remove(event.Id)
	if err := stream.Send(event); err != nil {
		return nil, err
	}
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case result, ok := <-resp:
			if !ok {
				return nil, fmt.Errorf("request time out")
			}
			if result.Error != nil && result.Error.Error != "" {
				return nil, errors.New(result.Error.Error)
			}
			return result, nil
		}
	}
}
