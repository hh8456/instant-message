package client

import (
	"fmt"
	"instant-message/common-library/log"
	"instant-message/common-library/proto/msg_id"
	"instant-message/common-library/proto/msg_struct"
	"runtime"
	"sync"
	"time"
)

func (c *Client) C2LGateAddr(waitingGroup *sync.WaitGroup) {
	WaitingGroup = waitingGroup
	if err := c.Sock.SendPbMsg(msg_id.NetMsgId_C2l_gate_addr, &msg_struct.C2LGateAddr{
		AccountId: c.Account.AccountId,
	}); nil != err {
		log.Errorf("发送登录消息失败")
	}
}

func (c *Client) C2SLogin() {
	if err := c.Sock.SendPbMsg(msg_id.NetMsgId_C2SLogin, &msg_struct.C2SLogin{
		AccountId: c.Account.AccountId,
	}); nil != err {
		log.Errorf("Send Login 2 Gate Failed,err:%v", err)
	}
}

func (c *Client) C2SHeartBeat() {
	fmt.Println(runtime.Caller(0))
	for {
		if err := c.Sock.SendPbMsg(msg_id.NetMsgId_C2SHeartbeat, &msg_struct.C2SHeartbeat{
			TimeStamp: time.Now().Unix(),
		}); nil != err {
			log.Errorf("Send HeartBeat 2 Gate Failed,err:%v", err)
		}

		time.Sleep(time.Second * 20)
	}
}

func (c *Client) C2SAddFriend(addFriendId int64) {
	if err := c.Sock.SendPbMsg(msg_id.NetMsgId_C2SAddFriend, &msg_struct.C2SAddFriend{
		SenderId:   c.Account.AccountId,
		ReceiverId: addFriendId,
	}); nil != err {
		fmt.Println("Send C2SAddFriend Failed,err:%v", err)
		log.Errorf("Send C2SAddFriend Failed,err:%v", err)
	}
}

func (c *Client) C2SAgreeAddFriend(friendReqSenderId int64) {
	fmt.Println(runtime.Caller(0))

	if err := c.Sock.SendPbMsg(msg_id.NetMsgId_C2SAgreeBecomeFriend, &msg_struct.C2SAgreeBecomeFriend{
		SenderId:   friendReqSenderId,
		ReceiverId: c.Account.AccountId,
	}); nil != err {
		fmt.Println("Send C2SAgreeAddFriend Failed,err:%v", err)
		log.Errorf("Send C2SAgreeAddFriend Failed,err:%v", err)
	}
}

func (c *Client) C2SCreateChatGroup(groupName, groupPic string, inviteeFriends []int64) {
	if err := c.Sock.SendPbMsg(msg_id.NetMsgId_C2SCreateChatGroup, &msg_struct.C2SCreateChatGroup{}); nil != err {
		fmt.Println("Send C2SCreateChatGroup Failed,err:%v", err)
		log.Errorf("Send C2SCreateChatGroup Failed,err:%v", err)
	}
}

func (c *Client) C2SPrivateChat(receiverId int64, sendText, msgType string) {
	fmt.Println(runtime.Caller(0))

	if err := c.Sock.SendPbMsg(msg_id.NetMsgId_C2SPrivateChat, &msg_struct.C2SPrivateChat{
		ChatMsg: &msg_struct.ChatMessage{
			SenderId:   c.Account.AccountId,
			ReceiverId: receiverId,
			Text:       sendText,
			MsgType:    msgType,
		},
		CliTimeStamp: time.Now().Unix() / 1e3,
	}); nil != err {
		fmt.Println("Send C2SAgreeAddFriend Failed,err:%v", err)
		log.Errorf("Send C2SAgreeAddFriend Failed,err:%v", err)
	}
}
