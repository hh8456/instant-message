package client

import (
	"fmt"
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"instant-message/common-library/packet"
	"instant-message/common-library/proto/msg_id"
	"instant-message/common-library/proto/msg_struct"
	"runtime"
)

func init() {
	registerMessageHandle(msg_id.NetMsgId_S2CErrorCode, S2CErrorCode)
	registerMessageHandle(msg_id.NetMsgId_S2CLogin, S2CLogin)
	registerMessageHandle(msg_id.NetMsgId_S2CHeartbeat, S2CHeartBeat)
	/* 加好友 */
	registerMessageHandle(msg_id.NetMsgId_S2CAddFriend, S2CAddFriend)
	registerMessageHandle(msg_id.NetMsgId_S2CRecvAddFriend, S2CRecvAddFriend)
	registerMessageHandle(msg_id.NetMsgId_S2CAgreeBecomeFriend, S2CAgreeBecomeFriend)
	registerMessageHandle(msg_id.NetMsgId_S2CRecvAgreeBecomeFriend, S2CRecvAgreeBecomeFriend)
	/* 私聊 */
	registerMessageHandle(msg_id.NetMsgId_S2CRecvPrivateChat, S2CRecvPrivateChat)
	registerMessageHandle(msg_id.NetMsgId_S2CGetOfflinePrivateChatMsg, S2CGetOfflinePrivateChatMsg)
	/* 群管理 */
	registerMessageHandle(msg_id.NetMsgId_S2CCreateChatGroup, S2CCreateChatGroup)
	registerMessageHandle(msg_id.NetMsgId_S2CRecvCreateChatGroup, S2CRecvCreateChatGroup)
	registerMessageHandle(msg_id.NetMsgId_S2CRecvReqJoinChatGroup, S2CRecvReqJoinChatGroup)
	registerMessageHandle(msg_id.NetMsgId_S2CSomeOneReqJoinChatGroup, S2CSomeOneReqJoinChatGroup)
	registerMessageHandle(msg_id.NetMsgId_S2CManagerAgreeSomeOneReqJoinChatGroup, S2CManagerAgreeSomeOneReqJoinChatGroup)
	registerMessageHandle(msg_id.NetMsgId_S2CManagerAgreeJoinChatGroup, S2CManagerAgreeJoinChatGroup)
	registerMessageHandle(msg_id.NetMsgId_S2CManagerRefuseJoinChatGroup, S2CManagerRefuseJoinChatGroup)
	registerMessageHandle(msg_id.NetMsgId_S2CChatGroupInfo, S2CChatGroupInfo)
	registerMessageHandle(msg_id.NetMsgId_S2CInviteYouJoinChatGroup, S2CInviteYouJoinChatGroup)
	registerMessageHandle(msg_id.NetMsgId_S2CSomeOneCancelChatGroup, S2CSomeOneCancelChatGroup)
	registerMessageHandle(msg_id.NetMsgId_S2CUnChatGroup, S2CUnChatGroup)
	registerMessageHandle(msg_id.NetMsgId_S2CManagerKickSomeOne, S2CManagerKickSomeOne)
	registerMessageHandle(msg_id.NetMsgId_S2CKickedByChatGroupManager, S2CKickedByChatGroupManager)
	/* 群聊 */
	registerMessageHandle(msg_id.NetMsgId_S2CGroupChat, S2CGroupChat)
}

func S2CErrorCode(c *Client, msg []byte) {
	fmt.Println(runtime.Caller(0))

	s2CErrorCode := &msg_struct.S2CErrorCode{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], s2CErrorCode, "msg_struct.S2CErrorCode") {
		log.Errorf("unmarsharshal S2CErrorCode failed,msg:%v", msg)
		return
	}

	fmt.Println("RECV S2CERRORCODE:", s2CErrorCode.ErrCode)
}

func L2CGateAddr(c *Client, msg []byte) {
	fmt.Println(runtime.Caller(0))

	l2CGateAddr := &msg_struct.L2CGateAddr{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], l2CGateAddr, "msg_struct.L2CGateAddr") {
		log.Errorf("unmarsharshal L2CGateAddr failed,msg:%v", msg)
		return
	}

	fmt.Println(l2CGateAddr.Ip)
	WaitingGroup.Done()
}

func S2CLogin(c *Client, msg []byte) {
	fmt.Println(runtime.Caller(0))
	fmt.Println("S2CLogin, Ip =", c.Sock.RemoteAddr())

	s2CLogin := &msg_struct.S2CLogin{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], s2CLogin, "msg_struct.S2CLogin") {
		log.Errorf("unmarsharshal S2CLogin failed,msg:%v", msg)
		return
	}

	//fmt.Println("FriendReq Offline")
	//for _, friendReq := range s2CLogin.FriendReqs {
	//	fmt.Println(friendReq.SenderId)
	//	fmt.Println(friendReq.ReqDateTime)
	//
	//	go c.C2SAgreeAddFriend(friendReq.SenderId)
	//}

	go c.C2SHeartBeat()
}

