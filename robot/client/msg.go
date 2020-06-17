package client

import (
	"encoding/binary"
	"fmt"
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"instant-message/common-library/packet"
	"instant-message/common-library/proto/msg_id"
	"io"
	"runtime"
)

var (
	handleMsgMap = make(map[msg_id.NetMsgId]func(*Client, []byte))
)

func init() {
	registerMessageHandle(msg_id.NetMsgId_L2c_gate_addr, L2CGateAddr)
}

func registerMessageHandle(id msg_id.NetMsgId, handleMsg func(*Client, []byte)) {
	handleMsgMap[id] = handleMsg
}

func getMessageHandle(id msg_id.NetMsgId) func(*Client, []byte) {
	return handleMsgMap[id]
}

func (c *Client) RecvLoop() {
	defer function.Catch()
	for {
		bytes, err := c.Sock.ReadOne()
		if nil != err {
			if io.EOF != err {
				log.Errorf("client.RecvLoop read error:%v,accountId:%d\n", err, c.Account.AccountId)
				continue
			}

			if !c.isSocketClosed() {
				c.closeSocket()
				break
			}
		}

		fmt.Printf("Recv From Server Message16:[")
		for index := 0; index < len(bytes); index++ {
			fmt.Printf("%x ", bytes[index])
		}
		fmt.Printf("]\n")

		c.msgChan <- bytes
	}
}

func (c *Client) HandleMsg() {
	for {
		msg := <-c.msgChan
		if packet.PacketHeaderSize > len(msg) {
			log.Error(runtime.Caller(0))
			log.Error("received error msg = %v, len = %v", msg, len(msg))
			continue
		}

		msgId := binary.BigEndian.Uint32(msg)
		handleMsg := getMessageHandle(msg_id.NetMsgId(msgId))
		if nil == handleMsg {
			log.Errorf("no function register with msgId:%d\n", msgId)
			return
		}

		handleMsg(c, msg)
	}
}

func (c *Client) isSocketClosed() bool {
	c.lockSock.RLock()
	defer c.lockSock.RUnlock()
	return c.isClosed
}

func (c *Client) closeSocket() {
	c.lockSock.Lock()
	defer c.lockSock.Unlock()
	c.Sock.Close()
}
