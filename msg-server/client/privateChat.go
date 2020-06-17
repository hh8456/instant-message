package client

import (
	"errors"
	"fmt"
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"instant-message/common-library/packet"
	"instant-message/common-library/proto/msg_err_code"
	"instant-message/common-library/proto/msg_id"
	"instant-message/common-library/proto/msg_struct"
	"instant-message/common-library/pushService/jPushService"
	"instant-message/common-library/snowflake"
	"instant-message/im-library/redisPubSub"
	"instant-message/msg-server/eventmanager"
	"strconv"
	"time"

	"github.com/gogo/protobuf/proto"
)

var (
	mapFunc = make(map[msg_id.NetMsgId]func(*Client, []byte))
)

func init() {
	// 客户端发了条私聊消息
	registerLogicCall(msg_id.NetMsgId_C2SPrivateChat, C2SPrivateChat)
	// 客户端上报收到的最后一条私聊时间戳
	registerLogicCall(msg_id.NetMsgId_C2SLastMsgTimeStamp, C2SLastMsgTimestamp)
	// 安卓发送环信推送
	registerLogicCall(msg_id.NetMsgId_C2SHuanXinPush, C2SHuanXinPush)
	// 客户端登录后, 获取离线私聊消息
	registerLogicCall(msg_id.NetMsgId_C2SGetOfflinePrivateChatMsg, C2SGetOfflinePrivateChatMsg)
	// 客户端通知服务器准备发送图片,获得一个 uid
	registerLogicCall(msg_id.NetMsgId_C2SPicWillBeSend, C2SPicWillBeSend)
	// 客户端发送登出包
	registerLogicCall(msg_id.NetMsgId_C2SLogout, C2SLogout)
	// 客户端撤回消息
	registerLogicCall(msg_id.NetMsgId_C2SWithdrawMessage, C2SWithdrawMessage)
}

func registerLogicCall(id msg_id.NetMsgId, call func(*Client, []byte)) {
	mapFunc[id] = call
}

func getMsgFunc(id msg_id.NetMsgId) (func(*Client, []byte), bool) {
	f, ok := mapFunc[id]
	return f, ok
}

// 处理客户端上报服务器最后时间戳
func C2SLastMsgTimestamp(c *Client, msg []byte) {
	if len(msg) <= packet.PacketHeaderSize {
		log.Warnf("received unmatched message:%v", msg)
		return
	}
	reqMsg := &msg_struct.C2SLastMsgTimeStamp{}
	if function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], reqMsg, "msg_struct.C2SLastMsgTimeStamp") {
		/*
			1
				client 发送私聊消息 C2SPrivateChat 给 server 后, 会获得 server 服务器时间戳 S2CPrivateChat.SrvTimeStamp
				client 向 server 发送 C2SLastMsgTimestamp 时, C2SLastMsgTimestamp.SrvTimeStamp 就是 S2CPrivateChat.SrvTimeStamp
				所以 server 上当前时间戳 time.Now().UnixNano() / 1e3 必然大于 C2SLastMsgTimestamp.SrvTimeStamp

				这里客户端发送 S2CPrivateChat.SrvTimeStamp 的合法值是: S2CPrivateChat.SrvTimeStamp < current

			2
				server 上 c.account.LastMsgTimestamp 表示 client 前次上报的时间戳, 也必然小于 C2SLastMsgTimestamp.SrvTimeStamp
				在 time.Now().UnixNano() / 1e3 这个时间精度下,C2SLastMsgTimestamp.SrvTimeStamp 不可能等于 c.account.LastMsgTimestamp

				这里客户端发送 S2CPrivateChat.SrvTimeStamp 的合法值是:  S2CPrivateChat.SrvTimeStamp > c.account.LastMsgTimestamp


			3
				所以客户端发送的时间戳合法,会满足下面的表达式
				if lastMsgTimeStamp < current && lastMsgTimeStamp > c.account.LastMsgTimestamp {
				}

		*/

		lastMsgTimeStamp := reqMsg.GetSrvTimeStamp()
		//当前时间, 允许 10 秒的误差
		//current := time.Now().UnixNano() / 1e3
		current := time.Now().Add(time.Second*10).UnixNano() / 1e3
		if false == (lastMsgTimeStamp < current && lastMsgTimeStamp > c.account.LastMsgTimestamp) {
			log.Errorf("account id: %d 上报收到的最后一条私聊时间戳 %d 不合法, 应该 < 当前时间戳: %d, 并且 > account.LastMsgTimestamp: %d",
				c.account.AccountId,
				lastMsgTimeStamp,
				current,
				c.account.LastMsgTimestamp)

			return
		}
		//更新client最后消息时间
		c.SetLastMsgTimestamp(lastMsgTimeStamp)
		log.Tracef("update send timestamp,account id:%d,current timestamp:%d,received timestamp:%d,saved timestamp:%d\n",
			c.account.AccountId,
			current,
			lastMsgTimeStamp,
			c.account.LastMsgTimestamp)

		eventArg := eventmanager.NewEventArg()
		eventArg.SetUserData("client", c)
		eventArg.SetInt64("lastMsgTimeStamp", lastMsgTimeStamp)
		eventmanager.Publish(eventmanager.C2SLastMsgTimeStampEvent, eventArg)
	} else {
		log.Errorf("error message:%v", msg)

	}

}

