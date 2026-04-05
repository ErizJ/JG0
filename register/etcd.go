package register

import (
	"context"
	"errors"
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
	defer cancel()
	_, err = cli.Put(ctx, option.ServiceName, fmt.Sprintf("%s:%d", option.Host, option.Port))
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
	defer cancel()
	v, err := cli.Get(ctx, option.ServiceName)
	if err != nil {
		return "", err
	}
	if len(v.Kvs) == 0 {
		return "", errors.New("key not found: " + option.ServiceName)
	}
	return string(v.Kvs[0].Value), nil
}
