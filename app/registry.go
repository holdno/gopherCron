package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/holdno/gocommons/selection"
	"github.com/holdno/gopherCron/pkg/cronpb"
	"github.com/holdno/gopherCron/pkg/infra"
	"github.com/spacegrower/watermelon/infra/wlog"
	"go.uber.org/zap"
)

func (a *app) StreamManager() *streamManager {
	return a.streamManager
}

type streamManager struct {
	aliveSrv        map[string]map[string]*Stream
	hostIndex       map[string]streamHostIndex
	streamStoreLock sync.RWMutex
}

type streamHostIndex struct {
	one string
	two string
}

type Stream struct {
	stream interface {
		Send(*cronpb.Event) error
	}
	cancelFunc  func()
	CreateTime  time.Time
	Host        string
	Port        int32
	ServiceName string
	Region      string
	System      int64
}

func (s *Stream) Cancel() {
	if s.cancelFunc != nil {
		s.cancelFunc()
	}
}

func (s *Stream) Send(e *cronpb.Event) error {
	return s.stream.Send(e)
}

func (c *streamManager) generateKey(region string, projectID int64, srvName string) string {
	return fmt.Sprintf("%s_%d/%s", region, projectID, srvName)
}

func (c *streamManager) generateServiceKey(info infra.NodeMeta) string {
	return fmt.Sprintf("%s_%d_%s_%s_%d", info.Region, info.System, info.ServiceName, info.Host, info.Port)
}

func (c *streamManager) SaveStream(info infra.NodeMeta, stream interface {
	Send(*cronpb.Event) error
}, cancelFunc func()) {
	c.streamStoreLock.Lock()
	defer c.streamStoreLock.Unlock()
	k := c.generateKey(info.Region, info.System, info.ServiceName)
	srvList, exist := c.aliveSrv[k]
	if !exist {
		srvList = make(map[string]*Stream)
		c.aliveSrv[k] = srvList
	}

	kk := c.generateServiceKey(info)
	srvList[kk] = &Stream{
		cancelFunc:  cancelFunc,
		stream:      stream,
		CreateTime:  time.Now(),
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

func (c *streamManager) RemoveStream(info infra.NodeMeta) {
	c.streamStoreLock.Lock()
	defer c.streamStoreLock.Unlock()
	k := c.generateKey(info.Region, info.System, info.ServiceName)
	srvList, exist := c.aliveSrv[k]
	if !exist {
		return
	}
	delete(srvList, c.generateServiceKey(info))
	if len(srvList) == 0 {
		delete(c.aliveSrv, k)
	}
}

func (c *streamManager) GetStreamsByHost(host string) *Stream {
	if index, exist := c.hostIndex[host]; exist {
		if levelOne, exist := c.aliveSrv[index.one]; exist {
			return levelOne[index.two]
		}
	}
	return nil
}

func (c *streamManager) GetStreams(region string, system int64, srvName string) map[string]*Stream {
	c.streamStoreLock.RLock()
	defer c.streamStoreLock.RUnlock()
	srvList, exist := c.aliveSrv[c.generateKey(region, system, srvName)]
	if !exist {
		return nil
	}
	return srvList
}

type AgentProjectCompareInfo struct {
	Hash       string
	LatestTime int64
	BeforAddrs []string
}

func (a *app) CalcAgentDataConsistency(done <-chan struct{}) error {
	c := time.NewTicker(time.Minute)

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


