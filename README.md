## gopherCron  
开箱即用的分布式可视化crontab  

可以通过配置文件指定某个节点所受理的业务线，从而做到业务统一管理但隔离调度
### 依赖  
- Etcd   # 服务注册与发现
- Gin # webapi 提供可视化操作
- MongoDB  # 任务日志存储
- cronexpr # github.com/gorhill/cronexpr cron表达式解析器  
  
### 实现功能  
- 秒级定时任务  
- 任务日志查看  
- 随时结束任务进程  
- 分布式扩展  
- 健康节点检测 (分项目显示对应的健康节点IP及节点数)  

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

#### 配置文件  
``` toml 
[deploy]
# 当前的环境:dev、release
environment = "dev"
# 对外提供的端口
host = ["0.0.0.0:6306"]

# etcd
[etcd]
service = ["0.0.0.0:2379"]
dialtimeout = 5000
# etcd kv存储的key前缀 用来与其他业务做区分
prefix = "/gopher_cron"

[mongodb]
service = ["mongodb://0.0.0.0:27017"]
username = ""
password = ""
# 可自行修改数据库表名 新表会自动生成admin账号
table = "gopher_cron"
# mongodb的验证方式
auth_mechanism = "SCRAM-SHA-1"

# jwt用来做api的身份校验
[jwt]
# jwt签名的secret 建议修改
secret = "fjskfjls2ifeew2mn"
exp = 12 # token 有效期
```

#### service 部署  
``` shell
$ ./service -conf ./conf/config-default.toml // 配置文件名请随意  
2019-01-18 00:00:45 listening and serving HTTP on 0.0.0.0:6306

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

### Admin 管理页面  
访问地址: localhost:6306/admin    
  
> 管理员初始账号密码为 admin  123456  

![image](http://img.holdno.com/github/holdno/gopher_cron/admin.png)

### TODO   
- 项目组概念  多个用户共享某些项目  
- 继续完善admin后台管理页面   
  - 修改密码  
  - 管理员创建新用户
  - 系统公告 
  - 项目相关图标  
  - 日志管理