func S2CHeartBeat(c *Client, msg []byte) {
	fmt.Println(runtime.Caller(0))

	s2CHeartbeat := &msg_struct.S2CHeartbeat{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], s2CHeartbeat, "msg_struct.S2CHeartbeat") {
		log.Errorf("unmarsharshal S2CHeartbeat failed,msg:%v", msg)
		return
	}
}

func S2CAddFriend(c *Client, msg []byte) {
	fmt.Println(runtime.Caller(0))

	s2CAddFriend := &msg_struct.S2CAddFriend{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], s2CAddFriend, "msg_struct.S2CAddFriend") {
		log.Errorf("unmarsharshal S2CAddFriend failed,msg:%v", msg)
		return
	}

	fmt.Println(s2CAddFriend.SenderId)
	fmt.Println(s2CAddFriend.ReceiverId)
	fmt.Println(s2CAddFriend.SrvTimeStamp)
}

func S2CRecvAddFriend(c *Client, msg []byte) {
	fmt.Println(runtime.Caller(0))

	s2CRecvAddFriend := &msg_struct.S2CRecvAddFriend{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], s2CRecvAddFriend, "msg_struct.S2CRecvAddFriend") {
		log.Errorf("unmarsharshal S2CRecvAddFriend failed,msg:%v", msg)
		return
	}

	fmt.Println(s2CRecvAddFriend.SenderId)
	fmt.Println(s2CRecvAddFriend.ReceiverId)
	fmt.Println(s2CRecvAddFriend.SrvTimeStamp)

	go c.C2SAgreeAddFriend(s2CRecvAddFriend.SenderId)
}

func S2CAgreeBecomeFriend(c *Client, msg []byte) {
	s2CAgreeBecomeFriend := &msg_struct.S2CAgreeBecomeFriend{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], s2CAgreeBecomeFriend, "msg_struct.S2CAgreeBecomeFriend") {
		log.Errorf("unmarsharshal S2CAgreeBecomeFriend failed,msg:%v", msg)
		return
	}

	fmt.Println(s2CAgreeBecomeFriend.SenderId)
}

func S2CRecvAgreeBecomeFriend(c *Client, msg []byte) {
	s2CRecvAgreeBecomeFriend := &msg_struct.S2CRecvAgreeBecomeFriend{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], s2CRecvAgreeBecomeFriend, "msg_struct.S2CRecvAgreeBecomeFriend") {
		log.Errorf("unmarsharshal S2CRecvAgreeBecomeFriend failed,msg:%v", msg)
		return
	}

	fmt.Println(s2CRecvAgreeBecomeFriend.SenderId)
	fmt.Println(s2CRecvAgreeBecomeFriend.ReceiverId)
}

func S2CRecvPrivateChat(c *Client, msg []byte) {
	s2CRecvPrivateChat := &msg_struct.S2CRecvPrivateChat{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], s2CRecvPrivateChat, "msg_struct.s2CRecvPrivateChat") {
		log.Errorf("unmarsharshal S2CRecvPrivateChat failed,msg:%v", msg)
		return
	}

	fmt.Printf("%d在%v发来消息:%s", s2CRecvPrivateChat.ChatMsg.SenderId,
		s2CRecvPrivateChat.ChatMsg.SrvTimeStamp, s2CRecvPrivateChat.ChatMsg.Text)
}

func S2CGetOfflinePrivateChatMsg(c *Client, msg []byte) {
	s2CGetOfflinePrivateChatMsg := &msg_struct.S2CGetOfflinePrivateChatMsg{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], s2CGetOfflinePrivateChatMsg, "msg_struct.S2CGetOfflinePrivateChatMsg") {
		log.Errorf("unmarsharshal S2CGetOfflinePrivateChatMsg failed,msg:%v", msg)
		return
	}

	for _, chatMsg := range s2CGetOfflinePrivateChatMsg.Msgs {
		fmt.Printf("%d在%v发来消息:%s", chatMsg.SenderId, chatMsg.SrvTimeStamp, chatMsg.Text)
	}
}

func S2CCreateChatGroup(c *Client, msg []byte) {
	s2CCreateChatGroup := &msg_struct.S2CCreateChatGroup{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], s2CCreateChatGroup, "msg_struct.S2CCreateChatGroup") {
		log.Errorf("unmarsharshal S2CCreateChatGroup failed,msg:%v", msg)
		return
	}

}

func S2CRecvCreateChatGroup(c *Client, msg []byte) {
	s2CRecvCreateChatGroup := &msg_struct.S2CRecvCreateChatGroup{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], s2CRecvCreateChatGroup, "msg_struct.S2CRecvCreateChatGroup") {
		log.Errorf("unmarsharshal S2CRecvCreateChatGroup failed,msg:%v", msg)
		return
	}
}

