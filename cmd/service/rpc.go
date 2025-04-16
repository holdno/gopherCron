package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/middleware"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/jwt"
	"github.com/holdno/gopherCron/pkg/cronpb"
	"github.com/holdno/gopherCron/pkg/etcd"
	"github.com/holdno/gopherCron/pkg/infra"
	"github.com/holdno/gopherCron/utils"

	"github.com/spacegrower/watermelon/infra/register"
	wutils "github.com/spacegrower/watermelon/infra/utils"
	"github.com/spacegrower/watermelon/infra/wlog"
	"github.com/spacegrower/watermelon/pkg/safe"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type cronRpc struct {
	app app.App
	cronpb.UnimplementedCenterServer
	registerMetricsAdd      func(add float64, labels ...string)
	eventsMetricsInc        func()
	getCurrentRegisterAddrs func() []*net.TCPAddr
}

func (s *cronRpc) RemoveStream(ctx context.Context, req *cronpb.RemoveStreamRequest) (*cronpb.Result, error) {
	stream := s.app.StreamManager().GetStreamsByHost(req.Client)
	currentRegistry := false
	if stream != nil {
		currentRegistry = true
		stream.Cancel()
	} else {
		streamV2 := s.app.StreamManagerV2().GetStreamsByHost(req.Client)
		if streamV2 != nil {
			currentRegistry = true
			streamV2.Cancel()
		}
	}
	return &cronpb.Result{
		Result:  currentRegistry,
		Message: "ok",
	}, nil
}

func (s *cronRpc) TryLock(req cronpb.Center_TryLockServer) error {
	authenticator := jwt.GetProjectAuthenticator(req.Context())
	agentIP, exist := middleware.GetAgentIP(req.Context())
	if !exist {
		return status.Error(codes.PermissionDenied, codes.PermissionDenied.String())
	}
	var (
		locker      *etcd.Locker
		heartbeat   = time.NewTicker(time.Second * 5)
		receiveChan = make(chan *cronpb.TryLockRequest)
	)

	agentVersion, existAgentVersion := middleware.GetAgentVersion(req.Context())
	now := time.Now()

	defer func() {
		heartbeat.Stop()
		if locker == nil {
			return
		}
		if existAgentVersion && strings.Contains(agentVersion, "2.4.7") && time.Since(now).Seconds() < 5 {
			time.Sleep(time.Duration(5-time.Since(now).Seconds()) * time.Second)
		}
		locker.Unlock()
	}()

	go func() {
		defer close(receiveChan)
		for {
			task, err := req.Recv()
			if err != nil || task == nil {
				return
			}
			receiveChan <- task
		}
	}()

	for {
		select {
		case <-req.Context().Done():
			return req.Context().Err()
		case <-heartbeat.C:
			if err := req.Send(&cronpb.TryLockReply{
				Result:  true,
				Message: "heartbeat",
			}); err != nil {
				return err
			}
		case task := <-receiveChan:
			if task == nil || task.Type == cronpb.LockType_UNLOCK {
				return nil
			}

			if authenticator != nil && !authenticator.Allow(task.ProjectId) {
				return status.Error(codes.Unauthenticated, codes.Unauthenticated.String())
			}
			locker = s.app.GetTaskLocker(&common.TaskInfo{TaskID: task.TaskId, ProjectID: task.ProjectId})
			// 锁的持有者除了agentip外还应该增加tmpid来确保是同一个任务在尝试恢复锁
			if err := locker.TryLockWithOwner(fmt.Sprintf("%s:%s", agentIP, task.TaskTmpId)); err != nil {
				return status.Error(codes.Aborted, err.Error())
			}

			// // 加锁成功后获取任务运行中状态的key是否存在，若存在则说明之前执行该任务的机器网络中断 / 宕机
			// runningKey, runningInfo, err := s.app.CheckTaskIsRunning(task.ProjectId, task.TaskId)
			// if err != nil {
			// 	return err
			// }

			// // 获取任务日志，如果日志存在，则说明是续锁操作
			// // 续锁的话就得判断任务是否已经被杀掉
			// if runningKey.TmpID == "" {
			// 第一次加锁 或 任务已结束
			log, err := s.app.GetTaskLogDetail(task.ProjectId, task.TaskId, task.TaskTmpId)
			if err != nil {
				return err
			}
			if log != nil {
				if log.EndTime != 0 {
					// 任务已经结束
					return status.Error(codes.Aborted, "任务已结束")
				}
				if log.ClientIP != task.AgentIp {
					return status.Error(codes.Aborted, "任务运行中")
				}
			}
			// }

			if err = req.Send(&cronpb.TryLockReply{
				Result:  true,
				Message: "ok",
			}); err != nil {
				return err
			}
		}
	}
}

