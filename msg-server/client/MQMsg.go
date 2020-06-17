// 处理来自 pulsar 的消息

package client

import (
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"instant-message/common-library/packet"
	"instant-message/common-library/proto/msg_id"
	"instant-message/common-library/proto/msg_struct"
	"instant-message/msg-server/eventmanager"
	"strconv"
	"time"
)

func init() {
	// B 用户收到 MQ ( 转发的 A 用户的 )私聊消息
	registerLogicCall(msg_id.NetMsgId_MQPrivateChat, MQPrivateChat)
	// B 用户收到 MQ ( 转发的 A 用户的 )请求加好友的消息
	registerLogicCall(msg_id.NetMsgId_MQAddFriend, MQAddFriend)
	// A 用户收到 MQ ( 转发的 B 用户的 )同意成为好友的消息
	registerLogicCall(msg_id.NetMsgId_MQAgreeBecomeFriend, MQAgreeBecomeFriend)
	// 删除好友消息
	registerLogicCall(msg_id.NetMsgId_MQDeleteFriend, MQDeleteFriend)
	// 其他人建立了个新群,并把 "我" 加入群内
	registerLogicCall(msg_id.NetMsgId_MQCreateChatGroup, MQCreateChatGroup)
	// 群主接收其他用户加群消息
	registerLogicCall(msg_id.NetMsgId_MQReqJoinChatGroup, MQReqJoinChatGroup)
	// 通知群其他成员，群主同意某用户加群申请
	registerLogicCall(msg_id.NetMsgId_MQManagerAgreeSomeOneReqJoinChatGroup, MQManagerAgreeSomeOneReqJoinChatGroup)
	// 通知申请加群用户，群主同意加群申请
	registerLogicCall(msg_id.NetMsgId_S2CManagerAgreeJoinChatGroup, S2CManagerAgreeJoinChatGroup)
	// 群主拒绝用户加群消息
	registerLogicCall(msg_id.NetMsgId_MQRefuseSomeOneReqJoinChatGroup, MQRefuseSomeOneReqJoinChatGroup)
	// 群成员邀请好友进群
	registerLogicCall(msg_id.NetMsgId_MQInviteJoinChatGroup, MQInviteJoinChatGroup)
	// 群成员退群
	registerLogicCall(msg_id.NetMsgId_MQCancelChatGroup, MQCancelChatGroup)
	// 群主解散群
	registerLogicCall(msg_id.NetMsgId_MQUnChatGroup, MQUnChatGroup)
	// 群主踢人
	registerLogicCall(msg_id.NetMsgId_MQChatGroupKick, MQChatGroupKick)
	// 用户发朋友圈
	registerLogicCall(msg_id.NetMsgId_MQPublishWeibo, MQPublishWeibo)
	// 用户点赞朋友圈
	registerLogicCall(msg_id.NetMsgId_MQLikeWeibo, MQLikeWeibo)
	// 用户评论朋友圈
	registerLogicCall(msg_id.NetMsgId_MQReplyWeibo, MQReplyWeibo)
	// 用户删除朋友圈
	registerLogicCall(msg_id.NetMsgId_MQDeleteWeibo, MQDeleteWeibo)
}

func MQPrivateChat(c *Client, msg []byte) {
	p := &msg_struct.MQPrivateChat{}
	if function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], p, "msg_struct.MQPrivateChat") {
		// 通知接收者
		chatMsg := p.GetChatMsg()
		if chatMsg != nil {
			ackMsg := &msg_struct.S2CRecvPrivateChat{}
			ackMsg.ChatMsg = chatMsg
			if !c.IsFriend(chatMsg.SenderId) {
				isMember, _ := c.rdsAgent.IsSetMember(strconv.FormatInt(chatMsg.SenderId, 10), c.GetStrAccount())
				if 1 != isMember && !c.IsAgentOrMember(chatMsg.SenderId) {
					return
				}
				ackMsg.ChatMsg.SenderRemarkName = c.GetAgentOrMemberUsername(chatMsg.SenderId)
			} else {
				ackMsg.ChatMsg.SenderRemarkName = c.GetFriendRemarkName(ackMsg.ChatMsg.GetSenderId())
			}

			if c.IsBlackList(chatMsg.SenderId) {
				return
			}

			c.SendPbMsg(msg_id.NetMsgId_S2CRecvPrivateChat, ackMsg)
			log.Tracef("account: %d 收到其他人 %d 发来的私聊消息 %s, 服务器上的时间戳 %d", c.GetAccountId(), chatMsg.SenderId, chatMsg.Text, chatMsg.SrvTimeStamp)
		}
	} else {
		log.Errorf("解析 MQ 消息失败")
	}
}

