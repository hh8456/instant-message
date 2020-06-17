package main

import (
	"fmt"
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"instant-message/robot/client"
	"os"
	"sync"
)

var waitGroup *sync.WaitGroup

func init() {
	logDir := os.Getenv("IMLOGPATH")
	logDir += "/robot"
	exists, err := function.PathExists(logDir)
	if err != nil {
		str := fmt.Sprintf("get dir err: %v\n", err)
		panic(str)
	}
	if !exists {
		os.Mkdir(logDir, os.ModePerm)
	}
	handler := log.NewDefaultRotatingFileAtDayHandler(logDir+"/robot", 1024*1024*50)
	log.SetHandler(handler)
	log.SetLevel(log.LevelTrace)
}

func StartNewClient(accountId int64) {
	newClient := client.NewClient(accountId, &client.RemoteAddr{
		Ip:   "192.168.10.234",
		Port: 5001,
	})
	if nil == newClient {
		fmt.Println("Connect 2 Server Failed")
		waitGroup.Done()
		return
	}

	/* tcp数据接收协程 */
	go newClient.RecvLoop()
	/* 报文数据处理协程 */
	go newClient.HandleMsg()
	newClient.C2SLogin()

	/* 此处添加测试代码 */

	/* 测试添加好友流程，（帐号设置为下一行==前面的帐号） */
	if 18280048700 == newClient.Account.AccountId {
		newClient.C2SAddFriend(13398165953)
	}
}

func main() {
	//accountId, err := strconv.ParseInt(os.Args[1], 10, 64)
	//if nil != err {
	//	fmt.Println("robot命令行参数，请输入AccountId")
	//	return
	//}

	var accountId int64 = 13398165953

	waitGroup = &sync.WaitGroup{}
	waitGroup.Add(1)

	go StartNewClient(accountId)

	waitGroup.Wait()
}
