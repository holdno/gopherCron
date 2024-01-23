package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/config"
	"github.com/holdno/gopherCron/pkg/cronpb"
	"github.com/holdno/gopherCron/pkg/daemon"
	"github.com/holdno/gopherCron/pkg/infra/register"
	"github.com/holdno/gopherCron/pkg/warning"
	"github.com/holdno/gopherCron/utils"

	"github.com/spacegrower/watermelon/infra/wlog"
	"go.uber.org/zap"
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
	// etcd       protocol.EtcdManager
	// protocol.ClientEtcdManager
	warning.Warner
	centerSrv       register.CenterClient
	srvShutdownFunc func()
	onCommand       func(*cronpb.CommandRequest) (*cronpb.Result, error)
	authenticator   *Authenticator

	cronpb.UnimplementedAgentServer
	metrics *Metrics
}

type Client interface {
	GetIP() string
	Loop()
	Close()
	Down() chan struct{}
	Cfg() *config.ClientConfig
	warning.Warner

	Schedule(ctx context.Context, req *cronpb.ScheduleRequest) (*cronpb.Result, error)
	CheckRunning(ctx context.Context, req *cronpb.CheckRunningRequest) (*cronpb.Result, error)
	KillTask(ctx context.Context, req *cronpb.KillTaskRequest) (*cronpb.Result, error)
	ProjectTaskHash(ctx context.Context, req *cronpb.ProjectTaskHashRequest) (*cronpb.ProjectTaskHashReply, error)
	Command(ctx context.Context, req *cronpb.CommandRequest) (*cronpb.Result, error)
}

func (agent *client) loadConfigAndSetupAgentFunc() func() error {
	inited := false

	return func() error {
		// var err error
		cfg := config.InitClientConfig(agent.configPath)

		if !inited {
			inited = true
			agent.cfg = &config.ClientConfig{}
			if agent.configPath == "" {
				panic("empty config path")
			}
			if cfg.LogAge == 0 {
				cfg.LogAge = 1
			}
			if cfg.LogSize == 0 {
				cfg.LogSize = 100
			}
			if cfg.Timeout == 0 {
				cfg.Timeout = 5
			}

			wlog.SetGlobalLogger(wlog.NewLogger(&wlog.Config{
				Name:  "gophercron-agent",
				Level: wlog.ParseLevel(cfg.LogLevel),
				File:  cfg.LogFile,
				RotateConfig: &wlog.RotateConfig{
					MaxAge:     cfg.LogAge,
					MaxSize:    cfg.LogSize,
					MaxBackups: cfg.LogBackups,
					Compress:   cfg.LogCompress,
				},
			}))

			// 任务日志及任务结果上报时会带有agent_ip，所以这边snowflake的cluster_id可以在允许值内随便声生成一个
			var clusterID int64 = int64(utils.Random(0, 1024))
			// why 1024. view https://github.com/holdno/snowFlakeByGo
			utils.InitIDWorker(clusterID % 1024)
			agent.logger = wlog.With()
			agent.scheduler = initScheduler(agent)
			agent.daemon = daemon.NewProjectDaemon(nil, agent.logger)

			if cfg.RegisterAddress != "" {
				agent.localip = strings.Split(cfg.RegisterAddress, ":")[0]
			}
			if agent.localip == "" {
				var err error
				if agent.localip, err = utils.GetLocalIP(); err != nil {
					agent.logger.Panic("failed to get local ip", zap.Error(err))
				}
			}
			agent.metrics = NewMonitor(agent.localip, cfg.Prometheus.PushGateway, cfg.Prometheus.JobName)
		} else if agent.configPath == "" {
			return fmt.Errorf("invalid config path")
		}
		agent.cfg = cfg

		if cfg.ReportAddr != "" {
			agent.logger.Info(fmt.Sprintf("init http task log reporter, address: %s", cfg.ReportAddr))
			agent.Warner = warning.NewHttpReporter(cfg.ReportAddr, func() (string, error) {
				if agent.authenticator != nil {
					ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Timeout)*time.Second)
					defer cancel()
					return agent.authenticator.GetToken(ctx, agent.centerSrv)
				}
				return "", errors.New("author is not init")
			})
		} else {
			agent.Warner = warning.NewDefaultWarner(agent.logger)
		}

		addProjects, _ := agent.daemon.DiffAndAddProjects(cfg.Auth.Projects)
		agent.logger.Debug("diff projects", zap.Any("projects", addProjects))

		// remove all old plan
		agent.scheduler.RemoveAll()
		agent.authenticator = NewAuthenticator(cfg.Auth.Projects)

		if agent.srvShutdownFunc != nil { // 先停掉旧配置启动的服务
			agent.srvShutdownFunc()
		}
		srv := agent.SetupMicroService() // 重新注册服务，注册后拿到新的projects对应的tasks
		agent.srvShutdownFunc = func() { // 注册服务的停止方法
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
		close(c.closeChan)
		// 中断服务，停止接收新的调度信息
		if c.srvShutdownFunc != nil {
			c.srvShutdownFunc()
		}
		// 关闭本地调度器 并 等待执行中的任务结束
		if c.scheduler != nil {
			c.scheduler.Stop()
		}
		// 等待所有任务运行结束
		if c.daemon != nil {
			c.daemon.Close()
		}

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