// 客户端发送消息给服务器
func C2SPrivateChat(c *Client, msg []byte) {
	// 写入 redis 后, 回发给客户端, 再投递给 MQ
	reqMsg := &msg_struct.C2SPrivateChat{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], reqMsg, "msg_struct.C2SPrivateChat") {
		log.Error("收到客户端发来的私聊消息, msg_struct.C2SPrivateChat.ChatMessage 为空")
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarsharl_msg_struct_C2SPrivateChat_has_error, 0)
		return
	}

	chatMsg := reqMsg.GetChatMsg()
	if chatMsg != nil {
		if chatMsg.ReceiverId <= 0 {
			c.SendErrorCode(msg_err_code.MsgErrCode_private_chat_receiver_illegal, 0)
			log.Errorf("合法性检测未通过, msg_struct.C2SPrivateChat.ChatMessage.ReceiverId <= 0 ")
			return
		}
		/* 判断对方是不是自己好友 */
		if !c.IsFriend(chatMsg.ReceiverId) && !c.IsAgentOrMember(chatMsg.ReceiverId) {
			c.SendErrorCode(msg_err_code.MsgErrCode_private_chat_receiver_is_not_friend, 0)
			log.Errorf("%d发送私聊消息到%d时，接受者%d不是发送者%d的好友", c.GetAccountId(), chatMsg.ReceiverId, chatMsg.ReceiverId, c.GetAccountId())
			return
		}

		/* 判断自己是不是对方好友 */
		isFriend, _ := c.rdsFriend.IsSetMember(strconv.FormatInt(chatMsg.ReceiverId, 10), c.GetStrAccount())
		if 1 != isFriend {
			isMember, _ := c.rdsAgent.IsSetMember(strconv.FormatInt(chatMsg.ReceiverId, 10), c.GetStrAccount())
			if 1 != isMember && !c.IsAgentOrMember(chatMsg.ReceiverId) {
				c.SendErrorCode(msg_err_code.MsgErrCode_private_chat_receiver_is_not_friend, 0)
				log.Errorf("%d发送私聊消息到%d时，接受者%d不是发送者%d的好友", c.GetAccountId(), chatMsg.ReceiverId, chatMsg.ReceiverId, c.GetAccountId())
				return
			}
		}

		// 合法性检测
		b := chatMsg.MsgType == "word" || chatMsg.MsgType == "pic" || chatMsg.MsgType == "voice" || chatMsg.MsgType == "file" || chatMsg.MsgType == "video" || chatMsg.MsgType == "card"
		if b == false {
			c.SendErrorCode(msg_err_code.MsgErrCode_private_chat_receiver_type_illegal, 0)
			log.Errorf("合法性检测未通过, msg_struct.C2SPrivateChat.MsgType 不是预期值 ")
			return
		}

		chatMsg.SenderId = c.GetAccountId()
		chatMsg.SenderName = c.GetName()
		chatMsg.SenderheadPortraitUrl = c.GetUserHeadPortraitUrl(chatMsg.SenderId)

		if "voice" == chatMsg.MsgType {
			voiceMsg := chatMsg.GetVoiceMessage()
			if nil == voiceMsg {
				log.Errorf("用户%d发送语音消息到好友%d，但语音为空", chatMsg.GetSenderId(), chatMsg.GetReceiverId())
				c.SendErrorCode(msg_err_code.MsgErrCode_private_chat_message_type_not_match_message, 0)
				return
			}
			if "" == voiceMsg.GetAudioUrl() {
				log.Errorf("用户%d发送语音消息到好友%d，但语音url为空", chatMsg.GetSenderId(), chatMsg.GetReceiverId())
				c.SendErrorCode(msg_err_code.MsgErrCode_private_chat_voice_url_is_empty, 0)
				return
			}
			if 60 < voiceMsg.GetAudioSecond() {
				log.Errorf("用户%d发送语音消息到好友%d，但语音长度过长", chatMsg.GetSenderId(), chatMsg.GetReceiverId())
				c.SendErrorCode(msg_err_code.MsgErrCode_private_chat_voice_too_long, 0)
				return
			}
		}

		if "file" == chatMsg.MsgType {
			fileMessage := chatMsg.GetFileMessage()
			if nil == fileMessage {
				log.Errorf("用户%d发送文件消息到好友%d，但文件为空", chatMsg.GetSenderId(), chatMsg.GetReceiverId())
				c.SendErrorCode(msg_err_code.MsgErrCode_private_chat_message_type_not_match_message, 0)
				return
			}
			if "" == fileMessage.GetFileUrl() {
				log.Errorf("用户%d发送文件消息到好友%d，但文件url为空", chatMsg.GetSenderId(), chatMsg.GetReceiverId())
				c.SendErrorCode(msg_err_code.MsgErrCode_private_chat_file_url_is_empty, 0)
				return
			}
			if "" == fileMessage.FileThumbnail {
				log.Errorf("用户%d发送文件消息到好友%d，但文件缩略图url为空", chatMsg.GetSenderId(), chatMsg.GetReceiverId())
				c.SendErrorCode(msg_err_code.MsgErrCode_private_chat_file_thumbnail_url_is_empty, 0)
				return
			}
		}

		if "video" == chatMsg.MsgType {
			videoMessage := chatMsg.GetVideoMessage()
			if nil == videoMessage {
				log.Errorf("用户%d发送视频消息到好友%d，但视频为空", chatMsg.GetSenderId(), chatMsg.GetReceiverId())
				c.SendErrorCode(msg_err_code.MsgErrCode_private_chat_message_type_not_match_message, 0)
				return
			}
			if "" == videoMessage.VideoUrl {
				log.Errorf("用户%d发送视频消息到好友%d，但视频为空", chatMsg.GetSenderId(), chatMsg.GetReceiverId())
				c.SendErrorCode(msg_err_code.MsgErrCode_private_chat_video_url_is_empty, 0)
				return
			}
			if "" == videoMessage.VideoThumbnail {
				log.Errorf("用户%d发送视频消息到好友%d，但视频为空", chatMsg.GetSenderId(), chatMsg.GetReceiverId())
				c.SendErrorCode(msg_err_code.MsgErrCode_private_chat_video_thumbnail_url_is_empty, 0)
				return
			}
		}

		if "card" == chatMsg.MsgType {
			cardMessage := chatMsg.GetCardMessage()
			if nil == cardMessage {
				log.Errorf("用户%d发送名片消息到好友%d，但名片为空", chatMsg.GetSenderId(), chatMsg.GetReceiverId())
				c.SendErrorCode(msg_err_code.MsgErrCode_private_chat_card_message_is_empty, 0)
				return
			}
			headPortraitUrl := c.GetUserHeadPortraitUrl(cardMessage.Id)
			if "" != headPortraitUrl {
				chatMsg.CardMessage.Portrait = headPortraitUrl
			} else {
				log.Warnf("用户%d发送名片消息到好友%d，获取用户%d的头像失败，采用客户端上传的头像", c.GetAccountId(), chatMsg.GetReceiverId(), cardMessage.Id)
			}
			briefInfo, _ := c.GetOtherBriefInfoFromRds(cardMessage.Id)
			if nil == briefInfo {
				log.Errorf("用户%d发送名片消息到好友%d，获取用户%d的昵称失败，采用客户端上传的昵称", c.GetAccountId(), chatMsg.GetReceiverId(), cardMessage.Id)
			} else {
				chatMsg.CardMessage.NickName = briefInfo.Nickname
			}
		}

		// XXX 后期这里考虑换成 LUA, 以减少访问 IO 的次数
		ts := time.Now().UnixNano() / 1e3
		strSenderId := strconv.Itoa(int(chatMsg.SenderId))
		strReceiverId := strconv.Itoa(int(chatMsg.ReceiverId))
		chatMsg.SrvTimeStamp = ts

		buf, e := function.ProtoMarshal(chatMsg, "msg_struct.ChatMessage")
		if e {
			rdsTxt := string(buf)
			err := c.rdsPrivateChatRecord.SortedSetAddSingle(strSenderId, rdsTxt, ts)
			if err != nil {
				log.Errorf("操作 redis 错误: %v", err)
			}

			err = c.rdsPrivateChatRecord.SortedSetAddSingle(strReceiverId, rdsTxt, ts)
			if err != nil {
				log.Errorf("操作 redis 错误: %v", err)
			}

			// 20191216 尹延注释下方代码（撤回消息，不设置时间限制）
			// // 保存发出去的消息
			// key := c.GetStrAccount() + ":" + strconv.Itoa(int(ts))
			// c.chatRecord.Set(key, &rdsTxt, time.Minute*5)
		}

		// XXX 更高效的做法是,现在判断 B 是否在同服, 如果是的话,直接把消息投递给 B
		// 这里刻意投递给 MQ, 是为了验证流程是通畅的; 后期稳定后可以优化 19.10.20

		if IsUserOnline(c, chatMsg.ReceiverId) { // 用户 app 端或者 web 端其中一个在线，通过MQ发送在线私聊信息
			/* 投递给 MQ, 以便让 B 用户接收 */
			MQMsg := &msg_struct.MQPrivateChat{}
			MQMsg.ChatMsg = proto.Clone(chatMsg).(*msg_struct.ChatMessage)
			redisPubSub.SendPbMsg(chatMsg.ReceiverId, msg_id.NetMsgId_MQPrivateChat, MQMsg)
		} else { // 用户不在线，通过第三方推送平台，推送离线私聊信息
			deviceInfo := GetUserPushInfo(c, chatMsg.ReceiverId)
			if nil != deviceInfo {
				var iosReceiver, androidReceiver []string
				if "ios" == deviceInfo.DeviceProducter && "" != deviceInfo.DeviceInfo {
					iosReceiver = append(iosReceiver, deviceInfo.DeviceInfo)
				} else if "" != deviceInfo.DeviceInfo {
					androidReceiver = append(androidReceiver, deviceInfo.DeviceInfo)
				}
				if _, err := jPushService.Push(&jPushService.JPushServiceParam{
					AndroidReceiver: androidReceiver,
					IOSReceiver:     iosReceiver,
				}); nil != err {
					log.Errorf("%d发送私聊信息到%d时，发送推送信息失败", c.GetAccountId(), chatMsg.ReceiverId)
				}
			} else {
				log.Errorf("%d发送私聊信息到%d时，未获取到推送参数", c.GetAccountId(), chatMsg.ReceiverId)
			}
		}

		// 回复 sender
		ackMsg := &msg_struct.S2CPrivateChat{}
		ackMsg.ReceiverId = chatMsg.ReceiverId
		ackMsg.SrvTimeStamp = ts
		ackMsg.CliTimeStamp = reqMsg.GetCliTimeStamp()

		log.Tracef("account: %d 发给其他人: %d 的消息: %s, 服务器上的时间戳 %d", chatMsg.SenderId, chatMsg.ReceiverId, chatMsg.Text, ts)

		c.SendPbMsg(msg_id.NetMsgId_S2CPrivateChat, ackMsg)

		// 投递给其他端
		eventArg := eventmanager.NewEventArg()
		eventArg.SetUserData("client", c)
		eventArg.SetUserData("pbdata", chatMsg)
		eventmanager.Publish(eventmanager.PrivateChatEvent, eventArg)
	}
}

