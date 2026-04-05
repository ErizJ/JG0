package register

import (
	"fmt"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

type MsNacos struct {
	client naming_client.INamingClient
}

func MsNacosDefault() *MsNacos {
	nacos := &MsNacos{}
	options := MsRegisterOption{}
	options.NacosClientConfig = constant.NewClientConfig(
		constant.WithNamespaceId(""), //When namespace is public, fill in the blank string here.
		constant.WithTimeoutMs(5000),
		constant.WithNotLoadCacheAtStart(true),
		constant.WithLogDir("/tmp/nacos/log"),
		constant.WithCacheDir("/tmp/nacos/cache"),
		constant.WithLogLevel("debug"),
	)
	options.NacosServerConfig = []constant.ServerConfig{
		*constant.NewServerConfig(
			"127.0.0.1",
			8848,
			constant.WithScheme("http"),
			constant.WithContextPath("/nacos"),
		),
	}
	nacos.CreateClient(options)
	return nacos
}

func (m *MsNacos) CreateClient(option MsRegisterOption) error {
	namingClient, err := clients.NewNamingClient(
		vo.NacosClientParam{
			ClientConfig:  option.NacosClientConfig,
			ServerConfigs: option.NacosServerConfig,
		},
	)
	if err != nil {
		return err
	}
	m.client = namingClient
	return nil
}

func (m *MsNacos) RegisterService(serviceName string, host string, port int) error {
	_, err := m.client.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          host,
		Port:        uint64(port),
		ServiceName: serviceName,
		Weight:      10,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   true,
		Metadata:    map[string]string{"idc": "shanghai"},
		//ClusterName: "cluster-a", // default value is DEFAULT
		//GroupName:   "group-a",   // default value is DEFAULT_GROUP
	})
	return err
}

func (m *MsNacos) GetService(serviceName string) (string, error) {
	instance, err := m.client.SelectOneHealthyInstance(vo.SelectOneHealthInstanceParam{
		ServiceName: serviceName,
		//GroupName:   "group-a",             // default value is DEFAULT_GROUP
		//Clusters:    []string{"cluster-a"}, // default value is DEFAULT
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d", instance.Ip, instance.Port), nil
}

func (m *MsNacos) Close() error {
	m.client.CloseClient()
	return nil
}
