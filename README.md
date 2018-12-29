## gopherCron  
开箱即用的分布式可视化crontab  

可以通过配置文件制定某个节点所受理的业务线，从而做到业务统一管理但隔离调度
### 依赖  
- Etcd   # 服务注册与发现
- MongoDB  # 任务日志存储
- cronexpr # github.com/gorhill/cronexpr cron表达式解析器  
  
### 实现功能  
- 秒级定时任务  
- 任务日志查看  
- 随时结束任务进程  
- 分布式扩展  
- 健康节点检测 (分项目显示对应的健康节点IP及节点数)

### 使用方法  
下载项目到本地并编译，根据cmd文件夹下service和client中包含的conf/config-default.toml进行配置  

#### service 部署  
``` shell
$ ./service -conf ./conf/config-default.toml // 配置文件名请随意  
2018-12-29 14:31:15 listening and serving HTTP on 0.0.0.0:6306

```
#### client 部署  
 
``` shell
$ ./client -conf ./conf/config-default.toml
// 等待如下输入即启动成功 
InitExecuter
InitScheduler
TaskWatcher
TaskKiller
Register
client runing
```  

