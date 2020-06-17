package msgServerApp

import (
	"fmt"
	"github.com/hh8456/redisSession"
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"instant-message/im-library/consistentHash"
	"instant-message/im-library/etcd"
	"instant-message/im-library/im_net"
	"instant-message/msg-server/client"
	"net"
	"time"

	"github.com/hh8456/go-common/cache"
	"github.com/hh8456/go-common/redisObj"

	"github.com/coreos/etcd/clientv3"
	"github.com/g4zhuj/hashring"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gomodule/redigo/redis"

	"github.com/go-xorm/xorm"
)

type msgServerObj struct {
	orm                  *xorm.Engine
	listenAddrOutNet     string
	subscribeTopic       string // 其他 msg-server 往这个主题上发送消息
	hashRing             *hashring.HashRing
	chatRecord           *cache.Cache               // key 是 accountid( groupid ) + ":" + 客户端时间戳, value 是聊天记录
	rdsRegister          *redisSession.RedisSession // 用户注册
	rdsLogin             *redisSession.RedisSession // 用户登录
	rdsRegisterSMS       *redisSession.RedisSession // 用户注册短信验证码
	rdsForgetPasswordSMS *redisSession.RedisSession // 用户忘记密码短信验证码
	rdsRegionRegular     *redisSession.RedisSession // 用户手机号正则匹配
	rdsLoginToken        *redisSession.RedisSession // 第三方平台用户，通过token登录
	rdsAgent             *redisSession.RedisSession // 代理与会员关系
	rdsBriefInfo         *redisSession.RedisSession
}

func NewMsgServer(dataSourceName, listenAddrOutNet string, etcdEndPoints []string,
	subscribeTopic string, ipAndWeights []string) *msgServerObj {
	orm := function.Must(xorm.NewEngine("mysql", dataSourceName)).(*xorm.Engine)
	orm.SetMaxIdleConns(5)
	orm.SetMaxOpenConns(10)
	orm.SetConnMaxLifetime(time.Minute * 14)
	function.Must(nil, orm.Ping())

	srv := &msgServerObj{orm: orm,
		listenAddrOutNet:     listenAddrOutNet,
		subscribeTopic:       subscribeTopic,
		hashRing:             consistentHash.New(ipAndWeights),
		chatRecord:           cache.New(),
		rdsRegister:          redisObj.NewSessionWithPrefix(client.RdsKeyRegisterAccount),
		rdsLogin:             redisObj.NewSessionWithPrefix(client.RdskeyLoginAccount),
		rdsRegisterSMS:       redisObj.NewSessionWithPrefix(client.RdsKeyPrefixVerificationRegisterString),
		rdsForgetPasswordSMS: redisObj.NewSessionWithPrefix(client.RdsKeyPrefixVerificationForgetPasswordString),
		rdsRegionRegular:     redisObj.NewSessionWithPrefix(client.RdsKeyPrefixRegionRegularString),
		rdsLoginToken:        redisObj.NewSessionWithPrefix(client.RdsKeyPrefixThirdPlatformLoginToken),
		rdsAgent:             redisObj.NewSessionWithPrefix(client.RdsKeyPrefixHighAgentSet),
		rdsBriefInfo:         redisObj.NewSessionWithPrefix(client.RdsKeyPrefixBriefInfo),
	}

	// XXX etcd 的代码做保留,以后可能会用 19.10.20
	//srv.etcdInit(etcdEndPoints)

	return srv
}

func (g *msgServerObj) etcdInit(etcdEndPoints []string) {
	etcdClient, err := clientv3.New(clientv3.Config{
		Endpoints:   etcdEndPoints,
		DialTimeout: 5 * time.Second,
	})

	if err != nil {
		str := fmt.Sprintf("etcd init error: %v", err)
		panic(str)
	}

	//onlineChatGroup := "onlineChatGroup"
	//// 把本地的在线群信息写入 etcd
	//go etcd.RegisterOnlineChatGroup(etcdClient, onlineChatGroup, g.subscribeTopic)

	//// 观察其他服务器的在线群信息
	//go etcd.WatchOnlineChatGroup(etcdClient, onlineChatGroup)

	// XXX 最小连接数负载均衡算法, 把本地的在线人数写入 etcd
	go etcd.RegisterOnlineUserNumber(etcdClient, etcd.OnlineUserNumberDir, g.subscribeTopic)
}

func (g *msgServerObj) Run() {

	// XXX 把 pulsar 替换为 redis, 系统稳定后删除注释 19.10.20
	//topics := pulsar_client.GetSubscribeTopices()
	//g.recvFromPulsar(topics)

	go func() {
		ticker := time.NewTicker(time.Second * 60)
		defer ticker.Stop()
		for range ticker.C {
			err := g.orm.Ping()
			if err != nil {
				log.Errorf("db ping err: %v",
					err)
			}
		}
	}()

	rdsSessPubSub := redisObj.NewSessionWithPrefix("sub") // rdsSessPubSub 的名字前缀没有任何意义
	sub := rdsSessPubSub.CreatePubSubConn()
	go func() {
		sub.Subscribe(g.subscribeTopic)
		for {
			p := sub.Receive()
			switch p.(type) {
			case redis.Message:
				g.HandleMQMessage(p.(redis.Message).Data)

			case redis.Subscription:
			}
		}
	}()

	g.listenClient(g.listenAddrOutNet)
}

func (g *msgServerObj) listenClient(addr string) {
	defer function.Catch()
	listenerPtr := im_net.CreateListener("tcp", addr, func(conn net.Conn) {
		c := im_net.CreateSocket(conn, 1<<14)
		// 先处理注册账号或者登录
		go g.clientRegOrLogin(conn, c)
	})

	for {
		err := listenerPtr.Start()
		if err != nil {
			log.Errorf("listen client connection error:%s", err.Error())
		}

		time.Sleep(time.Second)
	}
}
