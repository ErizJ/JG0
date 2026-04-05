package register

import (
	"context"
	"fmt"
	client3 "go.etcd.io/etcd/client/v3"
	"time"
)

type Option struct {
	Endpoints   []string      //节点
	DialTimeout time.Duration //超时时间
	ServiceName string
	Host        string
	Port        int
}

func EtcdRegisterService(option Option) error {
	cli, err := client3.New(client3.Config{
		Endpoints:   option.Endpoints,   //节点
		DialTimeout: option.DialTimeout, //超过5秒钟连不上超时
	})
	if err != nil {
		return err
	}
	defer cli.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	_, err = cli.Put(ctx, option.ServiceName, fmt.Sprintf("%s:%d", option.Host, option.Port))
	defer cancel()
	return err
}

func GetEtcdValue(option Option) (string, error) {
	cli, err := client3.New(client3.Config{
		Endpoints:   option.Endpoints,   //节点
		DialTimeout: option.DialTimeout, //超过5秒钟连不上超时
	})
	if err != nil {
		return "", err
	}
	defer cli.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	v, err := cli.Get(ctx, option.ServiceName)
	defer cancel()
	kvs := v.Kvs
	return string(kvs[0].Value), err
}
