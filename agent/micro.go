package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"syscall"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/pkg/cronpb"
	"github.com/holdno/gopherCron/pkg/infra"
	"github.com/holdno/gopherCron/pkg/infra/register"
	"go.uber.org/zap"

	winfra "github.com/spacegrower/watermelon/infra"
	wregister "github.com/spacegrower/watermelon/infra/register"
	"github.com/spacegrower/watermelon/infra/wlog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const version = "v2.1.0"

func (a *client) SetupMicroService() *winfra.Srv[infra.NodeMetaRemote] {
	register := a.MustSetupRemoteRegister()
	cfg := a.Cfg()
	if cfg.Address == "" {
		cfg.Address = a.GetIP()
	}
	newsrv := infra.NewAgentServer()
	srv := newsrv(func(srv *grpc.Server) {
		cronpb.RegisterAgentServer(srv, a)
	}, newsrv.WithOrg(cfg.Micro.OrgID),
		newsrv.WithRegion(cfg.Micro.Region),
		newsrv.WithSystems(cfg.Projects),
		newsrv.WithWeight(cfg.Micro.Weigth),
		newsrv.WithAddress([]infra.Address{{ListenAddress: cfg.Address}}),
		newsrv.WithTags(map[string]string{
			"agent-version": version,
		}),
		newsrv.WithServiceRegister(register))
	go func() {
		fmt.Println("start grpc service, on", cfg.Address)
		srv.RunUntil(syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	}()
	return srv
}

func (a *client) MustSetupRemoteRegister() wregister.ServiceRegister[infra.NodeMetaRemote] {

	genMetadata := func(ctx context.Context) context.Context {
		return metadata.NewOutgoingContext(ctx, metadata.New(map[string]string{"gophercron-agent-ip": a.localip}))
	}

	r, err := register.NewRemoteRegister(a.localip, func() (register.CenterClient, error) {
		wlog.Debug("start build new center client")
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.cfg.Timeout)*time.Second)
		defer cancel()
		cc, err := grpc.DialContext(ctx, a.cfg.Micro.Endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithKeepaliveParams(keepalive.ClientParameters{
				Time:    time.Second * 10,
				Timeout: time.Second * 3,
			}),
			grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
				return invoker(genMetadata(ctx), method, req, reply, cc, opts...)
			}),
			grpc.WithStreamInterceptor(func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
				return streamer(genMetadata(ctx), desc, cc, method, opts...)
			}))
		if err != nil {
			wlog.Error("failed to dial center service", zap.String("endpoint", a.cfg.Micro.Endpoint), zap.Error(err))
			return register.CenterClient{}, err
		}
		a.centerSrv = register.CenterClient{
			CenterClient: cronpb.NewCenterClient(cc),
			Cc:           cc,
		}

		return a.centerSrv, nil
	}, func(e *cronpb.Event) {
		a.handlerEventFromCenter(e)
	})
	if err != nil {
		panic(err)
	}
	return r
}

func (a *client) CheckRunning(ctx context.Context, req *cronpb.CheckRunningRequest) (*cronpb.Result, error) {
	if _, taskExecuting := a.scheduler.CheckTaskExecuting(common.BuildSchedulerKey(req.ProjectId, req.TaskId)); taskExecuting {
		return &cronpb.Result{
			Result:  true,
			Message: "running",
		}, nil
	}
	return &cronpb.Result{
		Result:  false,
		Message: "not running",
	}, nil
}

func (a *client) Schedule(ctx context.Context, req *cronpb.ScheduleRequest) (*cronpb.Result, error) {
	unmarshalTask := func(value []byte) (*common.TaskInfo, error) {
		var task common.TaskInfo
		if err := json.Unmarshal(value, &task); err != nil {
			return nil, status.Error(codes.InvalidArgument, "failed to unmarshal task")
		}
		return &task, nil
	}

	switch req.Event.Type {
	case common.REMOTE_EVENT_TMP_SCHEDULE:
		task, err := unmarshalTask(req.Event.Value)
		if err != nil {
			return nil, err
		}
		if _, taskExecuting := a.scheduler.CheckTaskExecuting(task.SchedulerKey()); taskExecuting {
			return nil, status.Error(codes.AlreadyExists, "the task already executing, try again later")
		}
		// plan, exist := a.GetPlan(task.SchedulerKey())
		// if !exist {
		// 	return nil, status.Error(codes.NotFound, "task plan is not found")
		// }
		// 兼容临时调度
		plan, err := common.BuildTaskSchedulerPlan(task)
		if err != nil {
			return nil, status.Error(codes.Aborted, "failed to build task plan: "+err.Error())
		}
		a.TryStartTask(plan)
	case common.REMOTE_EVENT_WORKFLOW_SCHEDULE:
		task, err := unmarshalTask(req.Event.Value)
		if err != nil {
			return nil, err
		}
		// 下发 task
		if _, taskExecuting := a.scheduler.CheckTaskExecuting(task.SchedulerKey()); taskExecuting {
			return nil, status.Error(codes.AlreadyExists, "the task already executing, try again later")
		}
		plan, err := common.BuildWorkflowTaskSchedulerPlan(task)
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to build workflow task schedule plan")
		}
		a.TryStartTask(plan)
	case common.REMOTE_EVENT_TASK_STOP:
		task, err := unmarshalTask(req.Event.Value)
		if err != nil {
			return nil, err
		}
		if task, taskExecuting := a.scheduler.CheckTaskExecuting(task.SchedulerKey()); taskExecuting {
			task.CancelFunc()
		}
	}

	return &cronpb.Result{
		Result:  true,
		Message: "ok",
	}, nil
}

func (a *client) KillTask(ctx context.Context, req *cronpb.KillTaskRequest) (*cronpb.Result, error) {
	if taskExecuteInfo, taskExecuting := a.scheduler.CheckTaskExecuting(common.GenTaskSchedulerKey(req.ProjectId, req.TaskId)); taskExecuting {
		taskExecuteInfo.CancelFunc()
		return &cronpb.Result{
			Result: true,
		}, nil
	}
	return &cronpb.Result{
		Result:  false,
		Message: "task not running",
	}, nil
}

func (a *client) ProjectTaskHash(ctx context.Context, req *cronpb.ProjectTaskHashRequest) (*cronpb.ProjectTaskHashReply, error) {
	hash, latestUpdateTime := a.scheduler.GetProjectTaskHash(req.ProjectId)
	return &cronpb.ProjectTaskHashReply{
		Hash:             hash,
		LatestUpdateTime: latestUpdateTime,
	}, nil
}