func (s *cronRpc) StatusReporter(ctx context.Context, req *cronpb.ScheduleReply) (*cronpb.Result, error) {
	author := jwt.GetProjectAuthenticator(ctx)
	if author != nil && !author.Allow(req.ProjectId) {
		return nil, status.Error(codes.Unauthenticated, codes.Unauthenticated.String())
	}
	agentIP, _ := middleware.GetAgentIP(ctx)
	agentVersion, existAgentVersion := middleware.GetAgentVersion(ctx)
	switch req.Event.Type {
	case common.TASK_STATUS_RUNNING_V2:
		var result common.TaskExecutingInfo
		if err := json.Unmarshal(req.Event.Value, &result); err != nil {
			return nil, err
		}

		if result.PlanType != common.ActivePlan {
			// 如果任务不是被人工调度，需要检测任务当前的可被调度状态
			taskInfo, err := s.app.GetTask(req.ProjectId, result.Task.TaskID)
			if err != nil {
				return nil, err
			}

			if taskInfo.Status != common.TASK_STATUS_SCHEDULING {
				return &cronpb.Result{
					Result:  false,
					Message: fmt.Sprintf("该任务已停止调度"),
				}, status.Error(codes.Aborted, "The task's schedule is being stopped now, and the operation has been rejected.")
			}
		}

		if err := s.app.SetTaskRunning(agentIP, agentVersion, &result); err != nil {
			var workflowID int64
			if result.Task.FlowInfo != nil {
				workflowID = result.Task.FlowInfo.WorkflowID
			}
			wlog.Error("failed to set task running status", zap.Error(err), zap.String("task_id", result.Task.TaskID),
				zap.Int64("project_id", result.Task.ProjectID), zap.String("tmp_id", result.TmpID),
				zap.Any("plan_time", result.PlanTime),
				zap.Int64("workflow_id", workflowID))

			if cerr, ok := err.(*errors.Error); ok && cerr.Code != http.StatusInternalServerError {
				// aborted 状态可以让agent取消请求重试，直接终止任务
				return nil, status.Error(codes.Aborted, cerr.Msg)
			}
			return nil, err
		}
	case common.TASK_STATUS_FINISHED_V2:
		var result common.TaskFinishedV2
		if err := json.Unmarshal(req.Event.Value, &result); err != nil {
			return nil, err
		}

		if existAgentVersion && utils.CompareVersion("v2.1.9999", agentVersion) {
			s.app.SaveTaskLog(agentIP, result)
		}

		if err := s.app.HandlerTaskFinished(agentIP, &result); err != nil && err != app.ErrWorkflowInProcess {
			wlog.Error("failed to set task finished status", zap.Error(err), zap.String("task_id", result.TaskID),
				zap.Int64("project_id", result.ProjectID), zap.String("tmp_id", result.TmpID),
				zap.Int64("workflow_id", result.WorkflowID))
			return nil, err
		}
	}
	return &cronpb.Result{
		Result:  true,
		Message: "ok",
	}, nil
}