// C2SHuanXinPush 安卓发送环信推送
func C2SHuanXinPush(c *Client, msg []byte) {
	huanXinPush := &msg_struct.C2SHuanXinPush{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], huanXinPush, "msg_struct.C2SHuanXinPush") {
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SHuanXinPush_has_error, 0)
		log.Errorf("%d用户发送环信推送消息时，反序列化 msg_struct.C2SHuanXinPush 失败", c.GetAccountId())
		return
	}

	if !c.IsFriend(huanXinPush.FriendId) {
		c.SendErrorCode(msg_err_code.MsgErrCode_huan_xin_push_is_not_your_friend, 0)
		log.Errorf("%d用户发送环信推送消息时，对方%d不是你的好友", c.GetAccountId(), huanXinPush.FriendId)
		return
	}

	_, err := c.rdsFriend.IsSetMember(strconv.FormatInt(huanXinPush.FriendId, 10), c.GetStrAccount())
	if nil != err {
		c.SendErrorCode(msg_err_code.MsgErrCode_huan_xin_push_you_are_not_his_friend, 0)
		log.Errorf("%d用户发送环信推送消息时，你不是对方%d的好友", c.GetAccountId(), huanXinPush.FriendId)
		return
	}

	pushInfo := GetUserPushInfo(c, huanXinPush.FriendId)
	if nil == pushInfo {
		c.SendErrorCode(msg_err_code.MsgErrCode_huan_xin_push_get_friend_push_info_failed, 0)
		log.Errorf("%d用户发送环信推送消息到好友%d时，获取好友推送消息失败", c.GetAccountId(), huanXinPush.FriendId)
		return
	}

	if "ios" == pushInfo.DeviceProducter {
		c.SendErrorCode(msg_err_code.MsgErrCode_huan_xin_push_is_not_android, 0)
		log.Errorf("%d用户发送环信推送消息到好友%d时，好友为IOS，不是安卓", c.GetAccountId(), huanXinPush.FriendId)
		return
	}

	if !IsUserOnline(c, huanXinPush.FriendId) {
		var androidReceiver []string
		androidReceiver = append(androidReceiver, pushInfo.DeviceInfo)
		if _, err := jPushService.Push(&jPushService.JPushServiceParam{
			AndroidReceiver: androidReceiver,
		}); nil != err {
			log.Errorf("用户%d发送环信消息到好友%d时，发送推送信息失败", c.GetAccountId(), huanXinPush.FriendId)
		}
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CHuanXinPush, &msg_struct.S2CHuanXinPush{
		Ret: "Success",
	})
}

