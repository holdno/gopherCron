package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/config"
	"github.com/holdno/gopherCron/pkg/daemon"
	"github.com/holdno/gopherCron/pkg/etcd"
	"github.com/holdno/gopherCron/pkg/logger"
	"github.com/holdno/gopherCron/pkg/panicgroup"
	"github.com/holdno/gopherCron/pkg/warning"
	"github.com/holdno/gopherCron/protocol"
	"github.com/holdno/gopherCron/utils"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/sirupsen/logrus"
)

type client struct {
	localip    string
	logger     *logrus.Logger
	etcd       protocol.EtcdManager
	scheduler  *TaskScheduler
	configPath string
	cfg        *config.ServiceConfig

	daemon *daemon.ProjectDaemon

	isClose   bool
	closeChan chan struct{}

	panicgroup.PanicGroup
	ClientTaskReporter
	protocol.ClientEtcdManager
	warning.Warner
}

type Client interface {
	Go(f func())
	GetIP() string
	Loop()
	ResultReport(result *common.TaskExecuteResult) error
	Close()
	Down() chan struct{}
	warning.Warner
}

type ClientOptions func(a *client)

func ClientWithTaskReporter(reporter ClientTaskReporter) ClientOptions {
	return func(agent *client) {
		agent.ClientTaskReporter = reporter
	}
}

func ClientWithWarning(w warning.Warner) ClientOptions {
	return func(a *client) {
		a.Warner = w
	}
}

func (agent *client) loadConfigAndSetupAgentFunc() func() {
	inited := false

	return func() {
		var err error
		cfg := config.InitServiceConfig(agent.configPath)
		if !inited {
			inited = true
			agent.cfg = &config.ServiceConfig{}
			if agent.configPath == "" {
				panic("empty config path")
			}

			if agent.etcd, err = etcd.Connect(cfg.Etcd); err != nil {
				panic(err)
			}

			agent.ClientEtcdManager = protocol.NewClientEtcdManager(agent.etcd, cfg.Etcd.Projects)

			clusterID, err := agent.etcd.Inc(cfg.Etcd.Prefix + common.CLUSTER_AUTO_INDEX)
			if err != nil {
				panic(err)
			}

			// why 1024. view https://github.com/holdno/snowFlakeByGo
			utils.InitIDWorker(clusterID % 1024)
			agent.logger = logger.MustSetup(cfg.LogLevel)
			agent.scheduler = initScheduler()
			agent.daemon = daemon.NewProjectDaemon(nil, agent.logger)
		} else if agent.configPath == "" {
			return
		}

		if cfg.ReportAddr != agent.cfg.ReportAddr {
			agent.logger.Infof("init http task log reporter, address: %s", cfg.ReportAddr)
			reporter := NewHttpReporter(cfg.ReportAddr)
			agent.ClientTaskReporter = reporter
			agent.Warner = reporter
		} else if agent.ClientTaskReporter == nil {
			agent.logger.Info("init default task log reporter, it must be used mysql config")
			agent.ClientTaskReporter = NewDefaultTaskReporter(agent.logger, cfg.Mysql)
		}

		if agent.Warner == nil {
			agent.Warner = warning.NewDefaultWarner(agent.logger)
		}

		addProjects, _ := agent.daemon.DiffProjects(cfg.Etcd.Projects)
		agent.logger.WithField("projects", addProjects).Debug("diff projects")

		agent.TaskWatcher(addProjects)
		agent.TaskKiller(addProjects)
		agent.Register(addProjects)

		agent.cfg = cfg
	}

}

func NewClient(configPath string, opts ...ClientOptions) Client {
	var err error

	agent := &client{
		configPath: configPath,
		isClose:    false,
		closeChan:  make(chan struct{}),
	}
	agent.configPath = configPath
	setupFunc := agent.loadConfigAndSetupAgentFunc()

	if agent.localip, err = utils.GetLocalIP(); err != nil {
		agent.logger.Error("failed to get local ip")
	}

	for _, opt := range opts {
		opt(agent)
	}

	agent.PanicGroup = panicgroup.NewPanicGroup(func(err error) {
		reserr := agent.Warning(warning.WarningData{
			Data:    err.Error(),
			Type:    warning.WarningTypeSystem,
			AgentIP: agent.localip,
		})
		if reserr != nil {
			agent.logger.WithFields(logrus.Fields{
				"desc":         reserr,
				"source_error": err,
			}).Error("panicgroup: failed to warning panic error")
		}
	})

	setupFunc()

	agent.OnCommand(func(command string) {
		switch command {
		case common.AGENT_COMMAND_RELOAD_CONFIG:
			setupFunc()
		}
	})

	return agent
}

func (c *client) Close() {
	if !c.isClose {
		c.isClose = true
		c.daemon.Close()
		c.scheduler.Stop()
		<-c.closeChan
		fmt.Println("shut down")
	}
}

func (c *client) Down() chan struct{} {
	return c.closeChan
}

func (c *client) GetIP() string {
	return c.localip
}

func (agent *client) OnCommand(f func(command string)) {
	var (
		err     error
		preKey  = common.BuildAgentRegisteKey(agent.localip)
		getResp *clientv3.GetResponse
	)

	if err = utils.RetryFunc(5, func() error {
		if getResp, err = agent.etcd.KV().Get(context.TODO(), preKey, clientv3.WithPrefix()); err != nil {
			return err
		}
		return nil
	}); err != nil {
		err = agent.Warning(warning.WarningData{
			Data:    fmt.Sprintf("etcd kv get error: %s, client_ip: %s", err.Error(), agent.localip),
			Type:    warning.WarningTypeSystem,
			AgentIP: agent.GetIP(),
		})
		if err != nil {
			agent.logger.Errorf("failed to push warning, %s", err.Error())
		}
		return
	}

	watchResp := agent.etcd.Watcher().Watch(context.TODO(), strings.TrimRight(preKey, "/"),
		clientv3.WithRev(getResp.Header.Revision+1), clientv3.WithPrefix())
	agent.Go(func() {
		for resp := range watchResp {
			for _, watchEvent := range resp.Events {
				switch watchEvent.Type {
				case mvccpb.PUT:
					command := common.ExtractAgentCommand(string(watchEvent.Kv.Key))
					f(command)
				case mvccpb.DELETE:
				default:
					agent.logger.Warnf("the current version can not support this event, event: %d", watchEvent.Type)
				}
			}
		}
	})
}
