package main

import (
	"fmt"
	"instant-message/common-library/config"
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"instant-message/login-server/login_app"
	"os"
)

func init() {
	// 建立日志文件
	logDir := os.Getenv("IMLOGPATH")
	exits, err := function.PathExists(logDir)
	if err != nil {
		str := fmt.Sprintf("get dir error, %v", err)
		panic(str)
	}

	if !exits {
		os.Mkdir(logDir, os.ModePerm)
	}

	handler := log.NewDefaultRotatingFileAtDayHandler(logDir+"/login", 1024*1024*50)
	log.SetHandler(handler)
	log.SetLevel(log.LevelTrace)
}

func deferFunc() {
	log.Close()
	function.Catch()
}

func main() {
	defer deferFunc()

	//dbSrcName := "root:84565310" + "@tcp(192.168.6.132:3306)/im?charset=utf8mb4"
	//db := login_app.NewLoginDB(dbSrcName)
	//db.LoadAccountIDTableConcurrent()

	msgSrvIpListCfg, err := config.ReadMsgSrvIpListConfig(os.Getenv("IMCONFIGPATH") + "/msg.server.ip.list.json")

	if nil != err {
		log.Errorf("读取配置 msg.server.ip.list.json 文件出错:%v", err)
		return
	}

	imPublicConfig, err1 := config.ReadPublicConfig(os.Getenv("IMCONFIGPATH") + "/public.json")
	if nil != err1 {
		log.Errorf("读取配置 public.json 文件出错:%v", err1)
		return
	}

	ipAndWeights := msgSrvIpListCfg.GetIpAndWeight()

	login_app.Init("0.0.0.0:4001", ipAndWeights, imPublicConfig.Etcd)

	login_app.Start()
}
