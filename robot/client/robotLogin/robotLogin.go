package main

import (
	"instant-message/robot/client"
	"sync"
)

var waitGroup *sync.WaitGroup

func StartNewClient(accountId int64) {
	newClient := client.NewClient(accountId, &client.RemoteAddr{
		Ip:   "192.168.10.234",
		Port: 4001,
	})
	if nil == newClient {
		waitGroup.Done()
		return
	}

	go newClient.RecvLoop()
	go newClient.HandleMsg()
	newClient.C2LGateAddr(waitGroup)
}

func main() {
	var accountId int64 = 13398165953

	waitGroup = &sync.WaitGroup{}
	waitGroup.Add(1)

	go StartNewClient(accountId)

	waitGroup.Wait()
}