func C2SGetOfflinePrivateChatMsg(c *Client, msg []byte) {
	log.Tracef("account: %d 请求获取离线消息", c.GetAccountId())
	reqMsg := &msg_struct.C2SGetOfflinePrivateChatMsg{}
	if function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], reqMsg, "msg_struct.C2SGetOfflinePrivateChatMsg") {

		lastMsgTimestamp := c.GetLastMsgTimestamp()
		// 此处服务器做容错性处理
		// 比如A 服务器比B 服务器有时间误差，那么防止丢失消息，需要取客户端上传时间戳之前的时间来获取数据，并下发给客户端，以此来保证消息不会丢失
		// 此处提前10s, XXX 二期完成
		//protoMsgs, e := c.rdsPrivateChatRecord.SortedSetRangebyScore(c.GetStrAccount(), fmt.Sprintf("(%d", lastMsgTimestamp-int64(time.Second*10)/1e3), "+inf")
		protoMsgs, e := c.rdsPrivateChatRecord.SortedSetRangebyScore(c.GetStrAccount(), fmt.Sprintf("(%d", lastMsgTimestamp), "+inf")
		if e != nil {
			log.Errorf("处理用户获取离线私聊消息 C2SGetOfflinePrivateChatMsg时, 调用 SortedSetRangebyScore 接口操作 redis 出错,%v", e)
			return
		}

		ackMsg := &msg_struct.S2CGetOfflinePrivateChatMsg{}
		ackMsg.SrvTimeStamp = time.Now().UnixNano() / 1e3
		for _, v := range protoMsgs {
			chatMsg := &msg_struct.ChatMessage{}
			if function.ProtoUnmarshal(v.([]byte),
				chatMsg, "msg_struct.ChatMessage") {
				if chatMsg.SrvTimeStamp != lastMsgTimestamp {
					// 只发送 [别人发给他的消息, 他发给别人的消息过滤掉不发]
					if chatMsg.ReceiverId == c.GetAccountId() {
						ackMsg.Msgs = append(ackMsg.Msgs, chatMsg)
					}

					//log.Tracef("account:%d 获取到离线消息: receiverId:%d, "+
					//"senderId:%d, text:%s, srvTimeStamp:%d, msgType:%s",
					//c.GetAccountId(), chatMsg.ReceiverId, chatMsg.SenderId,
					//chatMsg.Text, chatMsg.SrvTimeStamp, chatMsg.MsgType)

				}
			} else {
				log.Errorf("解析redis消息失败:%v\n", v)
			}
		}

		log.Tracef("account: %d 请求获取离线消息完毕", c.GetAccountId())
		c.SendPbMsg(msg_id.NetMsgId_S2CGetOfflinePrivateChatMsg, ackMsg)
	}
}

