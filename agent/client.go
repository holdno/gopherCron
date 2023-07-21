package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/config"
	"github.com/holdno/gopherCron/pkg/cronpb"
	"github.com/holdno/gopherCron/pkg/daemon"
	"github.com/holdno/gopherCron/pkg/infra"
	"github.com/holdno/gopherCron/pkg/infra/register"
	"github.com/holdno/gopherCron/pkg/warning"
	"github.com/holdno/gopherCron/utils"
	"go.uber.org/zap"

	wregister "github.com/spacegrower/watermelon/infra/register"
	"github.com/spacegrower/watermelon/infra/wlog"
	"google.golang.org/grpc"
)

type client struct {
	localip string
	logger  wlog.Logger

	scheduler  *TaskScheduler
	configPath string
	cfg        *config.ClientConfig

	daemon *daemon.ProjectDaemon

	isClose   bool
	closeChan chan struct{}
	ClientTaskReporter
	// etcd       protocol.EtcdManager
	// protocol.ClientEtcdManager
	warning.Warner
	centerSrv register.CenterClient
	onCommand func(*cronpb.CommandRequest) (*cronpb.Result, error)
	author    *Author

	cronpb.UnimplementedAgentServer
}

type Client interface {
	GetIP() string
	Loop()
	Close()
	Down() chan struct{}
	Cfg() *config.ClientConfig
	warning.Warner

	MustSetupRemoteRegister() wregister.ServiceRegister[infra.NodeMetaRemote]
	Schedule(ctx context.Context, req *cronpb.ScheduleRequest) (*cronpb.Result, error)
	CheckRunning(ctx context.Context, req *cronpb.CheckRunningRequest) (*cronpb.Result, error)
	KillTask(ctx context.Context, req *cronpb.KillTaskRequest) (*cronpb.Result, error)
	ProjectTaskHash(ctx context.Context, req *cronpb.ProjectTaskHashRequest) (*cronpb.ProjectTaskHashReply, error)
	Command(ctx context.Context, req *cronpb.CommandRequest) (*cronpb.Result, error)
}

func (agent *client) loadConfigAndSetupAgentFunc() func() error {
	inited := false
	var shutDown func()

	return func() error {
		// var err error
		cfg := config.InitClientConfig(agent.configPath)
		if !inited {
			inited = true
			agent.cfg = &config.ClientConfig{}
			if agent.configPath == "" {
				panic("empty config path")
			}

			var clusterID int64 = 1

			wlog.SetGlobalLogger(wlog.NewLogger(&wlog.Config{
				Level: wlog.ParseLevel(cfg.LogLevel),
				File:  cfg.LogFile,
				RotateConfig: &wlog.RotateConfig{
					MaxAge:  24,
					MaxSize: 100,
				},
			}))

			// why 1024. view https://github.com/holdno/snowFlakeByGo
			utils.InitIDWorker(clusterID % 1024)
			agent.logger = wlog.With()
			agent.scheduler = initScheduler()
			agent.daemon = daemon.NewProjectDaemon(nil, agent.logger)

			if cfg.Address != "" {
				agent.localip = strings.Split(cfg.Address, ":")[0]
			}
			if agent.localip == "" {
				var err error
				if agent.localip, err = utils.GetLocalIP(); err != nil {
					agent.logger.Panic("failed to get local ip", zap.Error(err))
				}
			}

		} else if agent.configPath == "" {
			return fmt.Errorf("invalid config path")
		}

		if cfg.ReportAddr != "" {
			agent.logger.Info(fmt.Sprintf("init http task log reporter, address: %s", cfg.ReportAddr))
			reporter := warning.NewHttpReporter(cfg.ReportAddr)
			agent.ClientTaskReporter = reporter
			agent.Warner = reporter
		} else {
			agent.logger.Info("init default task log reporter, it must be used mysql config")
			agent.ClientTaskReporter = NewDefaultTaskReporter(agent.logger, cfg.Mysql)
		}

		if agent.Warner == nil {
			agent.Warner = warning.NewDefaultWarner(agent.logger)
		}

		addProjects, _ := agent.daemon.DiffAndAddProjects(cfg.Auth.Projects)
		agent.logger.Debug("diff projects", zap.Any("projects", addProjects))

		// remove all plan
		agent.scheduler.RemoveAll()

		agent.cfg = cfg
		if shutDown != nil {
			shutDown()
		}
		agent.author = NewAuthor(cfg.Auth.Projects)
		srv := agent.SetupMicroService()
		shutDown = func() {
			srv.ShutDown()
		}

		return nil
	}
}

func NewClient(configPath string) Client {
	agent := &client{
		configPath: configPath,
		isClose:    false,
		closeChan:  make(chan struct{}),
	}
	setupFunc := agent.loadConfigAndSetupAgentFunc()

	setupFunc()

	agent.onCommand = func(e *cronpb.CommandRequest) (*cronpb.Result, error) {
		var err error
		switch e.Command {
		case common.AGENT_COMMAND_RELOAD_CONFIG:
			err = setupFunc()
		default:
			err = fmt.Errorf("unsupport command %s", e.Command)
		}
		if err != nil {
			return nil, err
		}
		return &cronpb.Result{
			Result:  true,
			Message: "ok",
		}, nil
	}

	return agent
}

func (c *client) Command(ctx context.Context, req *cronpb.CommandRequest) (*cronpb.Result, error) {
	return c.onCommand(req)
}

func (c *client) Cfg() *config.ClientConfig {
	return c.cfg
}

func (c *client) Close() {
	if !c.isClose {
		c.isClose = true
		c.daemon.Close()
		c.scheduler.Stop()
		<-c.closeChan
		wlog.Info("agent has been shut down")
	}
}

func (c *client) Down() chan struct{} {
	return c.closeChan
}

func (c *client) GetIP() string {
	return c.localip
}

func (a *client) GetStatusReporter() func(ctx context.Context, in *cronpb.ScheduleReply, opts ...grpc.CallOption) (*cronpb.Result, error) {
	return a.centerSrv.StatusReporter
}

func (a *client) GetCenterSrv() cronpb.CenterClient {
	return a.centerSrv.CenterClient
}
