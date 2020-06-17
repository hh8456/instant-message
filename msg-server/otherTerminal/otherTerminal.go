package otherTerminal

import (
	"instant-message/common-library/proto/msg_id"
	"instant-message/common-library/proto/msg_struct"
	"instant-message/msg-server/client"
	"instant-message/msg-server/clientManager"
	"instant-message/msg-server/eventmanager"
)

const systemName = "otherTerminal"

func Init() {
	subscribeEvent()
}

func subscribeEvent() {
	eventmanager.Subscribe(systemName, eventmanager.PrivateChatEvent, onPrivateChatEvent)                               // 用户发送私聊消息
	eventmanager.Subscribe(systemName, eventmanager.AgreeBecomeFriendEvent, onAgreeBecomeFriendEvent)                   // 用户同意了其他人的好友请求
	eventmanager.Subscribe(systemName, eventmanager.ReadFriendReqEvent, onReadFriendReqEvent)                           // 用户拒绝了其他人的好友请求
	eventmanager.Subscribe(systemName, eventmanager.C2SLastMsgTimeStampEvent, onC2SLastMsgTimeStampEvent)               // 客户端上报收到的最后一条私聊时间戳
	eventmanager.Subscribe(systemName, eventmanager.DeleteFriendEvent, onDeleteFriendEvent)                             // 删除好友
	eventmanager.Subscribe(systemName, eventmanager.ConfirmDeleteFriendEvent, onConfirmDeleteFriend)                    // A 删除 B, B 发送 C2SConfirmDeleteFriend 确认删除 A
	eventmanager.Subscribe(systemName, eventmanager.C2SAddFriendBlackListEvent, onC2SAddFriendBlackList)                // 好友拉入黑名单
	eventmanager.Subscribe(systemName, eventmanager.S2CMoveOutFriendBlackListEvent, onS2CMoveOutFriendBlackListEvent)   // 好友移出黑名单
	eventmanager.Subscribe(systemName, eventmanager.S2CSetFriendRemarkNameEvent, onS2CSetFriendRemarkNameEvent)         // 好友打备注
	eventmanager.Subscribe(systemName, eventmanager.PrivateChatWithDrawMessageEvent, onPrivateChatWithDrawMessageEvent) // 用户撤回私聊消息
}

// 向另外一种终端登录的 client 转发私聊消息; 比如用户 a 同时登录了 APP 和 WEB 端 IM
// a 在 APP 发送给别人的私聊消息, 在 WEB 端也能看见
func onPrivateChatEvent(eventArg *eventmanager.EventArg) {
	userData := eventArg.GetUserData("client")
	chatData := eventArg.GetUserData("pbdata")
	cli := userData.(*client.Client)
	chatMsg := chatData.(*msg_struct.ChatMessage)

	clientManager.SendPbMsgToOtherTerminal(cli, msg_id.NetMsgId_S2COtherTerminalPrivateChat, chatMsg)
}

// a 在 APP 同意别人好友申请, 在 WEB 端也能看见成为好友
func onAgreeBecomeFriendEvent(eventArg *eventmanager.EventArg) {
	userData := eventArg.GetUserData("client")
	pbData := eventArg.GetUserData("pbdata")
	cli := userData.(*client.Client)
	pbMsg := pbData.(*msg_struct.S2CAgreeBecomeFriend)

	p := &msg_struct.S2COtherTerminalAgreeBecomeFriend{
		SenderId:       pbMsg.SenderId,
		SenderNickName: pbMsg.SenderNickName,
	}

	otherTerminalClient := clientManager.GetOtherTerminalClient(cli)
	if otherTerminalClient != nil {
		otherTerminalClient.BeFriend(pbMsg.SenderId)
		otherTerminalClient.UpdateContactInfoInMemory()
		otherTerminalClient.SendPbMsg(msg_id.NetMsgId_S2COtherTerminalAgreeBecomeFriend, p)
	}
}

// 拒绝好友请求以后,转告给其他端
func onReadFriendReqEvent(eventArg *eventmanager.EventArg) {
	userData := eventArg.GetUserData("client")
	pbData := eventArg.GetUserData("pbdata")
	cli := userData.(*client.Client)
	pbMsg := pbData.(*msg_struct.S2CReadFriendReq)

	clientManager.SendPbMsgToOtherTerminal(cli, msg_id.NetMsgId_S2CReadFriendReq, pbMsg)
}