func (s *cronRpc) SendEvent(ctx context.Context, req *cronpb.SendEventRequest) (*cronpb.ClientEvent, error) {
	if req.ProjectId == 0 {
		// got event for center
		if err := s.app.HandleCenterEvent(req.Event); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	} else {
		if req.Agent != "" {
			stream := s.app.StreamManagerV2().GetStreamsByHost(req.Agent)
			if stream == nil {
				return nil, status.Error(codes.NotFound, fmt.Sprintf("stream not found, service ip: %s, agent: %s", s.app.GetIP(), req.Agent))
			}
			resp, err := s.app.StreamManagerV2().SendEventWaitResponse(ctx, stream, req.Event)
			if err != nil {
				gerr, ok := status.FromError(err)
				if err == app.SendEventRequestTimeOutError || (ok && (gerr.Code() == codes.Unavailable || gerr.Code() == codes.Canceled)) {
					stream.Cancel()
				}
				return nil, status.Error(gerr.Code(), fmt.Sprintf("stream 下发任务操作失败, %s", gerr.Message()))
			}
			return resp, err
		}
		for _, v := range s.app.StreamManager().GetStreams(req.ProjectId, cronpb.Agent_ServiceDesc.ServiceName) {
			if err := v.Send(req.Event.GetRegisterReply()); err != nil {
				v.Cancel()
				return nil, errors.NewError(http.StatusInternalServerError, fmt.Sprintf("下发任务操作失败, 主动断开agent链接, %s:%d, %v", v.Host, v.Port, err))
			}
		}
		for _, v := range s.app.StreamManagerV2().GetStreams(req.ProjectId, cronpb.Agent_ServiceDesc.ServiceName) {
			_, err := s.app.StreamManagerV2().SendEventWaitResponse(ctx, v, req.Event)
			if err != nil {
				v.Cancel()
				return nil, errors.NewError(http.StatusInternalServerError, fmt.Sprintf("stream 下发任务操作失败, 主动断开agent链接, %s:%d, %v", v.Host, v.Port, err))
			}
		}
	}
	return &cronpb.ClientEvent{
		Id: req.Event.Id,
	}, nil
}

func (s *cronRpc) Auth(ctx context.Context, req *cronpb.AuthReq) (*cronpb.AuthReply, error) {
	var pids []int64
	for pid, token := range req.Kvs {
		pids = append(pids, pid)
		project, err := s.app.GetProject(pid)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

		if project.Token != token {
			return nil, status.Error(codes.PermissionDenied, codes.PermissionDenied.String())
		}
	}

	claims := jwt.AgentTokenClaims{
		Biz:        jwt.DefaultBIZ,
		ProjectIDs: pids,
		Exp:        int64(time.Now().Add(time.Duration(s.app.GetConfig().JWT.Exp) * time.Hour).Unix()),
		Iat:        time.Now().Unix(),
	}
	token, err := jwt.BuildAgentJWT(claims, []byte(s.app.GetConfig().JWT.PrivateKey))
	if err != nil {
		return nil, err
	}
	return &cronpb.AuthReply{
		Jwt:        token,
		ExpireTime: claims.Exp,
	}, nil
}