func MQAddFriend(c *Client, msg []byte) {
	log.Tracef("accountid: %d 收到好友请求", c.GetAccountId())
	p := &msg_struct.MQAddFriend{}
	if function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], p, "msg_struct.MQAddFriend") {
		addFriend := p.GetAddFriend()
		if addFriend != nil {
			if IsUserOnline(c, c.GetAccountId()) {
				log.Tracef("accountid: %d 收到好友请求, senderId: %d", c.GetAccountId(), addFriend.SenderId)
				ackMsg := &msg_struct.S2CRecvAddFriend{}
				ackMsg.SenderId = addFriend.SenderId
				ackMsg.ReceiverId = addFriend.ReceiverId
				ackMsg.SrvTimeStamp = time.Now().UnixNano() / 1e3
				ackMsg.SenderNickName = p.SenderNickName
				ackMsg.SenderHeadPortrait = c.GetUserHeadPortraitUrl(ackMsg.SenderId)
				c.SendPbMsg(msg_id.NetMsgId_S2CRecvAddFriend, ackMsg)
			}
		}
	}
}

func MQAgreeBecomeFriend(c *Client, msg []byte) {
	p := &msg_struct.MQAgreeBecomeFriend{}
	if function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], p, "msg_struct.AgreeBecomeFriend") {
		agreeBeFri := p.GetAgreeBeFriend()
		if agreeBeFri != nil {
			receiverId := agreeBeFri.GetReceiverId()
			if c.IsFriend(receiverId) == false {
				// 成为好友
				c.BeFriend(receiverId)
				log.Tracef("A 通过 MQ 获知 B 答应成为好友,  %d 和 %d 成为好友", c.GetAccountId(), receiverId)
				// 更新数据库
				c.SaveContactInfo()
				c.SaveFriendInfoToRedis()
			} else {
				// XXX 如果执行到这里,表示 A B 相互发送好友请求,又各自同意了好友请求
				log.Tracef("如果看到这条记录,表示 A B 相互发送好友请求,又各自同意了好友请求")
			}

			c.orm.DB().Exec("delete from friend_verification where friend_id = ? and account_id = ?",
				c.GetAccountId(), receiverId)

			c.SendPbMsg(msg_id.NetMsgId_S2CRecvAgreeBecomeFriend, &msg_struct.S2CRecvAgreeBecomeFriend{
				SenderId:             agreeBeFri.SenderId,
				ReceiverId:           agreeBeFri.ReceiverId,
				ReceiverNickName:     p.FriendNickName,
				ReceiverHeadPortrait: c.GetUserHeadPortraitUrl(receiverId),
			})

		} else {
			log.Errorf("agreeBeFri is nil")
		}
	}
}

func MQDeleteFriend(c *Client, msg []byte) {
	deleteFriend := &msg_struct.MQDeleteFriend{}
	if function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], deleteFriend, "msg_struct.MQDeleteFriend") {
		c.SendPbMsg(msg_id.NetMsgId_S2CDeleteFriend, &msg_struct.S2CDeleteFriend{
			FriendId: deleteFriend.FriendId,
		})
	}
}

