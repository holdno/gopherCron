package config

var serviceConf *ServiceConfig

type ClientConfig struct {
	Shell      string  `toml:"shell,omitempty"`
	LogLevel   string  `toml:"log_level"`
	LogFile    string  `toml:"log_file"`
	ReportAddr string  `toml:"report_addr"`
	Projects   []int64 `toml:"projects,omitempty"`
	Timeout    int     `toml:"timeout"`

	Token   string     `toml:"token"`
	Address string     `toml:"address"`
	Micro   Micro      `toml:"micro"`
	Mysql   *MysqlConf `toml:"mysql"`
}

type Project struct {
	Appid  int64  `toml:"appid"`
	Secret string `toml:"secret"`
}

// APIConfig 配置文件Root
type ServiceConfig struct {
	LogLevel string `toml:"log_level"`
	LogPath  string `toml:"log_path"`

	Deploy *DeployConf `toml:"deploy"` // host配置
	Etcd   *EtcdConf   `toml:"etcd"`
	Micro  Micro       `toml:"micro"`
	JWT    *JWTConf    `toml:"jwt"`
	Mysql  *MysqlConf  `toml:"mysql"`
}

type Micro struct {
	Endpoint string `toml:"endpoint"`
	OrgID    string `toml:"org_id"`
	Region   string `toml:"region"`
	Weigth   int32  `toml:"weigth"`
}

// DeployConf 部署配置
type DeployConf struct {
	Environment string `toml:"environment"`
	Timeout     int    `toml:"timeout"`
	ViewPath    string `toml:"view_path"`
	Host        string `toml:"host"`
}

// EtcdConf etcd配置
type EtcdConf struct {
	Service     []string `toml:"service"`
	Username    string   `toml:"username"`
	Password    string   `toml:"password"`
	DialTimeout int      `toml:"dialtimeout"`
	Prefix      string   `toml:"prefix"`
	Projects    []int64  `toml:"projects,omitempty"`
	Shell       string   `toml:"shell,omitempty"`
}

// MongoDBConf mongodb连接配置
type MongoDBConf struct {
	Service       []string `toml:"service"`
	Username      string   `toml:"username"`
	Password      string   `toml:"password"`
	Table         string   `toml:"table"`
	AuthMechanism string   `toml:"auth_mechanism"`
}

type MysqlConf struct {
	Service  string `toml:"service"`
	Username string `toml:"username"`
	Password string `toml:"password"`
	Database string `toml:"database"`
}

// JWTConf 签名方法配置
type JWTConf struct {
	Secret string `toml:"secret"`
	Exp    int    `toml:"exp"`
}

// InitServiceConfig 获取api相关配置
func InitServiceConfig(path string) *ServiceConfig {
	if path == "" {
		return nil
	}

	var c ServiceConfig
	LoadFrom(path, &c)
	serviceConf = &c
	return &c
}

// InitServiceConfig 获取api相关配置
func InitClientConfig(path string) *ClientConfig {
	if path == "" {
		return nil
	}

	var c ClientConfig
	LoadFrom(path, &c)
	return &c
}

// GetServiceConfig 获取服务配置
func GetServiceConfig() *ServiceConfig {
	if serviceConf != nil {
		return serviceConf
	}
	return nil
}