func C2SPicWillBeSend(c *Client, msg []byte) {
	uid := snowflake.GetSnowflakeId()
	_, err := c.rdsUid.Do("set", uid, uid, "ex", 15) // 把字符串键值 uid - uid 保存进 redis, 数据存活期是 15 秒
	if err != nil {
		log.Errorf("操作 redis 错误: %v", err)
	}

	ackMsg := &msg_struct.S2CPicWillBeSend{}
	ackMsg.Uid = uid
	c.SendPbMsg(msg_id.NetMsgId_S2CPicWillBeSend, ackMsg)
}

func C2SLogout(c *Client, msg []byte) {
	c.SendPbMsg(msg_id.NetMsgId_S2CLogout, &msg_struct.S2CLogout{
		Str: "Success",
	})
}

func C2SWithdrawMessage(c *Client, msg []byte) {
	reqMsg := &msg_struct.C2SWithdrawMessage{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], reqMsg, "msg_struct.C2SWithdrawMessage") {
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SWithdrawMessage_has_error, 0)
		return
	}
	ts := reqMsg.ServerTimeStamp
	receiverId := reqMsg.ReceiverId

	inter, err := withDrawMsg(c, ts, receiverId, 0)
	if nil != err {
		log.Errorf(err.Error())
		c.SendErrorCode(msg_err_code.MsgErrCode_withDraw_message_private_chat_withdraw_fail, 0)
		return
	}
	if nil == inter {
		log.Errorf("用户%d撤回私聊消息%d时，未返回错误，但接口为空")
		c.SendErrorCode(msg_err_code.MsgErrCode_withDraw_message_private_chat_withdraw_fail, 0)
		return
	}
	chatMessage := inter.(*msg_struct.ChatMessage)

	if c.GetAccountId() != chatMessage.GetSenderId() {
		log.Errorf("用户%d撤回私聊消息%d时，消息发送者%d不是消息撤回者%d",
			c.GetAccountId(), ts, chatMessage.GetSenderId(), c.GetAccountId())
		c.SendErrorCode(msg_err_code.MsgErrCode_withDraw_message_withDrawer_is_not_sender, 0)
		return
	}

	ackMsg := &msg_struct.S2CWithdrawMessage{ServerTimeStamp: chatMessage.SrvTimeStamp}
	// 通知客户端撤回 serverTimeStamp = chatMsg.SrvTimeStamp 的消息
	c.SendPbMsg(msg_id.NetMsgId_S2CWithdrawMessage, ackMsg)

	if IsUserOnline(c, receiverId) { // 用户 app 端或者 web 端其中一个在线，通过MQ发送在线私聊信息
		/* 投递给 MQ, 以便让 B 用户接收 */
		MQMsg := &msg_struct.MQPrivateChat{}
		MQMsg.ChatMsg = proto.Clone(chatMessage).(*msg_struct.ChatMessage)
		redisPubSub.SendPbMsg(chatMessage.ReceiverId, msg_id.NetMsgId_MQPrivateChat, MQMsg)
	}
	/* 撤回消息不做推送处理 */
	//else { // 用户不在线，通过第三方推送平台，推送离线私聊信息
	//	deviceInfo := GetUserPushInfo(c, chatMsg.ReceiverId)
	//	if nil != deviceInfo {
	//		var iosReceiver, androidReceiver []string
	//		if "ios" == deviceInfo.DeviceProducter && "" != deviceInfo.DeviceInfo {
	//			iosReceiver = append(iosReceiver, deviceInfo.DeviceInfo)
	//		} else if "" != deviceInfo.DeviceInfo {
	//			androidReceiver = append(androidReceiver, deviceInfo.DeviceInfo)
	//		}
	//		if _, err := jPushService.Push(&jPushService.JPushServiceParam{
	//			AndroidReceiver: androidReceiver,
	//			IOSReceiver:     iosReceiver,
	//		}); nil != err {
	//			log.Errorf("%d发送私聊信息到%d时，发送推送信息失败", c.GetAccountId(), chatMsg.ReceiverId)
	//		}
	//	} else {
	//		log.Errorf("%d发送私聊信息到%d时，未获取到推送参数", c.GetAccountId(), chatMsg.ReceiverId)
	//	}
	//}

	// 投递给其他端
	eventArg := eventmanager.NewEventArg()
	eventArg.SetUserData("client", c)
	eventArg.SetUserData("pbdata", ackMsg)
	eventmanager.Publish(eventmanager.PrivateChatWithDrawMessageEvent, eventArg)
}