func MQCreateChatGroup(c *Client, msg []byte) {
	p := &msg_struct.MQCreateChatGroup{}
	if function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], p, "msg_struct.MQCreateChatGroup") {
		ccg := p.GetCcg()
		if ccg != nil {
			eventArg := eventmanager.NewEventArg()
			eventArg.SetInt64("groupid", ccg.GetChatGroupId())
			eventArg.SetUserData("client", c)
			eventmanager.Publish(eventmanager.JoinChatGroupEvent, eventArg)

			ackMsg := &msg_struct.S2CCreateChatGroup{Ccg: ccg}
			c.SendPbMsg(msg_id.NetMsgId_S2CRecvCreateChatGroup, ackMsg)
		}
	}
}

func MQReqJoinChatGroup(c *Client, msg []byte) {
	mqReqJoinChatGroup := &msg_struct.MQReqJoinChatGroup{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], mqReqJoinChatGroup, "msg_struct.MQReqJoinChatGroup") {
		return
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CSomeOneReqJoinChatGroup, &msg_struct.S2CSomeOneReqJoinChatGroup{
		Rjcg:   mqReqJoinChatGroup.Rjcg,
		ReqUid: mqReqJoinChatGroup.ReqUid,
	})
}

func MQManagerAgreeSomeOneReqJoinChatGroup(c *Client, msg []byte) {
	mqManagerAgreeSomeOneReqJoinChatGroup := &msg_struct.MQManagerAgreeSomeOneReqJoinChatGroup{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], mqManagerAgreeSomeOneReqJoinChatGroup, "msg_struct.MQManagerAgreeSomeOneReqJoinChatGroup") {
		return
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CManagerAgreeSomeOneReqJoinChatGroup, &msg_struct.S2CManagerAgreeSomeOneReqJoinChatGroup{
		ChatGroupId:  mqManagerAgreeSomeOneReqJoinChatGroup.ChatGroupId,
		UserId:       mqManagerAgreeSomeOneReqJoinChatGroup.UserId,
		UserNickName: mqManagerAgreeSomeOneReqJoinChatGroup.UserNickName,
	})
}

func S2CManagerAgreeJoinChatGroup(c *Client, msg []byte) {
	mqAgreeSomeOneReqJoinChatGroup := &msg_struct.MQAgreeSomeOneReqJoinChatGroup{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], mqAgreeSomeOneReqJoinChatGroup, "msg_struct.MQAgreeSomeOneReqJoinChatGroup") {
		return
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CManagerAgreeJoinChatGroup, &msg_struct.S2CManagerAgreeJoinChatGroup{
		ChatGroupId: mqAgreeSomeOneReqJoinChatGroup.ChatGroupId,
	})
}

func MQRefuseSomeOneReqJoinChatGroup(c *Client, msg []byte) {
	refuseSomeOneReqJoinChatGroup := &msg_struct.MQRefuseSomeOneReqJoinChatGroup{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], refuseSomeOneReqJoinChatGroup, "msg_struct.MQRefuseSomeOneReqJoinChatGroup") {
		return
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CManagerRefuseJoinChatGroup, &msg_struct.S2CManagerRefuseJoinChatGroup{
		ChatGroupId:                   refuseSomeOneReqJoinChatGroup.ChatGroupId,
		RefuseReqJoinChatGroupMessage: refuseSomeOneReqJoinChatGroup.RefuseReqJoinChatGroupMessage,
	})
}

func MQInviteJoinChatGroup(c *Client, msg []byte) {
	inviteYouJoinChatGroup := &msg_struct.MQInviteJoinChatGroup{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], inviteYouJoinChatGroup, "msg_struct.MQInviteJoinChatGroup") {
		return
	}

	eventArg := eventmanager.NewEventArg()
	eventArg.SetInt64("groupid", inviteYouJoinChatGroup.InviteJoinChatGroup.ChatGroupId)
	eventArg.SetUserData("client", c)
	eventmanager.Publish(eventmanager.JoinChatGroupEvent, eventArg)

	c.SendPbMsg(msg_id.NetMsgId_S2CInviteYouJoinChatGroup, &msg_struct.S2CInviteYouJoinChatGroup{
		InviteJoinChatGroup: inviteYouJoinChatGroup.InviteJoinChatGroup,
	})
}

