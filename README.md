<p align="center"><a href="/" target="_blank" rel="noopener noreferrer"><img width="200" src="http://img.holdno.com/github/holdno/gopher_cron/gopherCronLogo.png" alt="firetower logo"></a></p>

<p align="center">
  <img src="https://img.shields.io/badge/download-fast-brightgreen.svg" alt="Downloads"></a>
  <img src="https://img.shields.io/badge/build-passing-brightgreen.svg" alt="Build Status">
  <img src="https://img.shields.io/badge/package%20utilities-go modules-blue.svg" alt="Package Utilities">
  <img src="https://img.shields.io/badge/golang-1.11.0-%23ff69b4.svg" alt="Version">
  <img src="https://img.shields.io/badge/license-MIT-brightgreen.svg" alt="license">
</p>
<h1 align="center">GopherCron</h1>
开箱即用的分布式可视化crontab  

可以通过配置文件指定某个节点所受理的业务线，从而做到业务统一管理但隔离调度
### 依赖  
- Etcd   # 服务注册与发现
- Gin # webapi 提供可视化操作
- Mysql  # 任务日志存储
- cronexpr # github.com/gorhill/cronexpr cron表达式解析器  
  
### 实现功能  
- 秒级定时任务  
- 任务日志查看  
- 随时结束任务进程  
- 分布式扩展  
- 健康节点检测 (分项目显示对应的健康节点IP及节点数)    

### 任务日志集中上报  
1.10.x版本中client配置增加了report_addr项，该配置接收一个http接口  
配置后，任务日志将通过http发送到该地址进行集中处理  
可通过请求中的Head参数 Report-Type 来判断是告警还是日志来做对应的处理  
日志结构(参考：common/protocol.go 下的 TaskExecuteResult)：  
``` golang
// TaskExecuteResult 任务执行结果
type TaskExecuteResult struct {
	ExecuteInfo *TaskExecutingInfo `json:"execute_info"`
	Output      string             `json:"output"`     // 程序输出
	Err         string             `json:"error"`      // 是否发生错误
	StartTime   time.Time          `json:"start_time"` // 开始时间
	EndTime     time.Time          `json:"end_time"`   // 结束时间
}
```   
日志上报相关代码参考 app/taskreport.go  

### cronexpr 秒级cron表达式介绍(引用)  

    * * * * * * * 
    Field name     Mandatory?   Allowed values    Allowed special characters
    ----------     ----------   --------------    --------------------------
    Seconds        No           0-59              * / , -
    Minutes        Yes          0-59              * / , -
    Hours          Yes          0-23              * / , -
    Day of month   Yes          1-31              * / , - L W
    Month          Yes          1-12 or JAN-DEC   * / , -
    Day of week    Yes          0-6 or SUN-SAT    * / , - L #
    Year           No           1970–2099         * / , -

### 使用方法  
下载项目到本地并编译，根据cmd文件夹下service和client中包含的conf/config-default.toml进行配置  

#### service 配置文件  
``` toml 
log_level = "debug"

[deploy]
# 当前的环境:dev、release
environment = "release"
# 对外提供的端口
host = ["0.0.0.0:6306"]
# 数据库操作超时时间
timeout = 5  # 秒为单位
# 前端文件路径
view_path = "./view"

# etcd
[etcd]
service = ["0.0.0.0:2379"]
username = ""
password = ""
dialtimeout = 5000
# etcd kv存储的key前缀 用来与其他业务做区分
prefix = "/gopher_cron"

[mysql]
service="0.0.0.0:3306"
username=""
password=""
database=""

# jwt用来做api的身份校验
[jwt]
# jwt签名的secret 建议修改
secret = "fjskfjls2ifeew2mn"
exp = 168  # token 有效期(小时)
```

#### service 部署  
``` shell
$ ./gophercron service -c ./config/service-config-default.toml // 配置文件名请随意  
2019-01-18 00:00:45 listening and serving HTTP on 0.0.0.0:6306

```

#### client 配置文件
``` toml
log_level = "debug"
# 日志统一上报接口(http协议)，如配置此接口可忽略mysql的配置
report_addr = "" 

[deploy]
# 当前的环境:dev、release
environment = "release"
# 数据库操作超时时间
timeout = 5  # 秒为单位

# etcd
[etcd]
service = ["0.0.0.0:2379"]
username = ""
password = ""
dialtimeout = 5000
# etcd kv存储的key前缀 用来与其他业务做区分
prefix = "/gopher_cron"
# 当前节点需要处理的项目ID（先通过service创建项目并获取项目ID）
projects = [1,2]
# 命令调用脚本 /bin/sh  /bin/bash 根据自己系统情况决定
shell = "/bin/bash"

[mysql]
service="0.0.0.0:3306"
username=""
password=""
database=""
```
#### client 部署  
 
``` shell
$ ./gophercron client -c ./config/client-config-default.toml
// 等待如下输入即启动成功 
{"level":"info","msg":"task watcher start","project_id":14,"time":"2020-06-17T18:16:10+08:00"}
{"level":"info","msg":"[agent - TaskKiller] new task killer, project_id: 14","time":"2020-06-17T18:16:10+08:00"}
{"level":"info","msg":"[agent - TaskWatcher] new task watcher, project_id: 14","time":"2020-06-17T18:16:10+08:00"}
{"level":"info","msg":"[agent - Register] new project agent register, project_id: 14","time":"2020-06-17T18:16:10+08:00"}
...
```  

### Admin 管理页面  
访问地址: localhost:6306/admin    
  
> 管理员初始账号密码为 admin  123456  

![image](http://img.holdno.com/github/holdno/gopher_cron/admin_home.png)  

![image](http://img.holdno.com/github/holdno/gopher_cron/admin_task.png)

### 注意   
client配置文件中的project配置需要用户先部署service  
在service中创建项目后可以获得项目ID  
需要将项目ID填写在client的配置中该client才会调度这个项目的任务  