func onC2SLastMsgTimeStampEvent(eventArg *eventmanager.EventArg) {
	userData := eventArg.GetUserData("client")
	lastMsgTimeStamp := eventArg.GetInt64("lastMsgTimeStamp")
	cli := userData.(*client.Client)
	otherTerminalClient := clientManager.GetOtherTerminalClient(cli)
	if otherTerminalClient != nil {
		otherTerminalClient.SetLastMsgTimestamp(lastMsgTimeStamp)
	}
}

// 删除好友以后,转告给其他端
func onDeleteFriendEvent(eventArg *eventmanager.EventArg) {
	userData := eventArg.GetUserData("client")
	pbMsg := eventArg.GetUserData("pbdata").(*msg_struct.S2CRecvDeleteFriend)
	cli := userData.(*client.Client)
	clientManager.SendPbMsgToOtherTerminal(cli, msg_id.NetMsgId_S2CRecvDeleteFriend, pbMsg)
}

func onConfirmDeleteFriend(eventArg *eventmanager.EventArg) {
	userData := eventArg.GetUserData("client")
	pbMsg := eventArg.GetUserData("pbdata").(*msg_struct.S2CConfirmDeleteFriend)
	cli := userData.(*client.Client)

	otherTerminalClient := clientManager.GetOtherTerminalClient(cli)
	if otherTerminalClient != nil {
		if otherTerminalClient.DeleteFriend(pbMsg.FriendId) {
			otherTerminalClient.SendPbMsg(msg_id.NetMsgId_S2CConfirmDeleteFriend, pbMsg)
		}
	}
}

func onC2SAddFriendBlackList(eventArg *eventmanager.EventArg) {
	userData := eventArg.GetUserData("client")
	p := eventArg.GetUserData("pbdata").(*msg_struct.S2CAddBlackList)
	cli := userData.(*client.Client)

	otherTerminalClient := clientManager.GetOtherTerminalClient(cli)
	if otherTerminalClient != nil {
		/* 校验两者是否为好友关系 */
		if !otherTerminalClient.IsFriend(p.FriendId) {
			return
		}

		/* 校验好友是否已被拉入黑名单 */
		if !otherTerminalClient.IsBlackList(p.FriendId) {
			return
		}

		/* 好友拉入黑名单，存入内存 */
		otherTerminalClient.BeBlackList(p.FriendId)
		otherTerminalClient.UpdateBlackListInfoInMemory()
		otherTerminalClient.SendPbMsg(msg_id.NetMsgId_S2CAddFriendBlackList, p)
	}
}

func onS2CMoveOutFriendBlackListEvent(eventArg *eventmanager.EventArg) {
	userData := eventArg.GetUserData("client")
	p := eventArg.GetUserData("pbdata").(*msg_struct.S2CMoveOutFriendBlackList)
	cli := userData.(*client.Client)

	otherTerminalClient := clientManager.GetOtherTerminalClient(cli)
	if otherTerminalClient != nil {
		/* 校验两者是否为好友关系 */
		if !otherTerminalClient.IsFriend(p.FriendId) {
			return
		}

		if otherTerminalClient.IsBlackList(p.FriendId) {
			return
		}

		/* 好友移出黑名单，存入内存 */
		otherTerminalClient.BeNotBlackList(p.FriendId)
		otherTerminalClient.UpdateBlackListInfoInMemory()
		/* 回复客户端，黑名单好友移出黑名单列表 */
		otherTerminalClient.SendPbMsg(msg_id.NetMsgId_S2CMoveOutFriendBlackList, p)
	}
}

func onS2CSetFriendRemarkNameEvent(eventArg *eventmanager.EventArg) {
	userData := eventArg.GetUserData("client")
	p := eventArg.GetUserData("pbdata").(*msg_struct.S2CSetFriendRemarkName)
	cli := userData.(*client.Client)

	otherTerminalClient := clientManager.GetOtherTerminalClient(cli)
	if otherTerminalClient != nil {
		otherTerminalClient.SetFriendRemarkName(p.FriendId, p.FriendRemarkName)
		// 更新内存即可,不需要保持 mysql 和 redis
		otherTerminalClient.UpdateContactInfoInMemory()
		otherTerminalClient.SendPbMsg(msg_id.NetMsgId_S2CSetFriendRemarkName, p)
	}
}

// 用户撤回私聊消息
func onPrivateChatWithDrawMessageEvent(eventArg *eventmanager.EventArg) {
	userData := eventArg.GetUserData("client")
	p := eventArg.GetUserData("pbdata").(*msg_struct.S2CWithdrawMessage)
	cli := userData.(*client.Client)

	clientManager.SendPbMsgToOtherTerminal(cli, msg_id.NetMsgId_S2CWithdrawMessage, p)
}