func MQCancelChatGroup(c *Client, msg []byte) {
	cancelChatGroup := &msg_struct.MQCancelChatGroup{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], cancelChatGroup, "msg_struct.MQCancelChatGroup") {
		return
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CSomeOneCancelChatGroup, &msg_struct.S2CSomeOneCancelChatGroup{
		CancelChatGroup: cancelChatGroup.CancelChatGroup,
	})
}

func MQUnChatGroup(c *Client, msg []byte) {
	mqUnChatGroup := &msg_struct.MQUnChatGroup{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], mqUnChatGroup, "msg_struct.MQUnChatGroup") {
		return
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CUnChatGroup, &msg_struct.S2CUnChatGroup{
		ChatGroupId: mqUnChatGroup.ChatGroupId,
	})
}

func MQChatGroupKick(c *Client, msg []byte) {
	chatGroupKick := &msg_struct.MQChatGroupKick{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], chatGroupKick, "msg_struct.MQChatGroupKick") {
		return
	}

	msgId := msg_id.NetMsgId_S2CManagerKickSomeOne

	if c.GetAccountId() == chatGroupKick.ChatGroupKick.UserId {
		msgId = msg_id.NetMsgId_S2CKickedByChatGroupManager
	}

	c.SendPbMsg(msgId, &msg_struct.S2CKickedByChatGroupManager{
		ChatGroupKick: chatGroupKick.ChatGroupKick,
	})
}

func MQPublishWeibo(c *Client, msg []byte) {
	mqPublishWeibo := &msg_struct.MQPublishWeibo{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], mqPublishWeibo, "msg_struct.MQPublishWeibo") {
		log.Errorf("%d用户收到他人发的朋友圈消息时，反序列化 msg_struct.MQPublishWeibo 失败", c.GetAccountId())
		return
	}

	/* 判断 mqPublishWeibo.Weibo.PublisherId 是不是“我”的好友 */
	if !c.IsFriend(mqPublishWeibo.Weibo.PublisherId) {
		log.Errorf("%d用户收到好友%d发的朋友圈消息时，他已经不是“我”的好友", c.GetAccountId(), mqPublishWeibo.Weibo.PublisherId)
		return
	}

	/* 判断“我”是不是 mqPublishWeibo.Weibo.PublisherId 的好友 */
	isFriend, err := c.rdsFriend.IsSetMember(strconv.FormatInt(mqPublishWeibo.Weibo.PublisherId, 10), c.GetStrAccount())
	if nil != err {
		log.Errorf("%d用户收到好友%d发的朋友圈消息时，从redis判断“我”是不是“他”的好友失败%v", c.GetAccountId(), mqPublishWeibo.Weibo.PublisherId, err)
		return
	}
	if 1 != isFriend {
		log.Warnf("%d用户收到好友%d发的朋友圈消息时，“我”已经不是“他”的好友", c.GetAccountId(), mqPublishWeibo.Weibo.PublisherId)
		return
	}

	/* 判断“我”在不在 mqPublishWeibo.Weibo.PublisherId 的黑名单列表 */
	bInOtherBlackList, err := IsSomeoneInOtherBlackList(c, mqPublishWeibo.Weibo.PublisherId, c.GetAccountId())
	if nil != err {
		log.Errorf("%d用户收到好友%d发的朋友圈消息时，从redis判断“我”是不是“他”的黑名单列表失败%v", mqPublishWeibo.Weibo.PublisherId, c.GetAccountId(), err)
		return
	}
	if bInOtherBlackList {
		log.Errorf("%d用户收到好友%d发的朋友圈消息时，“我”在“他”的黑名单列表", mqPublishWeibo.Weibo.PublisherId, c.GetAccountId())
		return
	}

	/* 判断 mqPublishWeibo.Weibo.PublisherId 在不在“我”的黑名单列表 */
	isMyBlackList, err := IsSomeoneInOtherBlackList(c, c.GetAccountId(), mqPublishWeibo.Weibo.PublisherId)
	if nil != err {
		log.Errorf("%d用户收到好友%d发的朋友圈消息时，从redis判断“他”是不是“我”的黑名单列表失败%v", c.GetAccountId(), mqPublishWeibo.Weibo.PublisherId, err)
		return
	}
	if isMyBlackList {
		log.Errorf("%d用户收到好友%d发的朋友圈消息时，“他”在“我”的黑名单列表", c.GetAccountId(), mqPublishWeibo.Weibo.PublisherId)
		return
	}

	/* 判断“我”看不看 mqPublishWeibo.Weibo.PublisherId 的朋友圈*/
	if !DoesLookFriendWeiboRds(c, c.GetAccountId(), mqPublishWeibo.Weibo.PublisherId) {
		log.Errorf("%d用户收到好友%d发的朋友圈消息时，“我”不看“他”的朋友圈", c.GetAccountId(), mqPublishWeibo.Weibo.PublisherId)
		return
	}

	/* 判断 mqPublishWeibo.Weibo.PublisherId 是否允许“我”看“他”的朋友圈 */
	if !DoesAllowFriendLookMyWeiboRds(c, mqPublishWeibo.Weibo.PublisherId, c.GetAccountId()) {
		log.Errorf("%d用户收到好友%d发的朋友圈消息时，“他”不允许“我”看朋友圈", c.GetAccountId(), mqPublishWeibo.Weibo.PublisherId)
		return
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CPublishWeibo, &msg_struct.S2CPublishWeibo{
		Weibo: mqPublishWeibo.Weibo,
	})
}

