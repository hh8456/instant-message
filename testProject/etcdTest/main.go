package main

import (
	"fmt"
	"instant-message/common-library/config"
	"instant-message/im-library/etcd"
	"os"
	"time"

	"github.com/coreos/etcd/clientv3"
)

func main() {
	imPublicConfig, err1 := config.ReadPublicConfig(os.Getenv("IMCONFIGPATH") + "/public.json")
	if nil != err1 {
		str := fmt.Sprintf("读取配置 public.json 文件出错:%v", err1)
		panic(str)
	}

	fmt.Printf("etcd points: %v", imPublicConfig.Etcd)
	etcdClient, err := clientv3.New(clientv3.Config{
		Endpoints:   imPublicConfig.Etcd,
		DialTimeout: 5 * time.Second,
	})

	if nil != err {
		str := fmt.Sprintf("初始化 etcd 出错: %v", err)
		panic(str)
	}

	onlineUserNumber := "onlineUserNumber"
	go etcd.RegisterOnlineUserNumberTest(etcdClient, onlineUserNumber, "etcdTopic5", 1)
	go etcd.RegisterOnlineUserNumberTest(etcdClient, onlineUserNumber, "etcdTopic2", 2)
	go etcd.RegisterOnlineUserNumberTest(etcdClient, onlineUserNumber, "etcdTopic3", 3)
	go etcd.RegisterOnlineUserNumberTest(etcdClient, onlineUserNumber, "etcdTopic4", 4)
	go etcd.RegisterOnlineUserNumberTest(etcdClient, onlineUserNumber, "etcdTopic1", 5)

	on := etcd.NewOnlineUserNumber(onlineUserNumber)
	go etcd.WatchOnlineUserNmuber(etcdClient, onlineUserNumber, on)

	time.Sleep(time.Minute)
	topic, min := on.GetMininumLoad()
	println("minium topic", topic, min)
	select {}

	fmt.Println("vim-go")
}
