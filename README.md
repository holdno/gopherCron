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

### V2.2.0 2023-07-25

增加边缘与中心间的鉴权，配置文件会有调整，等后面稍微有空时我会整理一个使用文档出来。

### V2

全新 V2 版本支持 workflow，重写任务调度方式，移除 client 对 etcd 的依赖

### 依赖

- Etcd # 服务注册与发现
- Mysql # 任务日志存储

### 引用

- [Gin](https://github.com/gin-gonic/gin) 提供 webapi
- [gopherCronFe](https://github.com/holdno/gopherCronFe) 提供可视化管理界面(已将构建后的文件内置于 dist/view 目录下)
- [cronexpr](https://github.com/gorhill/cronexpr) 提供 cron 表达式解析器
- 🍉[水瓜](https://github.com/spacegrower/watermelon) 提供服务注册发现能力

### 实现功能

- 秒级定时任务
- 任务日志查看
- 随时结束任务进程
- 分布式扩展
- 健康节点检测 (分项目显示对应的健康节点 IP 及节点数)
- workflow 任务编排

### 任务日志集中上报

1.10.x 版本中 client 配置增加了 report_addr 项，该配置接收一个 http 接口  
配置后，任务日志将通过 http 发送到该地址进行集中处理  
可通过请求中的 Head 参数 Report-Type 来判断是告警还是日志来做对应的处理  
日志结构(参考：common/protocol.go 下的 TaskExecuteResult)：

```golang
// TaskExecuteResult 任务执行结果
type TaskExecuteResult struct {
	ExecuteInfo *TaskExecutingInfo `json:"execute_info"`
	Output      string             `json:"output"`     // 程序输出
	Err         string             `json:"error"`      // 是否发生错误
	StartTime   time.Time          `json:"start_time"` // 开始时间
	EndTime     time.Time          `json:"end_time"`   // 结束时间
}
```

v2.1.0 + 版本中移除了 client 对 etcd 的依赖

日志上报相关代码参考 app/taskreport.go

### cronexpr 秒级 cron 表达式介绍(引用)

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

下载项目到本地并编译，根据 cmd 文件夹下 service 和 client 中包含的 conf/config-default.toml 进行配置

### Admin 管理页面

访问地址: localhost:6306/admin

> 管理员初始账号密码为 admin 123456

![image](http://img.holdno.com/github/holdno/gopher_cron/admin_home.png)

![image](http://img.holdno.com/github/holdno/gopher_cron/admin_task.png)

### 注意

client 配置文件中的 project 配置需要用户先部署 service  
在 service 中创建项目后可以获得项目 ID  
需要将项目 ID 填写在 client 的配置中该 client 才会调度这个项目的任务