func MQLikeWeibo(c *Client, msg []byte) {
	likeWeibo := &msg_struct.MQLikeWeibo{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], likeWeibo, "msg_struct.MQLikeWeibo") {
		log.Errorf("用户%d接收好友点赞朋友圈信息，反序列化 msg_struct.MQLikeWeibo 失败", c.GetAccountId())
		return
	}

	/* 校验接收者与点赞者的好友关系 */
	if !c.IsFriend(likeWeibo.LikeWeibo.LikerId) {
		log.Errorf("用户%d点赞%d的朋友圈%d，接收者%d不是点赞者的好友", likeWeibo.LikeWeibo.LikerId, likeWeibo.LikeWeibo.WeiboPublisherId, likeWeibo.LikeWeibo.WeiboId, c.GetAccountId())
		return
	}

	/* 校验点赞者在不在接收者的黑名单里 */
	if c.IsBlackList(likeWeibo.LikeWeibo.LikerId) {
		log.Errorf("用户%d点赞%d的朋友圈%d，点赞者是接收者%d的黑名单好友", likeWeibo.LikeWeibo.LikerId, likeWeibo.LikeWeibo.WeiboPublisherId, likeWeibo.LikeWeibo.WeiboId, c.GetAccountId())
		return
	}

	if inOtherBlackList, err := IsSomeoneInOtherBlackList(c, likeWeibo.LikeWeibo.LikerId, c.GetAccountId()); nil != err || inOtherBlackList {
		log.Errorf("用户%d点赞%d的朋友圈%d，点赞者是接收者%d的黑名单好友", likeWeibo.LikeWeibo.LikerId, likeWeibo.LikeWeibo.WeiboPublisherId, likeWeibo.LikeWeibo.WeiboId, c.GetAccountId())
		return
	}

	/* 校验接收者看不看点赞者的朋友圈 */
	if !DoesLookFriendWeiboRds(c, c.GetAccountId(), likeWeibo.LikeWeibo.LikerId) {
		log.Tracef("用户%d点赞%d的朋友圈%d，接收者%d不看点赞者的朋友圈", likeWeibo.LikeWeibo.LikerId, likeWeibo.LikeWeibo.WeiboPublisherId, likeWeibo.LikeWeibo.WeiboId, c.GetAccountId())
		return
	}

	if !DoesAllowFriendLookMyWeiboRds(c, likeWeibo.LikeWeibo.LikerId, c.GetAccountId()) {
		return
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CLikeWeibo, &msg_struct.S2CLikeWeibo{
		LikeWeibo: likeWeibo.LikeWeibo,
	})
}