func (s *cronRpc) RegisterAgent(req cronpb.Center_RegisterAgentServer) error {
	author := jwt.GetProjectAuthenticator(req.Context())
	newRegister := make(chan *cronpb.RegisterAgentReq)
	go safe.Run(func() {
		for {
			select {
			case <-req.Context().Done():
				return
			default:
				info, err := req.Recv()
				if err != nil {
					close(newRegister)
					return
				}
				newRegister <- info
			}
		}
	})

	agentIP, _ := middleware.GetAgentIP(req.Context())

	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	r := infra.MustSetupEtcdRegister()
	var registerStream []infra.NodeMeta
	defer func() {
		s.registerMetricsAdd(-1, agentIP)
		for _, meta := range registerStream {
			s.app.StreamManager().RemoveStream(meta)
		}
		r.Close()
	}()
Here:
	for {
		select {
		case multiService := <-newRegister:
			if multiService == nil {
				break Here
			}

			var registerStreamOnce []infra.NodeMeta

			for _, info := range multiService.Agents {
				var methods []register.GrpcMethodInfo
				for _, v := range info.Methods {
					methods = append(methods, register.GrpcMethodInfo{
						Name:           v.Name,
						IsClientStream: v.IsClientStream,
						IsServerStream: v.IsServerStream,
					})
				}

				for _, v := range info.Systems {
					if author != nil && !author.Allow(v) {
						return status.Error(codes.Unauthenticated, fmt.Sprintf("registry: project id %d is unauthenticated, register failure", v))
					}
					meta := infra.NodeMeta{
						NodeMeta: register.NodeMeta{
							ServiceName: info.ServiceName,
							GrpcMethods: methods,
							Host:        info.Host,
							Port:        int(info.Port),
							Runtime:     info.Runtime,
							Version:     info.Version,
						},
						OrgID:        info.OrgID,
						Region:       info.Region,
						NodeWeight:   info.Weight,
						System:       v,
						Tags:         info.Tags,
						RegisterTime: time.Now().UnixNano(),
					}

					registerStreamOnce = append(registerStreamOnce, meta)
				}
			}

			r.SetMetas(registerStreamOnce)

			s.registerMetricsAdd(1, agentIP)
			if err := r.Register(); err != nil {
				wlog.Error("failed to register service", zap.Error(err), zap.String("method", "Register"))
				s.app.Metrics().CustomInc("register_error", s.app.GetIP(), err.Error())
				return status.Error(codes.Internal, "failed to register service")
			}

			for _, meta := range registerStreamOnce {
				s.app.StreamManager().SaveStream(meta, req, cancel)
			}

			registerStream = append(registerStream, registerStreamOnce...)

			for _, info := range multiService.Agents {
				for _, v := range info.Systems {
					// Dispatch 依赖 gRPC stream, 所以需要先 SaveStream 再 DispatchAgentJob
					if err := s.app.DispatchAgentJob(v, func(taskRaw []byte) error {
						if err := req.Send(&cronpb.Event{
							Version:   common.VERSION_TYPE_V1,
							Type:      common.REMOTE_EVENT_PUT,
							Value:     taskRaw,
							EventTime: time.Now().Unix(),
						}); err != nil {
							wlog.Info("failed to dispatch agent job v1", zap.String("host", fmt.Sprintf("%s:%d", info.Host, info.Port)), zap.Error(err))
							return err
						}
						return nil
					}); err != nil {
						return err
					}
				}
			}

			go func() {
				ticker := time.NewTicker(time.Second * 10)
				defer ticker.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						s.eventsMetricsInc()
						if err := req.Send(&cronpb.Event{
							Type:      "heartbeat",
							Version:   common.VERSION_TYPE_V1,
							Value:     []byte("heartbeat"),
							EventTime: time.Now().Unix(),
						}); err != nil {
							cancel()
							return
						}
					}
				}
			}()
		case <-ctx.Done():
			break Here
		}
	}

	return nil
}

type registerInfo struct {
	reqID string
	info  *cronpb.RegisterInfo
}

// watchAgentResponse watch agent register request or event handle response
func watchAgentResponse(ctx context.Context, receive func() (*cronpb.ClientEvent, error), callback func(*cronpb.ClientEvent)) <-chan *registerInfo {
	newRegisterInfo := make(chan *registerInfo)
	go safe.Run(func() {
		defer close(newRegisterInfo)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				info, err := receive()
				if err != nil {
					return
				}

				if info.Type == cronpb.EventType_EVENT_REGISTER_REQUEST {
					newRegisterInfo <- &registerInfo{
						reqID: info.Id,
						info:  info.GetRegisterInfo(),
					}
				} else {
					callback(info)
				}
			}
		}
	})
	return newRegisterInfo
}