func S2CRecvReqJoinChatGroup(c *Client, msg []byte) {
	s2CRecvReqJoinChatGroup := &msg_struct.S2CRecvReqJoinChatGroup{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], s2CRecvReqJoinChatGroup, "msg_struct.S2CRecvReqJoinChatGroup") {
		log.Errorf("unmarsharshal S2CRecvReqJoinChatGroup failed,msg:%v", msg)
		return
	}
}

func S2CSomeOneReqJoinChatGroup(c *Client, msg []byte) {
	s2CRecvReqJoinChatGroup := &msg_struct.S2CSomeOneReqJoinChatGroup{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], s2CRecvReqJoinChatGroup, "msg_struct.S2CRecvReqJoinChatGroup") {
		log.Errorf("unmarsharshal S2CRecvReqJoinChatGroup failed,msg:%v", msg)
		return
	}
}

func S2CManagerAgreeSomeOneReqJoinChatGroup(c *Client, msg []byte) {
	s2CManagerAgreeSomeOneReqJoinChatGroup := &msg_struct.S2CManagerAgreeSomeOneReqJoinChatGroup{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], s2CManagerAgreeSomeOneReqJoinChatGroup, "msg_struct.S2CManagerAgreeSomeOneReqJoinChatGroup") {
		log.Errorf("unmarsharshal S2CManagerAgreeSomeOneReqJoinChatGroup failed,msg:%v", msg)
		return
	}
}

func S2CManagerAgreeJoinChatGroup(c *Client, msg []byte) {
	s2CManagerAgreeJoinChatGroup := &msg_struct.S2CManagerAgreeJoinChatGroup{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], s2CManagerAgreeJoinChatGroup, "msg_struct.S2CManagerAgreeJoinChatGroup") {
		log.Errorf("unmarsharshal S2CManagerAgreeJoinChatGroup failed,msg:%v", msg)
		return
	}
}

func S2CManagerRefuseJoinChatGroup(c *Client, msg []byte) {
	s2CManagerRefuseJoinChatGroup := &msg_struct.S2CManagerRefuseJoinChatGroup{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], s2CManagerRefuseJoinChatGroup, "msg_struct.S2CManagerRefuseJoinChatGroup") {
		log.Errorf("unmarsharshal S2CManagerRefuseJoinChatGroup failed,msg:%v", msg)
		return
	}
}

func S2CChatGroupInfo(c *Client, msg []byte) {
	s2CChatGroupInfo := &msg_struct.S2CChatGroupInfo{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], s2CChatGroupInfo, "msg_struct.S2CChatGroupInfo") {
		log.Errorf("unmarsharshal S2CChatGroupInfo failed,msg:%v", msg)
		return
	}
}

func S2CInviteYouJoinChatGroup(c *Client, msg []byte) {
	s2CInviteYouJoinChatGroup := &msg_struct.S2CInviteYouJoinChatGroup{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], s2CInviteYouJoinChatGroup, "msg_struct.S2CInviteYouJoinChatGroup") {
		log.Errorf("unmarsharshal S2CInviteYouJoinChatGroup failed,msg:%v", msg)
		return
	}
}

func S2CSomeOneCancelChatGroup(c *Client, msg []byte) {
	s2CSomeOneCancelChatGroup := &msg_struct.S2CSomeOneCancelChatGroup{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], s2CSomeOneCancelChatGroup, "msg_struct.S2CSomeOneCancelChatGroup") {
		log.Errorf("unmarsharshal S2CSomeOneCancelChatGroup failed,msg:%v", msg)
		return
	}
}

func S2CUnChatGroup(c *Client, msg []byte) {
	s2CUnChatGroup := &msg_struct.S2CUnChatGroup{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], s2CUnChatGroup, "msg_struct.S2CUnChatGroup") {
		log.Errorf("unmarsharshal S2CUnChatGroup failed,msg:%v", msg)
		return
	}
}

func S2CManagerKickSomeOne(c *Client, msg []byte) {
	s2CManagerKickSomeOne := &msg_struct.S2CManagerKickSomeOne{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], s2CManagerKickSomeOne, "msg_struct.S2CManagerKickSomeOne") {
		log.Errorf("unmarsharshal S2CManagerKickSomeOne failed,msg:%v", msg)
		return
	}
}

func S2CKickedByChatGroupManager(c *Client, msg []byte) {
	s2CKickedByChatGroupManager := &msg_struct.S2CKickedByChatGroupManager{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], s2CKickedByChatGroupManager, "msg_struct.S2CKickedByChatGroupManager") {
		log.Errorf("unmarsharshal S2CKickedByChatGroupManager failed,msg:%v", msg)
		return
	}
}

func S2CGroupChat(c *Client, msg []byte) {
	s2CGroupChat := &msg_struct.S2CGroupChat{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], s2CGroupChat, "msg_struct.S2CGroupChat") {
		log.Errorf("unmarsharshal S2CGroupChat failed,msg:%v", msg)
		return
	}
}