func MQReplyWeibo(c *Client, msg []byte) {
	replyWeibo := &msg_struct.MQReplyWeibo{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], replyWeibo, "msg_struct.MQReplyWeibo") {
		log.Errorf("用户%d接收好友点赞朋友圈信息，反序列化 msg_struct.MQReplyWeibo 失败", c.GetAccountId())
		return
	}

	/* 校验接收者与评论者的好友关系 */
	if !c.IsFriend(replyWeibo.ReplyWeibo.CommentatorId) {
		log.Errorf("用户%d评论%d的朋友圈%d评论%d，接收者%d不是评论者的好友",
			replyWeibo.ReplyWeibo.CommentatorId, replyWeibo.ReplyWeibo.WeiboPublisherId,
			replyWeibo.ReplyWeibo.WeiboId, replyWeibo.ReplyWeibo.CommentedId, c.GetAccountId())
		return
	}

	/* 校验接收者与被评论者的好友关系 */
	if replyWeibo.ReplyWeibo.BeCommentatorId != c.GetAccountId() && !c.IsFriend(replyWeibo.ReplyWeibo.BeCommentatorId) {
		log.Errorf("用户%d评论%d的朋友圈%d评论%d，接收者%d不是被评论者%d的好友",
			replyWeibo.ReplyWeibo.CommentatorId, replyWeibo.ReplyWeibo.WeiboPublisherId, replyWeibo.ReplyWeibo.WeiboId,
			replyWeibo.ReplyWeibo.CommentedId, c.GetAccountId(), replyWeibo.ReplyWeibo.BeCommentatorId)
		return
	}

	/* 校验评论者在不在接收者的黑名单里 */
	if c.IsBlackList(replyWeibo.ReplyWeibo.CommentatorId) {
		log.Errorf("用户%d点赞%d的朋友圈%d评论%d，评论者是接收者%d的黑名单好友",
			replyWeibo.ReplyWeibo.CommentatorId, replyWeibo.ReplyWeibo.WeiboPublisherId, replyWeibo.ReplyWeibo.WeiboId,
			replyWeibo.ReplyWeibo.CommentedId, c.GetAccountId())
		return
	}

	/* 校验被评论者在不在接收者的黑名单里 */
	if c.IsBlackList(replyWeibo.ReplyWeibo.BeCommentatorId) {
		log.Errorf("用户%d评论%d的朋友圈%d评论%d，被评论者%d是接收者%d的黑名单好友",
			replyWeibo.ReplyWeibo.CommentatorId, replyWeibo.ReplyWeibo.WeiboPublisherId, replyWeibo.ReplyWeibo.WeiboId,
			replyWeibo.ReplyWeibo.CommentedId, replyWeibo.ReplyWeibo.BeCommentatorId, c.GetAccountId())
		return
	}

	/* 校验接收者看不看评论者的朋友圈 */
	if !DoesLookFriendWeiboRds(c, replyWeibo.ReplyWeibo.CommentatorId, c.GetAccountId()) {
		log.Errorf("用户%d评论%d的朋友圈%d评论%d，接收者%d不看评论者的朋友圈",
			replyWeibo.ReplyWeibo.CommentatorId, replyWeibo.ReplyWeibo.WeiboPublisherId, replyWeibo.ReplyWeibo.WeiboId,
			replyWeibo.ReplyWeibo.CommentedId, c.GetAccountId())
		return
	}

	/* 校验接收者看不看被评论者的朋友圈 */
	if !DoesAllowFriendLookMyWeiboRds(c, replyWeibo.ReplyWeibo.BeCommentatorId, c.GetAccountId()) {
		log.Errorf("用户%d评论%d的朋友圈%d评论%d，接收者%d不看被评论者%d的朋友圈",
			replyWeibo.ReplyWeibo.CommentatorId, replyWeibo.ReplyWeibo.WeiboPublisherId, replyWeibo.ReplyWeibo.WeiboId,
			replyWeibo.ReplyWeibo.CommentedId, c.GetAccountId(), replyWeibo.ReplyWeibo.BeCommentatorId)
		return
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CReplyWeibo, &msg_struct.S2CReplyWeibo{
		ReplyWeibo: replyWeibo.ReplyWeibo,
	})
}

