package config

var serviceConf *ServiceConfig

// APIConfig 配置文件Root
type ServiceConfig struct {
	Env        string       `toml:"env"`
	LogLevel   string       `toml:"log_level"`
	ReportAddr string       `toml:"report_addr"`
	Deploy     *DeployConf  `toml:"deploy"` // host配置
	Etcd       *EtcdConf    `toml:"etcd"`
	MongoDB    *MongoDBConf `toml:"mongodb"`
	JWT        *JWTConf     `toml:"jwt"`
	Mysql      *MysqlConf   `toml:"mysql"`
}

// DeployConf 部署配置
type DeployConf struct {
	Environment string   `toml:"environment"`
	Timeout     int      `toml:"timeout"`
	Host        []string `toml:"host"`
	ViewPath    string   `toml:"view_path"`
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

// GetServiceConfig 获取服务配置
func GetServiceConfig() *ServiceConfig {
	if serviceConf != nil {
		return serviceConf
	}
	return nil
}
