package register

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/spacegrower/watermelon/infra/register"
	"github.com/spacegrower/watermelon/infra/wlog"
	"github.com/spacegrower/watermelon/pkg/safe"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/status"

	"github.com/holdno/gopherCron/pkg/cronpb"
	"github.com/holdno/gopherCron/pkg/infra"
	u "github.com/holdno/gopherCron/utils"
)

type remoteRegistryV2 struct {
	once          sync.Once
	ctx           context.Context
	cancelFunc    context.CancelFunc
	client        *CenterClient
	currentStream cronpb.Center_RegisterAgentV2Client
	metas         []infra.NodeMetaRemote
	log           wlog.Logger
	reConnect     func() error
	localIP       string
	locker        *sync.Mutex
	changedChan   chan struct{}

	eventHandler func(context.Context, *cronpb.ServiceEvent) (*cronpb.ClientEvent, error)

	str string
}

func NewRemoteRegisterV2(localIP string, connect func() (*CenterClient, error), eventHandler func(context.Context, *cronpb.ServiceEvent) (*cronpb.ClientEvent, error)) (register.ServiceRegister[infra.NodeMetaRemote], error) {
	ctx, cancel := context.WithCancel(context.Background())

	rr := &remoteRegistryV2{
		ctx:          ctx,
		cancelFunc:   cancel,
		log:          wlog.With(zap.String("component", "remote-register")),
		eventHandler: eventHandler,
		localIP:      localIP,
		str:          u.RandomStr(32),
		changedChan:  make(chan struct{}),
		locker:       &sync.Mutex{},
	}

	rr.reConnect = func() (err error) {
		if rr.client != nil && rr.client.Cc != nil {
			if rr.client.Cc.GetState() == connectivity.Ready {
				return
			}
			rr.client.Cc.Close()
		}

		rr.currentStream = nil
		rr.client, err = connect()
		return
	}

	if err := rr.reConnect(); err != nil {
		cancel()
		return nil, err
	}

	return rr, nil
}

func (s *remoteRegistryV2) SetMetas(metas []infra.NodeMetaRemote) {
	s.metas = metas
}

func (s *remoteRegistryV2) Register() error {
	s.locker.Lock()
	defer s.locker.Unlock()
	s.log.Debug("start registerV2")

	if s.changedChan != nil {
		close(s.changedChan)
	}

	s.changedChan = make(chan struct{})

	if err := s.register(); err != nil {
		if errors.Is(err, io.EOF) {
			time.Sleep(time.Second)
		}
		return err
	}

	return nil
}

func (s *remoteRegistryV2) parserServices() (services []*cronpb.AgentInfo) {
	for _, item := range s.metas {
		meta := &cronpb.AgentInfo{
			Region:      item.Region,
			OrgID:       item.OrgID,
			Systems:     item.Systems,
			ServiceName: item.ServiceName,
			Host:        item.Host,
			Port:        int32(item.Port),
			Weight:      item.Weight,
			Runtime:     item.Runtime,
			Tags:        item.Tags,
			Version:     item.Version,
		}

		for _, v := range item.GrpcMethods {
			meta.Methods = append(meta.Methods, &cronpb.MethodInfo{
				Name:           v.Name,
				IsClientStream: v.IsClientStream,
				IsServerStream: v.IsServerStream,
			})
		}

		services = append(services, meta)
	}
	return
}

func (s *remoteRegistryV2) getConnect(ctx context.Context) (cronpb.Center_RegisterAgentV2Client, error) {
	if s.currentStream != nil {
		return s.currentStream, nil
	}
	var err error
	s.currentStream, err = s.client.RegisterAgentV2(ctx)
	return s.currentStream, err
}

func (s *remoteRegistryV2) register() error {
	var (
		receive     func() (*cronpb.ServiceEvent, error)
		send        func(*cronpb.ClientEvent) error
		closeStream func() error
		services    = s.parserServices()

		ctx, cancel = context.WithCancel(s.ctx)
	)

	if len(services) == 0 {
		cancel()
		return errors.New("empty service")
	} else {
		stream, err := s.getConnect(ctx)
		if err != nil {
			cancel()
			return err
		}

		if err = stream.Send(&cronpb.ClientEvent{
			Id:        u.GetStrID(),
			Type:      cronpb.EventType_EVENT_REGISTER_REQUEST,
			EventTime: time.Now().Unix(),
			Event: &cronpb.ClientEvent_RegisterInfo{
				RegisterInfo: &cronpb.RegisterInfo{
					Agents: services,
				},
			},
		}); err != nil {
			cancel()
			return err
		}

		receive = stream.Recv
		send = stream.Send
		closeStream = stream.CloseSend
	}

	errHandler := func(err error) {
		s.log.Error("agent handle event with error", zap.Error(err))
		time.Sleep(time.Second)
		if gerr, ok := status.FromError(err); ok && gerr.Code() == codes.Canceled {
			s.log.Warn("retry to reconnect", zap.String("status", gerr.Code().String()))
			if err = s.reConnect(); err != nil {
				s.log.Error("failed to reconnect registry", zap.Error(err))
			}
		}
		s.reRegister()
	}

	waitingFirstReceive := make(chan struct{})

	go safe.Run(func() {
		var (
			err   error
			resp  *cronpb.ServiceEvent
			reply *cronpb.ClientEvent
			once  = &sync.Once{}
		)
		defer func() {
			closeStream()
			if err != nil {
				errHandler(err)
			}
			cancel()
		}()
		for {
			select {
			case <-s.changedChan:
				return
			case <-s.ctx.Done():
				s.log.Warn("register receiver is down, context done")
				return
			default:
				resp, err = receive()
				if err != nil {
					return
				}

				if resp.Type == cronpb.EventType_EVENT_REGISTER_REPLY {
					once.Do(func() {
						waitingFirstReceive <- struct{}{}
					})
				}

				s.log.Debug("receive event", zap.String("event", resp.Type.String()), zap.Any("value", resp.GetEvent()))

				if reply, err = s.eventHandler(ctx, resp); err != nil {
					return
				}

				if reply != nil {
					if err = send(reply); err != nil {
						s.log.Error("failed reply center request", zap.Error(err), zap.String("event", resp.Type.String()),
							zap.String("value", resp.String()))
						return
					}
				}
			}
		}
	})
	<-waitingFirstReceive
	close(waitingFirstReceive)
	return nil
}

func (s *remoteRegistryV2) DeRegister() error {
	s.cancelFunc()
	return nil
}

func (s *remoteRegistryV2) Close() {
	// just close kvstore not etcd client
	s.DeRegister()
}

func (s *remoteRegistryV2) reRegister() {
	for {
		select {
		case <-s.ctx.Done():
			wlog.Warn("register is down, context done")
		default:
			if err := s.Register(); err != nil {
				if gerr, ok := status.FromError(err); ok {
					switch gerr.Code() {
					case codes.Canceled:
						if innererr := s.reConnect(); innererr == nil {
							s.log.Error("failed to reconnect registry", zap.Error(innererr))
						}
					case codes.Aborted:
						fallthrough
					case codes.Unauthenticated:
						fallthrough
					case codes.PermissionDenied:
						s.cancelFunc()
						s.log.Error("registration refused", zap.Error(err))
						return
					default:
					}
				}
				s.log.Error("failed to register service", zap.Error(err))
				time.Sleep(time.Second)
				continue
			}
		}

		return
	}
}