func MQDeleteWeibo(c *Client, msg []byte) {
	deleteWeibo := &msg_struct.MQDeleteWeibo{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], deleteWeibo, "msg_struct.MQDeleteWeibo") {
		return
	}

	/* 判断 deleteWeibo.FriendId 是不是“我”的好友 */
	if !c.IsFriend(deleteWeibo.FriendId) {
		log.Errorf("%d用户收到好友%d发的删除朋友圈消息时，他已经不是“我”的好友", c.GetAccountId(), deleteWeibo.FriendId)
		return
	}

	/* 判断“我”是不是 deleteWeibo.FriendId 的好友 */
	isFriend, err := c.rdsFriend.IsSetMember(strconv.FormatInt(deleteWeibo.FriendId, 10), c.GetStrAccount())
	if nil != err {
		log.Errorf("%d用户收到好友%d发的删除朋友圈消息时，从redis判断“我”是不是“他”的好友失败%v", c.GetAccountId(), deleteWeibo.FriendId, err)
		return
	}
	if 1 != isFriend {
		log.Warnf("%d用户收到好友%d发的删除朋友圈消息时，“我”已经不是“他”的好友", c.GetAccountId(), deleteWeibo.FriendId)
		return
	}

	/* 判断“我”在不在 deleteWeibo.FriendId 的黑名单列表 */
	bInOtherBlackList, err := IsSomeoneInOtherBlackList(c, c.GetAccountId(), deleteWeibo.FriendId)
	if nil != err {
		log.Errorf("%d用户收到好友%d发的删除朋友圈消息时，从redis判断“我”是不是“他”的黑名单列表失败%v", c.GetAccountId(), deleteWeibo.FriendId, err)
		return
	}
	if bInOtherBlackList {
		log.Errorf("%d用户收到好友%d发的删除朋友圈消息时，“我”在“他”的黑名单列表", c.GetAccountId(), deleteWeibo.FriendId)
		return
	}

	/* 判断 deleteWeibo.FriendId 在不在“我”的黑名单列表 */
	isMyBlackList, err := IsSomeoneInOtherBlackList(c, c.GetAccountId(), deleteWeibo.FriendId)
	if nil != err {
		log.Errorf("%d用户收到好友%d发的删除朋友圈消息时，从redis判断“他”是不是“我”的黑名单列表失败%v", c.GetAccountId(), deleteWeibo.FriendId, err)
		return
	}
	if isMyBlackList {
		log.Errorf("%d用户收到好友%d发的删除朋友圈消息时，“他”在“我”的黑名单列表", c.GetAccountId(), deleteWeibo.FriendId)
		return
	}

	/* 判断“我”看不看 deleteWeibo.FriendId 的朋友圈*/
	if !DoesLookFriendWeiboRds(c, c.GetAccountId(), deleteWeibo.FriendId) {
		log.Errorf("%d用户收到好友%d发的删除朋友圈消息时，“我”不看“他”的朋友圈", c.GetAccountId(), deleteWeibo.FriendId)
		return
	}

	/* 判断 deleteWeibo.FriendId 是否允许“我”看“他”的朋友圈 */
	if !DoesLookFriendWeiboRds(c, deleteWeibo.FriendId, c.GetAccountId()) {
		log.Errorf("%d用户收到好友%d发的删除朋友圈消息时，“他”不允许“我”看朋友圈", c.GetAccountId(), deleteWeibo.FriendId)
		return
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CDeleteWeibo, &msg_struct.S2CDeleteWeibo{
		FriendId: deleteWeibo.FriendId,
		WeiboId:  deleteWeibo.WeiboId,
	})
}
