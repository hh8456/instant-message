package client

import (
	"fmt"
	"instant-message/common-library/log"
	"instant-message/common-library/models"
	"instant-message/im-library/im_net"
	"sync"
)

type Client struct {
	Account  *models.Account
	Sock     *im_net.Socket
	isClosed bool
	lockSock sync.RWMutex
	msgChan  chan []byte
}

type RemoteAddr struct {
	Ip   string `json:"ip"`
	Port int16  `json:"port"`
}

var WaitingGroup *sync.WaitGroup

func NewClient(accountId int64, addr *RemoteAddr) *Client {
	remoteSocket, err := im_net.ConnectSocket("tcp", fmt.Sprintf("%s:%d", addr.Ip, addr.Port), 1024*1024)
	if err != nil {
		log.Errorf("connect server failed,ServerIp:%s:%d", addr.Ip, addr.Port)
		return nil
	} else {
		fmt.Println("connect server success,ServerIp:%s:%d", addr.Ip, addr.Port)
	}

	return &Client{
		Account: &models.Account{
			AccountId: accountId,
		},
		Sock:     remoteSocket,
		isClosed: false,
		msgChan:  make(chan []byte),
	}
}