// withDrawMsg 服务器处理私聊、群聊撤回消息
func withDrawMsg(c *Client, msgSrvTimestamp, receiverId, groupId int64) (interface{}, error) {
	var (
		inter interface{}
	)
	if 0 != receiverId && 0 == groupId {
		privateChatRecords, err := c.rdsPrivateChatRecord.SortedSetRangebyScore(c.GetStrAccount(), msgSrvTimestamp, msgSrvTimestamp)
		if nil != err {
			return nil, errors.New(fmt.Sprintf("用户%d撤回私聊消息时，从 redis zset 读取消息：%d出错：%v", c.GetAccountId(), msgSrvTimestamp, err))
		}
		if nil == privateChatRecords {
			return nil, errors.New(fmt.Sprintf("用户%d撤回私聊消息时，从 redis zset 读取到消息：%d，但 privateChatRecords 为空", c.GetAccountId(), msgSrvTimestamp))
		}
		if 1 != len(privateChatRecords) {
			return nil, errors.New(fmt.Sprintf("用户%d撤回私聊消息时，从 redis zset 读取到消息：%d，但 privateChatRecords 不只有一个", c.GetAccountId(), msgSrvTimestamp))
		}
		chatRecordBin := privateChatRecords[0].([]byte)

		// 构造一个 ChatMessage 插入 redis
		chatMsg := &msg_struct.ChatMessage{}
		if !function.ProtoUnmarshal(chatRecordBin, chatMsg, "msg_struct.ChatMessage") {
			return nil, errors.New(fmt.Sprintf("用户%d撤回私聊消息%d时，反序列化 msg_struct.ChatMessage 失败",
				c.GetAccountId(), msgSrvTimestamp))
		}

		chatMsg.WithDraw = true

		privateChatBuf, bRet := function.ProtoMarshal(chatMsg, "msg_struct.ChatMessage")
		if !bRet {
			return nil, errors.New(fmt.Sprintf("用户%d撤回私聊消息%d时，序列化设置撤回标识的 msg_struct.ChatMessage 失败",
				c.GetAccountId(), msgSrvTimestamp))
		}
		inter = chatMsg

		rdsTxt := string(privateChatBuf)

		ts := time.Now().UnixNano() / 1e3
		err = c.rdsPrivateChatRecord.SortedSetAddSingle(c.strAccount, rdsTxt, ts)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("用户%d撤回消息%d时，存入 redis %s:%s zset 失败:%v",
				c.GetAccountId(), msgSrvTimestamp, c.rdsPrivateChatRecord.GetPrefix(), c.strAccount, err))
		}

		err = c.rdsPrivateChatRecord.SortedSetAddSingle(strconv.FormatInt(receiverId, 10), rdsTxt, ts)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("用户%d撤回消息%d时，存入 redis %s:%s zset 失败:%v",
				c.GetAccountId(), msgSrvTimestamp, c.rdsPrivateChatRecord.GetPrefix(), strconv.FormatInt(receiverId, 10), err))
		}
	}
	if 0 != groupId && 0 == receiverId {
		groupIdStr := strconv.FormatInt(groupId, 10)
		groupChatRecords, err := c.rdsGroupChatRecord.SortedSetRangebyScore(groupIdStr, msgSrvTimestamp, msgSrvTimestamp)
		if nil != err {
			return nil, errors.New(fmt.Sprintf("用户%d撤回群%s群聊消息时，从 redis zset 读取消息：%d出错：%v", c.GetAccountId(), groupIdStr, msgSrvTimestamp, err))
		}
		if nil == groupChatRecords {
			return nil, errors.New(fmt.Sprintf("用户%d撤回群%s群聊消息时，从 redis zset 读取到消息：%d，但 groupChatRecords 为空", c.GetAccountId(), groupIdStr, msgSrvTimestamp))
		}
		if 1 != len(groupChatRecords) {
			return nil, errors.New(fmt.Sprintf("用户%d撤回群%s群聊消息时，从 redis zset 读取到消息：%d，但 groupChatRecords 不只有一个", c.GetAccountId(), groupIdStr, msgSrvTimestamp))
		}
		chatRecordBin := groupChatRecords[0].([]byte)

		// 构造一个 GroupMessage 插入redis
		groupMessage := &msg_struct.GroupMessage{}
		if !function.ProtoUnmarshal(chatRecordBin, groupMessage, "msg_struct.GroupMessage") {
			return nil, errors.New(fmt.Sprintf("用户%d撤回群%s群聊消息%d时，反序列化 msg_struct.GroupMessage 失败",
				c.GetAccountId(), groupIdStr, msgSrvTimestamp))
		}
		groupMessage.WithDraw = true

		groupChatBuf, bRet := function.ProtoMarshal(groupMessage, "msg_struct.GroupMessage")
		if !bRet {
			return nil, errors.New(fmt.Sprintf("用户%d撤回群%s群聊消息%d时，序列化设置撤回标识的 msg_struct.GroupMessage 失败",
				c.GetAccountId(), groupIdStr, msgSrvTimestamp))
		}
		inter = groupMessage
		rdsTxt := string(groupChatBuf)

		ts := time.Now().UnixNano() / 1e3
		err = c.rdsGroupChatRecord.SortedSetAddSingle(groupIdStr, rdsTxt, ts)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("用户%d撤回群%s群聊消息%d时，存入 redis %s:%s zset 失败:%v",
				c.GetAccountId(), groupIdStr, msgSrvTimestamp, c.rdsGroupChatRecord.GetPrefix(), groupIdStr, err))
		}
	}

	return inter, nil
}

func (c *Client) IsHigherOrMember(agentId, userId int64) bool {
	i, err := c.rdsAgent.IsSetMember(strconv.FormatInt(agentId, 10), strconv.FormatInt(userId, 10))
	return nil != err && 1 == i
}
