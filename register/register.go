package register

import (
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"time"
)

// MsRegister 注册中心, 服务名称获取服务的地址
type MsRegister interface {
	CreateClient(option MsRegisterOption) error
	RegisterService(serviceName string, host string, port int) error
	GetService(serviceName string) (string, error)
	Close() error
}

type MsRegisterOption struct {
	Endpoints         []string      //节点
	DialTimeout       time.Duration //超时时间
	ServiceName       string
	Host              string
	Port              int
	NamespaceId       string
	TimeoutMs         uint64
	LogLevel          string
	NacosServerConfig []constant.ServerConfig
	NacosClientConfig *constant.ClientConfig
}
