package main

import (
	"fmt"
	"instant-message/common-library/config"
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"instant-message/wsmsg/wsAgent"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
)

var (
	msgAddr string
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

	handler := log.NewDefaultRotatingFileAtDayHandler(logDir+"/wsmsg", 1024*1024*50)
	log.SetHandler(handler)
	log.SetLevel(log.LevelTrace)
}

func main() {
	wscfg, err1 := config.ReadWSMsgAgentConfig(os.Getenv("IMCONFIGPATH") + "/wsmsg.json")
	if nil != err1 {
		log.Errorf("读取配置 wsmsg.json 文件出错:%v", err1)
		return
	}

	msgAddr = wscfg.Msgaddr

	http.HandleFunc("/wsmsg", wsPage)
	http.ListenAndServe(":23456", nil)
	fmt.Println("vim-go")
}

func wsPage(res http.ResponseWriter, req *http.Request) {
	conn, err := (&websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}).Upgrade(res, req, nil)
	if err != nil {
		http.NotFound(res, req)
		return
	}

	agent, err2 := wsAgent.New(conn, msgAddr)
	if err2 != nil {
		return
	}
	agent.Run()
}