// buildAgentRegister 构建agent的注册器与反注册器
func (s *cronRpc) buildAgentRegister(ctx context.Context) (registerFunc func(req *cronpb.RegisterInfo, next func([]infra.NodeMeta) error) error,
	deRegisterFunc func(next func([]infra.NodeMeta))) {
	author := jwt.GetProjectAuthenticator(ctx)
	agentIP, _ := middleware.GetAgentIP(ctx)
	mustGetHostIP := func() string {
		ip, _ := wutils.GetHostIP()
		return ip
	}
	currentServiceAddr := strings.ReplaceAll(s.getCurrentRegisterAddrs()[0].String(), "[::]", mustGetHostIP())

	r := infra.MustSetupEtcdRegister() // 获取注册中心
	var totalRegisteredStreams []infra.NodeMeta

	deRegisterOnce := sync.Once{}
	deRegisterFunc = func(next func([]infra.NodeMeta)) {
		deRegisterOnce.Do(func() {
			r.Close()
			s.registerMetricsAdd(-1, agentIP)
			next(totalRegisteredStreams)
		})
	}

	innerDeRegisterFunc := func(beforeMetas []infra.NodeMeta) {
		for _, v := range beforeMetas {
			s.app.StreamManagerV2().RemoveStream(v)
		}
	}

	once := sync.Once{}
	registerFunc = func(multiService *cronpb.RegisterInfo, next func([]infra.NodeMeta) error) error {
		once.Do(func() {
			// 同一个链接，只有第一次注册才记录metrics，多次触发可能是更新注册信息
			s.registerMetricsAdd(1, agentIP)
		})

		var registerStreamOnce []infra.NodeMeta
		for _, info := range multiService.Agents {
			var methods []register.GrpcMethodInfo
			for _, v := range info.Methods {
				methods = append(methods, register.GrpcMethodInfo{
					Name:           v.Name,
					IsClientStream: v.IsClientStream,
					IsServerStream: v.IsServerStream,
				})
			}

			for _, v := range info.Systems { // 对应 projectid
				if author != nil && !author.Allow(v) {
					return status.Error(codes.Unauthenticated, fmt.Sprintf("registry: project id %d is unauthenticated, register failure", v))
				}
				meta := infra.NodeMeta{
					NodeMeta: register.NodeMeta{
						ServiceName: info.ServiceName,
						GrpcMethods: methods,
						Host:        info.Host,
						Port:        int(info.Port),
						Runtime:     info.Runtime,
						Version:     info.Version,
					},
					CenterServiceEndpoint: currentServiceAddr,
					CenterServiceRegion:   s.app.GetConfig().Micro.Region,
					OrgID:                 info.OrgID,
					Region:                info.Region,
					NodeWeight:            info.Weight,
					System:                v,
					Tags:                  info.Tags,
					RegisterTime:          time.Now().UnixNano(),
				}
				registerStreamOnce = append(registerStreamOnce, meta)
			}
		}

		r.SetMetas(registerStreamOnce)

		if err := r.Register(); err != nil {
			wlog.Error("failed to register service", zap.Error(err), zap.String("method", "Register"))
			s.app.Metrics().CustomInc("register_error", s.app.GetIP(), err.Error())
			return status.Error(codes.Internal, "failed to register service")
		}

		if len(totalRegisteredStreams) != 0 {
			innerDeRegisterFunc(totalRegisteredStreams)
		}
		totalRegisteredStreams = make([]infra.NodeMeta, len(registerStreamOnce))
		copy(totalRegisteredStreams, registerStreamOnce)
		return next(registerStreamOnce)
	}

	return registerFunc, deRegisterFunc
}

type dispatcher func(reqID string, meta infra.NodeMeta) app.JobDispatcher

