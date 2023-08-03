package register

import (
	"context"
	"errors"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/holdno/gopherCron/pkg/cronpb"
	"github.com/holdno/gopherCron/pkg/infra"
	u "github.com/holdno/gopherCron/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/spacegrower/watermelon/infra/definition"
	"github.com/spacegrower/watermelon/infra/register"
	"github.com/spacegrower/watermelon/infra/utils"
	"github.com/spacegrower/watermelon/infra/wlog"
	"github.com/spacegrower/watermelon/pkg/safe"
	"go.uber.org/zap"
)

type remoteRegistryV2 struct {
	once       sync.Once
	ctx        context.Context
	cancelFunc context.CancelFunc
	client     CenterClient
	metas      []infra.NodeMetaRemote
	log        wlog.Logger
	reConnect  func() error
	localIP    string

	eventHandler func(context.Context, *cronpb.ServiceEvent) (*cronpb.ClientEvent, error)

	str string
}

func NewRemoteRegisterV2(localIP string, connect func() (CenterClient, error), eventHandler func(context.Context, *cronpb.ServiceEvent) (*cronpb.ClientEvent, error)) (register.ServiceRegister[infra.NodeMetaRemote], error) {
	ctx, cancel := context.WithCancel(context.Background())

	rr := &remoteRegistryV2{
		ctx:          ctx,
		cancelFunc:   cancel,
		log:          wlog.With(zap.String("component", "remote-register")),
		eventHandler: eventHandler,
		localIP:      localIP,
		str:          u.RandomStr(32),
	}

	rr.reConnect = func() (err error) {
		if rr.client.Cc != nil {
			rr.client.Cc.Close()
		}
		rr.client, err = connect()
		return
	}

	if err := rr.reConnect(); err != nil {
		cancel()
		return nil, err
	}

	return rr, nil
}

func (s *remoteRegistryV2) Append(meta infra.NodeMetaRemote) error {
	// customize your register logic
	meta.Weight = utils.GetEnvWithDefault(definition.NodeWeightENVKey, 100, func(val string) (int32, error) {
		res, err := strconv.Atoi(val)
		if err != nil {
			return 0, err
		}
		return int32(res), nil
	})

	s.metas = append(s.metas, meta)
	return nil
}

func (s *remoteRegistryV2) Register() error {
	s.log.Debug("start register")

	if err := s.register(); err != nil {
		s.log.Error("failed to register service", zap.Error(err))
		if err == io.EOF {
			time.Sleep(time.Second)
			if innererr := s.reConnect(); innererr == nil {
				s.log.Error("failed to reconnect registry", zap.Error(innererr))
			}
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

func (s *remoteRegistryV2) register() error {
	var (
		receive  func() (*cronpb.ServiceEvent, error)
		send     func(*cronpb.ClientEvent) error
		close    func() error
		services = s.parserServices()

		ctx, cancel = context.WithCancel(s.ctx)
	)

	if len(services) == 0 {
		cancel()
		return errors.New("empty service")
	} else {
		cli, err := s.client.RegisterAgentV2(ctx)
		if err != nil {
			cancel()
			return err
		}

		if err = cli.Send(&cronpb.ClientEvent{
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

		receive = cli.Recv
		send = cli.Send
		close = cli.CloseSend
	}

	errHandler := func(err error) error {
		s.log.Warn("recv with error", zap.Error(err))
		time.Sleep(time.Second)
		grpcErr, ok := status.FromError(err)
		if err == io.EOF || !ok || grpcErr.Code() == codes.Unavailable {
			s.log.Warn("retry to reconnect", zap.String("status", grpcErr.Code().String()))
			if err = s.reConnect(); err != nil {
				s.log.Error("failed to reconnect registry", zap.Error(err))
				return err
			}
		}
		s.reRegister()
		return nil
	}

	go safe.Run(func() {
		defer func() {
			close()
			cancel()
		}()
		for {
			select {
			case <-s.ctx.Done():
				s.log.Warn("register receiver is down, context done")
				return
			default:
				resp, err := receive()
				if err != nil {
					if err = errHandler(err); err != nil {
						continue
					}
					return
				}

				s.log.Debug("receive event", zap.String("event", resp.Type.String()), zap.Any("value", resp.GetEvent()))

				reply, err := s.eventHandler(ctx, resp)
				if err != nil {
					if err = errHandler(err); err != nil {
						continue
					}
				}
				send(reply)

				switch resp.Type {
				case cronpb.EventType_EVENT_REGISTER_HEARTBEAT_PING:
					send(&cronpb.ClientEvent{
						Id:        resp.Id,
						Type:      cronpb.EventType_EVENT_REGISTER_HEARTBEAT_PONG,
						EventTime: time.Now().Unix(),
					})
				// case "confirm":
				// 	for _, service := range services {
				// 		s.log.Info("service registered successful",
				// 			zap.Any("systems", service.Systems),
				// 			zap.String("name", service.ServiceName),
				// 			zap.String("address", fmt.Sprintf("%s:%d", service.Host, service.Port)))
				// 	}
				default:

				}
			}
		}
	})

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
				time.Sleep(time.Second)
				continue
			}
		}

		return
	}
}
