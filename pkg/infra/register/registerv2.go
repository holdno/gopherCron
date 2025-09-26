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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/status"

	"github.com/holdno/gopherCron/pkg/cronpb"
	"github.com/holdno/gopherCron/pkg/infra"
	u "github.com/holdno/gopherCron/utils"
)

type remoteRegistryV2 struct {
	once            sync.Once
	ctx             context.Context
	cancelFunc      context.CancelFunc
	client          *CenterClient
	currentStream   cronpb.Center_RegisterAgentV2Client
	metas           []infra.NodeMetaRemote
	log             wlog.Logger
	reConnect       func() error
	localIP         string
	locker          *sync.Mutex
	receivingLocker *sync.Mutex

	eventHandler   func(context.Context, *cronpb.ServiceEvent) (*cronpb.ClientEvent, error)
	registerNotify *registerNotify

	str string
}

type registerNotify struct {
	locker           *sync.Mutex
	registeredNotify map[string]chan struct{}
}

func (s *registerNotify) notify(reqID string) {
	s.locker.Lock()
	if c, exist := s.registeredNotify[reqID]; exist {
		close(c)
		delete(s.registeredNotify, reqID)
	} else {
		// 兼容旧版本
		for key, v := range s.registeredNotify {
			close(v)
			delete(s.registeredNotify, key)
		}
	}
	s.locker.Unlock()
}

func (s *registerNotify) registerNotify(reqID string) func() {
	s.locker.Lock()
	c := make(chan struct{})
	s.registeredNotify[reqID] = c
	s.locker.Unlock()

	return func() {
		<-c
	}
}

func NewRemoteRegisterV2(localIP string, connect func() (*CenterClient, error), eventHandler func(context.Context, *cronpb.ServiceEvent) (*cronpb.ClientEvent, error)) (register.ServiceRegister[infra.NodeMetaRemote], error) {
	ctx, cancel := context.WithCancel(context.Background())

	rr := &remoteRegistryV2{
		ctx:             ctx,
		cancelFunc:      cancel,
		log:             wlog.With(zap.String("component", "remote-register")),
		eventHandler:    eventHandler,
		localIP:         localIP,
		str:             u.RandomStr(32),
		locker:          &sync.Mutex{},
		receivingLocker: &sync.Mutex{},
		registerNotify: &registerNotify{
			locker:           &sync.Mutex{},
			registeredNotify: make(map[string]chan struct{}),
		},
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
	// 如果当前有活跃的stream，直接在现有连接上重新注册，避免死锁
	if s.currentStream != nil {
		return s.ReRegisterWithCurrentStream()
	}

	s.locker.Lock()
	defer s.locker.Unlock()
	s.log.Debug("start registerV2")

	if err := s.register(); err != nil {
		if errors.Is(err, io.EOF) {
			time.Sleep(time.Second)
		}
		return err
	}

	return nil
}

// ReRegisterWithCurrentStream 在现有连接上重新发送注册信息，避免死锁
func (s *remoteRegistryV2) ReRegisterWithCurrentStream() error {
	services := s.parserServices()
	if len(services) == 0 {
		return errors.New("empty service")
	}

	// 检查当前连接是否可用
	if s.currentStream == nil {
		return errors.New("no active stream")
	}

	reqID := u.GetStrID()

	// 直接在现有连接上发送注册请求
	if err := s.currentStream.Send(&cronpb.ClientEvent{
		Id:        reqID,
		Type:      cronpb.EventType_EVENT_REGISTER_REQUEST,
		EventTime: time.Now().Unix(),
		Event: &cronpb.ClientEvent_RegisterInfo{
			RegisterInfo: &cronpb.RegisterInfo{
				Agents: services,
			},
		},
	}); err != nil {
		s.log.Error("failed to re-register on current stream", zap.Error(err))
		return err
	}

	s.log.Debug("re-registered successfully on current stream", zap.String("reqID", reqID))
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
	if s.currentStream != nil && s.currentStream.Context().Err() != nil {
		return s.currentStream, nil
	}
	var err error
	s.currentStream, err = s.client.RegisterAgentV2(ctx)
	return s.currentStream, err
}

type RegisterType int

const (
	FirstRegister RegisterType = iota
	SecondRegister
)

func (s *remoteRegistryV2) register() error {
	var (
		receive     func() (*cronpb.ServiceEvent, error)
		send        func(*cronpb.ClientEvent) error
		closeStream func() error
		services    = s.parserServices()
	)

	if len(services) == 0 {
		return errors.New("empty service")
	}

	stream, err := s.getConnect(s.ctx)
	if err != nil {
		return err
	}

	reqID := u.GetStrID()

	// 无论是第几次注册，都需要发送注册信息
	if err = stream.Send(&cronpb.ClientEvent{
		Id:        reqID,
		Type:      cronpb.EventType_EVENT_REGISTER_REQUEST,
		EventTime: time.Now().Unix(),
		Event: &cronpb.ClientEvent_RegisterInfo{
			RegisterInfo: &cronpb.RegisterInfo{
				Agents: services,
			},
		},
	}); err != nil {
		s.currentStream = nil
		return err
	}

	if s.receivingLocker.TryLock() {
		// 加入 hang 实现“同步模式”的重新注册
		hang := make(chan struct{}, 1)
		closeHang := sync.OnceFunc(func() {
			close(hang)
		})

		go safe.Run(func() {
			defer closeHang()
			wait := s.registerNotify.registerNotify(reqID)
			hang <- struct{}{}
			wait()
		})

		<-hang

		// 首次注册需要开启事件监听
		receive = stream.Recv
		send = stream.Send
		closeStream = stream.CloseSend
		kill := make(chan struct{})

		errHandler := func(err error) {
			if err == nil {
				return
			}
			closeHang() // 避免异常导致死锁
			close(kill)

			if closeStream != nil {
				closeStream()
				s.currentStream = nil
			}
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

		go safe.Run(func() {
			var (
				err               error
				resp              *cronpb.ServiceEvent
				eventHandleLocker = &sync.Mutex{}
			)
			defer func() {
				s.receivingLocker.Unlock()
				if err != nil {
					errHandler(err)
				}
			}()

			for {
				select {
				case <-kill:
					s.log.Warn("register killed by self")
					return
				case <-s.ctx.Done():
					s.log.Warn("register receiver is down, context done")
					return
				default:
					if resp, err = receive(); err != nil {
						err = fmt.Errorf("receive error: %w", err)
						return
					}
					if resp.Type == cronpb.EventType_EVENT_REGISTER_REPLY {
						// 重新注册的过程中修改任务信息，同样会下发 EventType_EVENT_REGISTER_REPLY 事件，可能导致 notify 不准确
						s.registerNotify.notify(resp.Id)
					}

					s.log.Debug("receive event", zap.String("event", resp.Type.String()), zap.Any("value", resp.GetEvent()))
					go safe.Run(func() {
						var (
							err   error
							reply *cronpb.ClientEvent
						)
						defer func() {
							if err != nil {
								errHandler(err)
							}
						}()
						eventHandleLocker.Lock()
						defer eventHandleLocker.Unlock()
						ctx, cancel := context.WithTimeout(s.ctx, time.Minute)
						defer cancel()
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
					})
				}
			}
		})

		<-hang
	}

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
