package client

import (
	"context"

	"github.com/holdno/gopherCron/agent"
	"github.com/holdno/gopherCron/pkg/cronpb"
)

type agentSrv struct {
	app agent.Client
	cronpb.UnimplementedAgentServer
}

func (a *agentSrv) Schedule(ctx context.Context, req *cronpb.ScheduleRequest) (*cronpb.Result, error) {
	return a.app.Schedule(ctx, req)
}

func (a *agentSrv) CheckRunning(ctx context.Context, req *cronpb.CheckRunningRequest) (*cronpb.Result, error) {
	return a.app.CheckRunning(ctx, req)
}

func (a *agentSrv) KillTask(ctx context.Context, req *cronpb.KillTaskRequest) (*cronpb.Result, error) {
	return a.app.KillTask(ctx, req)
}

func (a *agentSrv) ProjectTaskHash(ctx context.Context, req *cronpb.ProjectTaskHashRequest) (*cronpb.ProjectTaskHashReply, error) {
	return a.app.ProjectTaskHash(ctx, req)
}

