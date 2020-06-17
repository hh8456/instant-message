package login_app

import (
	"fmt"
	"instant-message/common-library/function"
	"instant-message/im-library/consistentHash"
	"instant-message/im-library/etcd"
	"instant-message/im-library/im_net"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/g4zhuj/hashring"
)

var (
	defApp *loginApp
)

type loginApp struct {
	listenAddrOutNet string
	listenerPtr      *im_net.Listener
	hashRing         *hashring.HashRing
	onlineUserNumber *etcd.OnlineUserNumber
}

func newLoginApp(listenAddrOutNet string,
	ipAndWeights []string, on *etcd.OnlineUserNumber) *loginApp {
	app := &loginApp{
		listenAddrOutNet: listenAddrOutNet,
		hashRing:         consistentHash.New(ipAndWeights),
		onlineUserNumber: on,
	}

	return app
}

func Init(listenAddrOutNet string,
	ipAndWeights []string, etcdEndPoints []string) {
	if defApp != nil {
		return
	}

	// XXX 最小连接数负载均衡算法
	//on := etcdInit(etcdEndPoints)
	//defApp = newLoginApp(listenAddrOutNet, ipAndWeights, on)

	defApp = newLoginApp(listenAddrOutNet, ipAndWeights, nil)
}

func etcdInit(etcdEndPoints []string) *etcd.OnlineUserNumber {
	etcdClient, err := clientv3.New(clientv3.Config{
		Endpoints:   etcdEndPoints,
		DialTimeout: 5 * time.Second,
	})

	if err != nil {
		str := fmt.Sprintf("etcd init error: %v", err)
		panic(str)
	}

	dir := etcd.OnlineUserNumberDir
	on := etcd.NewOnlineUserNumber(dir)
	go etcd.WatchOnlineUserNmuber(etcdClient, dir, on)

	return on
}

func Start() {
	defApp.start()
}

func (l *loginApp) start() {
	defer func() {
		function.Catch()
	}()

	l.listenClientConnection(l.listenAddrOutNet)
}