func buildDispatchJobsV2Handler(sendEvent func(ctx context.Context, e *cronpb.ServiceEvent) error) dispatcher {
	return func(firstReqID string, meta infra.NodeMeta) app.JobDispatcher {
		// firstReqID 代表需要向agent回复的任务下发id，agent在发起注册后，会监听该id的事件来作为中心对agent注册的回应
		// 但该id仅需要被回复一次即可
		once := sync.Once{}
		return func(taskRaw []byte) error {
			reqID := utils.GetStrID()
			once.Do(func() {
				reqID = firstReqID
			})
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			defer cancel()
			if err := sendEvent(ctx, &cronpb.ServiceEvent{
				Id:        reqID,
				Type:      cronpb.EventType_EVENT_REGISTER_REPLY,
				EventTime: time.Now().Unix(),
				Event: &cronpb.ServiceEvent_RegisterReply{
					RegisterReply: &cronpb.Event{
						Version:   common.VERSION_TYPE_V1,
						Type:      common.REMOTE_EVENT_PUT,
						Value:     taskRaw,
						EventTime: time.Now().Unix(),
					},
				},
			}); err != nil {
				wlog.Info("failed to dispatch agent job v2", zap.String("host", fmt.Sprintf("%s:%d", meta.Host, meta.Port)), zap.Error(err))
				return err
			}
			return nil
		}
	}
}

func (s *cronRpc) getHeartbeatKeeper(ctx context.Context, req interface {
	Send(*cronpb.ServiceEvent) error
}, onPong func(), onError func()) func() {
	var do sync.Once
	return func() {
		do.Do(func() {
			go func() {
				ticker := time.NewTicker(time.Second * 10)
				defer ticker.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						s.eventsMetricsInc()
						if err := req.Send(&cronpb.ServiceEvent{
							Id:        utils.GetStrID(),
							Type:      cronpb.EventType_EVENT_REGISTER_HEARTBEAT_PING,
							EventTime: time.Now().Unix(),
						}); err != nil {
							wlog.Error("failed to send heartbeat request", zap.Error(err))
							onError()
							return
						}
						onPong()
					}
				}
			}()
		})
	}
}

func (s *cronRpc) RegisterAgentV2(req cronpb.Center_RegisterAgentV2Server) error {
	// 实现center与agent间的通信，复用stream模拟unary调用
	newRegisterInfoChannel := watchAgentResponse(req.Context(), req.Recv, s.app.StreamManagerV2().RecvStreamResponse)
	// 获取注册与反注册逻辑方法
	register, deRegister := s.buildAgentRegister(req.Context())
	// 链接关闭后调用反注册
	defer deRegister(func(allRegisteredMetas []infra.NodeMeta) {
		// 反注册后，需要将stream从内存中剔除
		for _, meta := range allRegisteredMetas {
			s.app.StreamManagerV2().RemoveStream(meta)
		}
	})

	// 注册成功后向agent下发任务的处理方法
	dispatchHandler := buildDispatchJobsV2Handler(func(ctx context.Context, e *cronpb.ServiceEvent) error {
		_, err := s.app.StreamManagerV2().SendEventWaitResponse(ctx, req, e)
		return err
	})

	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()

	lock := &sync.Mutex{}
	var receiveMetas []infra.NodeMeta
	heartbeatAsync := s.getHeartbeatKeeper(ctx, req, func() {
		lock.Lock()
		defer lock.Unlock()
		for _, v := range receiveMetas {
			if err := s.app.UpsertAgentActiveTime(v.System, v.Host); err != nil {
				wlog.Error("Failed to upsert agent active time", zap.Error(err), zap.String("client_ip", v.Host),
					zap.Int64("project_id", v.System))
			}
		}
	}, func() {
		// heartbeat error 关闭连接
		cancel()
	})

	for {
		select {
		case multiService := <-newRegisterInfoChannel:
			if multiService == nil {
				return nil
			}
			// 将agent信息进行注册
			err := register(multiService.info, func(nm []infra.NodeMeta) error {
				lock.Lock()
				receiveMetas = append(receiveMetas, nm...)
				lock.Unlock()
				// 完成注册后将stream缓存至内存中，方便后续中心与agent通信时使用
				for _, meta := range nm {
					s.app.StreamManagerV2().SaveStream(meta, req, cancel)
					// 下发对应项目的任务列表
					if err := s.app.DispatchAgentJob(meta.System, dispatchHandler(multiService.reqID, meta)); err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				return err
			}

			heartbeatAsync()

		case <-ctx.Done():
			return nil
		}
	}
}
