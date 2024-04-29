package register

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/spacegrower/watermelon/infra/register"
	"github.com/spacegrower/watermelon/infra/wlog"
	"github.com/spacegrower/watermelon/pkg/safe"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/holdno/gopherCron/pkg/cronpb"
	"github.com/holdno/gopherCron/pkg/infra"
	u "github.com/holdno/gopherCron/utils"
)

type remoteRegistry struct {
	once       sync.Once
	ctx        context.Context
	cancelFunc context.CancelFunc
	client     CenterClient
	metas      []infra.NodeMetaRemote
	log        wlog.Logger
	reConnect  func() error
	localIP    string

	eventHandler func(*cronpb.Event)

	str string
}

type CenterClient struct {
	cronpb.CenterClient
	Cc *grpc.ClientConn
}

func NewRemoteRegister(localIP string, connect func() (CenterClient, error), eventHandler func(*cronpb.Event)) (register.ServiceRegister[infra.NodeMetaRemote], error) {
	ctx, cancel := context.WithCancel(context.Background())

	rr := &remoteRegistry{
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

func (s *remoteRegistry) SetMetas(metas []infra.NodeMetaRemote) {
	s.metas = metas
}

func (s *remoteRegistry) Register() error {
	s.log.Debug("start register")

	if err := s.register(); err != nil {
		s.log.Error("failed to register service", zap.Error(err))
		grpcErr, ok := status.FromError(err)
		if err == io.EOF || !ok || grpcErr.Code() == codes.Unavailable {
			time.Sleep(time.Second)
			if innererr := s.reConnect(); innererr == nil {
				s.log.Error("failed to reconnect registry", zap.Error(innererr))
			}
		}
		return err
	}

	return nil
}

func (s *remoteRegistry) parserServices() (services []*cronpb.AgentInfo) {
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

func (s *remoteRegistry) register() error {
	var (
		receive  func() (*cronpb.Event, error)
		close    func() error
		services = s.parserServices()

		ctx, cancel = context.WithCancel(s.ctx)
	)

	if len(services) == 0 {
		cancel()
		return errors.New("empty service")
	} else {
		cli, err := s.client.RegisterAgent(ctx)
		if err != nil {
			cancel()
			return err
		}

		if err = cli.Send(&cronpb.RegisterAgentReq{Agents: services}); err != nil {
			cancel()
			return err
		}

		receive = cli.Recv
		close = cli.CloseSend
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
					s.log.Warn("recv with error", zap.Error(err))
					time.Sleep(time.Second)
					grpcErr, ok := status.FromError(err)
					if err == io.EOF || !ok || grpcErr.Code() == codes.Unavailable {
						s.log.Warn("retry to reconnect", zap.String("status", grpcErr.Code().String()))
						if err = s.reConnect(); err != nil {
							s.log.Error("failed to reconnect registry", zap.Error(err))
							continue
						}
					}
					s.reRegister()
					return
				}

				s.log.Debug("receive event", zap.String("event", resp.Type), zap.String("value", string(resp.Value)))

				switch resp.Type {
				case "heartbeat":
				case "confirm":
					for _, service := range services {
						s.log.Info("service registered successful",
							zap.Any("systems", service.Systems),
							zap.String("name", service.ServiceName),
							zap.String("address", fmt.Sprintf("%s:%d", service.Host, service.Port)))
					}
				default:
					s.eventHandler(resp)
				}
			}
		}
	})

	return nil
}

func (s *remoteRegistry) DeRegister() error {
	s.cancelFunc()
	return nil
}

func (s *remoteRegistry) Close() {
	// just close kvstore not etcd client
	s.DeRegister()
}

func (s *remoteRegistry) reRegister() {
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
