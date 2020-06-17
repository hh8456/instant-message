package main

import (
	"fmt"
	"instant-message/common-library/config"
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"instant-message/common-library/snowflake"
	"instant-message/im-library/redisPubSub"
	"instant-message/msg-server/msgServerApp"
	"instant-message/msg-server/onlineChatGroupManager"
	"instant-message/msg-server/otherTerminal"
	"os"

	"github.com/hh8456/go-common/redisObj"
)

func init() {
	// 建立日志文件
	logDir := os.Getenv("IMLOGPATH")
	exits, err := function.PathExists(logDir)
	if err != nil {
		str := fmt.Sprintf("get dir error, %v", err)
		log.Errorf(str)
		panic(str)
	}

	if !exits {
		os.Mkdir(logDir, os.ModePerm)
	}

	handler := log.NewDefaultRotatingFileAtDayHandler(logDir+"/msg-server", 1024*1024*50)
	log.SetHandler(handler)
	log.SetLevel(log.LevelTrace)
}

func deferFunc() {
	log.Close()
	function.Catch()
}

func main() {
	defer deferFunc()

	imPublicConfig, err1 := config.ReadPublicConfig(os.Getenv("IMCONFIGPATH") + "/public.json")
	if nil != err1 {
		log.Errorf("读取配置 public.json 文件出错:%v", err1)
		return
	}

	imLocalConfig, err2 := config.ReadLocalConfig(os.Getenv("IMCONFIGPATH") + "/local.json")
	if nil != err2 {
		log.Errorf("读取配置 local.json 文件出错:%v", err2)
		return
	}

	msgSrvIpListCfg, err3 := config.ReadMsgSrvIpListConfig(os.Getenv("IMCONFIGPATH") + "/msg.server.ip.list.json")
	if nil != err3 {
		log.Errorf("读取配置 msg.server.ip.list.json 文件出错:%v", err3)
		return
	}

	ipAndWeights := msgSrvIpListCfg.GetIpAndWeight()

	redisObj.Init(imPublicConfig.Redis.Addr, imPublicConfig.Redis.Pwd)

	redisPubSub.Init(ipAndWeights)
	onlineChatGroupManager.Init(imLocalConfig.Local.Topic)
	otherTerminal.Init()

	snowflake.SetMachineId(1)

	s := msgServerApp.NewMsgServer(imPublicConfig.Mysql, imLocalConfig.Local.Listen,
		imPublicConfig.Etcd, imLocalConfig.Local.Topic, ipAndWeights)

	s.Run()
}
