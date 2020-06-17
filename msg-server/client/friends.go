package client

import (
	"database/sql"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"instant-message/common-library/models"
	"instant-message/common-library/packet"
	"instant-message/common-library/proto/msg_err_code"
	"instant-message/common-library/proto/msg_id"
	"instant-message/common-library/proto/msg_struct"
	"instant-message/im-library/redisPubSub"
	"instant-message/msg-server/eventmanager"
	"strconv"
	"time"
)

func init() {
	// A 向 B 发送好友申请, B 如果同时登录在 APP 和 WEB , 则都会收到好友请求
	registerLogicCall(msg_id.NetMsgId_C2SAddFriend, C2SAddFriend)
	// B 同意 A 的好友申请, 需要同步到其他端
	registerLogicCall(msg_id.NetMsgId_C2SAgreeBecomeFriend, C2SAgreeBecomeFriend)
	// 拒绝好友请求, 需要同步到其他端
	registerLogicCall(msg_id.NetMsgId_C2SReadFriendReq, C2SReadFriendReq)
	// 删除好友, 需要同步到其他端
	registerLogicCall(msg_id.NetMsgId_C2SDeleteFriend, C2SDeleteFriend)
	// 被删除的好友，确认删除好友信息
	registerLogicCall(msg_id.NetMsgId_C2SConfirmDeleteFriend, C2SConfirmDeleteFriend)
	// 好友拉入黑名单
	registerLogicCall(msg_id.NetMsgId_C2SAddFriendBlackList, C2SAddFriendBlackList)
	// 黑名单好友移出黑名单
	registerLogicCall(msg_id.NetMsgId_C2SMoveOutFriendBlackList, C2SMoveOutFriendBlackList)
	// 用户获取黑名单好友列表
	registerLogicCall(msg_id.NetMsgId_C2SGetFriendBlackList, C2SGetFriendBlackList)
	// 设置好友备注名
	registerLogicCall(msg_id.NetMsgId_C2SSetFriendRemarkName, C2SSetFriendRemarkName)
}

// 申请添加好友业务处理
func C2SAddFriend(c *Client, msg []byte) {
	reqMsg := &msg_struct.C2SAddFriend{}
	if function.ProtoUnmarshal(msg[8:], reqMsg, "msg_struct.C2SAddFriend") {

		// 合法性检查, 不能添加自己当好友
		if c.GetAccountId() == reqMsg.ReceiverId {
			c.SendErrorCode(msg_err_code.MsgErrCode_agree_become_friend_can_not_be_friend_with_self, reqMsg.MsgTimestamp)

			return
		}

		// 判断receiverId合法性
		strReceiverId := strconv.FormatInt(reqMsg.ReceiverId, 10)
		if _, err := c.rdsClientBriefInfo.Get(strReceiverId); err != nil {
			log.Errorf("in redis, Receiver Id Is Not Exist , ReceiverID = %v, error = %v",
				reqMsg.GetReceiverId(), err)

			c.SendErrorCode(msg_err_code.MsgErrCode_add_friend_param_error, reqMsg.MsgTimestamp)
			return
		}

		// XXX 下面是从 mysql 中查询, 做保留
		//if bExist, err := c.orm.Exist(&models.Account{
		//AccountId: reqMsg.GetReceiverId(),
		//}); nil != err || !bExist {
		//log.Errorf("Receiver Id Is Not Exist, ReceiverID = %v, exist = %v", reqMsg.GetReceiverId(), bExist)
		//c.SendErrorCode(msg_err_code.MsgErrCode_add_friend_param_error, reqMsg.MsgTimestamp)
		//}

		// 判断 receiverId 不在联系人列表
		if !c.IsFriend(reqMsg.GetReceiverId()) { // 接收者是不发送者的好友
			ack := &msg_struct.S2CAddFriend{SenderId: reqMsg.SenderId,
				ReceiverId:   reqMsg.ReceiverId,
				SrvTimeStamp: reqMsg.MsgTimestamp}

			var id int64
			err := c.orm.DB().QueryRow("select id from friend_req where receiver_id = ? and sender_id = ?",
				reqMsg.ReceiverId, reqMsg.SenderId).Scan(&id)
			if err == nil {
				// 发送添加好友消息到接收者
				if e := redisPubSub.SendPbMsg(reqMsg.ReceiverId, msg_id.NetMsgId_MQAddFriend,
					&msg_struct.MQAddFriend{
						AddFriend:      reqMsg,
						SenderNickName: c.account.NickName,
						SrvTimeStamp:   time.Now().UnixNano() / 1e3,
					}); nil != e {
					log.Errorf("向 MQ 发送消息失败: %s", e.Error())
				}

				c.SendPbMsg(msg_id.NetMsgId_S2CAddFriend, ack)

				return
			}

			// 使用 c.orm 把好友请求写入 mysql 表 friend_req 中, 如果写入失败, 表示要么发送过好友申请,要么是写 mysql 出错,
			sqlAddFriend := &models.FriendReq{
				ReceiverId:  reqMsg.ReceiverId,
				SenderId:    reqMsg.SenderId,
				ReqDatetime: time.Now(),
			}

			if num, err := c.orm.Insert(sqlAddFriend); nil != err || 0 == num {
				c.SendErrorCode(msg_err_code.MsgErrCode_add_friend_mysql_has_error_when_write, reqMsg.MsgTimestamp)
				log.Errorf("插入数据库 friend_req 失败 error = %v", err)
				return
			} else {
				// 发送添加好友消息到接收者
				if err := redisPubSub.SendPbMsg(sqlAddFriend.ReceiverId, msg_id.NetMsgId_MQAddFriend,
					&msg_struct.MQAddFriend{
						AddFriend:      reqMsg,
						SenderNickName: c.account.NickName,
						SrvTimeStamp:   time.Now().UnixNano() / 1e3,
					}); nil != err {
					log.Errorf(err.Error())
				}

				c.SendPbMsg(msg_id.NetMsgId_S2CAddFriend, ack)
			}
		} else { //	接收者是发送者的好友
			c.SendErrorCode(msg_err_code.MsgErrCode_add_friend_sender_and_recevier_are_friends, reqMsg.MsgTimestamp)
			log.Errorf("接收者是发送者的好友")
			return
		}
	} else {
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SAddFriend_has_error, reqMsg.MsgTimestamp)
		return
	}
}

// B 同意成为好友
func C2SAgreeBecomeFriend(c *Client, msg []byte) {
	reqMsg := &msg_struct.C2SAgreeBecomeFriend{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], reqMsg, "msg_struct.C2SAgreeBecomeFriend") {
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SAgreeBecomeFriend_has_error, reqMsg.MsgTimestamp)
		return
	}

	var id int64
	err := c.orm.DB().QueryRow("select id from friend_req where receiver_id = ? and sender_id = ?",
		c.GetAccountId(), reqMsg.SenderId).Scan(&id)
	// 没查询到
	if err != nil {
		if err == sql.ErrNoRows {
			// 没有好友申请
			c.SendErrorCode(msg_err_code.MsgErrCode_agree_become_friend_mysql_table_friend_req_has_no_record, reqMsg.MsgTimestamp)
		} else {
			// 查询数据库 friend_req 错误
			c.SendErrorCode(msg_err_code.MsgErrCode_agree_become_friend_mysql_table_friend_req_has_read_error, reqMsg.MsgTimestamp)
			log.Errorf("account: %d 查询数据库 friend_req 错误: %v", c.GetAccountId(), err)
		}

		return
	}

	// 合法性检查, 不能添加自己当好友
	if c.GetAccountId() == reqMsg.SenderId {
		c.SendErrorCode(msg_err_code.MsgErrCode_agree_become_friend_can_not_be_friend_with_self, reqMsg.MsgTimestamp)

		return
	}

	// 不信任客户端, 按照逻辑把 receiver_id 设置为 B 玩家的 id
	reqMsg.ReceiverId = c.GetAccountId()

	// 两人如果是好友,就 return
	if c.IsFriend(reqMsg.SenderId) {
		c.SendErrorCode(msg_err_code.MsgErrCode_agree_become_friend_sender_and_receiver_are_friends, reqMsg.MsgTimestamp)
		log.Errorf("Error:SenderID = %d and ReceiverId = %d, are friends", reqMsg.SenderId, reqMsg.ReceiverId)
		return
	}

	// reqMsg.ReceiverId 通过了 reqMsg.SenderId 的好友申请
	if _, err := c.orm.Insert(&models.FriendVerification{
		FriendId:             reqMsg.SenderId,
		AccountId:            reqMsg.ReceiverId,
		BecomeFriendDatetime: time.Now(),
	}); nil != err {
		c.SendErrorCode(msg_err_code.MsgErrCode_agree_become_friend_mysql_has_error_when_write, reqMsg.MsgTimestamp)
		return
	}

	// 添加好友
	c.BeFriend(reqMsg.SenderId)

	// 更新表 account 中的  contact_list 字段
	c.SaveContactInfo()
	c.SaveFriendInfoToRedis()
	_, err = c.rdsFriend.AddSetMembers(strconv.FormatInt(reqMsg.SenderId, 10), c.GetStrAccount())
	if nil != err {
		log.Errorf("用户%d同意用户%d的添加好友申请时，存入自己帐号到对方的好友关系到 redis出错：%v", err)
	}

	// 删除表 mysql 表 friend_req 中的相关记录
	if _, err := c.orm.Delete(&models.FriendReq{
		SenderId:   reqMsg.SenderId,
		ReceiverId: reqMsg.ReceiverId,
	}); nil != err {
		log.Errorf("delete friend request failed, err = %v", err)
	}

	// 把消息投递给 MQ
	if err := redisPubSub.SendPbMsg(reqMsg.SenderId, msg_id.NetMsgId_MQAgreeBecomeFriend,
		&msg_struct.MQAgreeBecomeFriend{
			AgreeBeFriend:  reqMsg,
			FriendNickName: c.account.NickName,
		}); nil != err {
		log.Errorf("Send MQ Agree Add Friend Failed : %v", err)
	}

	var senderName string
	p, err2 := c.QueryOtherBriefInfo(reqMsg.SenderId)
	if err2 == nil {
		senderName = p.Nickname
	}

	pbMsg := &msg_struct.S2CAgreeBecomeFriend{
		SenderId:                      reqMsg.SenderId,
		MsgTimestamp:                  reqMsg.MsgTimestamp,
		SenderNickName:                senderName,
		AgreeBecomeFriendHeadPortrait: c.GetUserHeadPortraitUrl(reqMsg.SenderId),
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CAgreeBecomeFriend, pbMsg)

	eventArg := eventmanager.NewEventArg()
	eventArg.SetUserData("client", c)
	eventArg.SetUserData("pbdata", pbMsg)
	eventmanager.Publish(eventmanager.AgreeBecomeFriendEvent, eventArg)

}

func C2SReadFriendReq(c *Client, msg []byte) {
	// 反序列化报文
	readFriendReq := &msg_struct.C2SReadFriendReq{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], readFriendReq, "msg_struct.C2SReadFriendReq") {
		return
	}

	_, e := c.orm.DB().Exec("delete from friend_req where "+
		"receiver_id = ? and sender_id = ?",
		c.GetAccountId(), readFriendReq.SenderId)
	if e != nil {
		log.Errorf("delete from friend_req where "+
			"receiver_id = %d and sender_id = %d Failed, err = %v",
			c.GetAccountId(), readFriendReq.SenderId, e)
	}

	p := &msg_struct.S2CReadFriendReq{SenderId: readFriendReq.SenderId}
	c.SendPbMsg(msg_id.NetMsgId_S2CReadFriendReq, p)

	eventArg := eventmanager.NewEventArg()
	eventArg.SetUserData("client", c)
	eventArg.SetUserData("pbdata", p)
	eventmanager.Publish(eventmanager.ReadFriendReqEvent, eventArg)

}

// C2SDeleteFriend 删除好友
func C2SDeleteFriend(c *Client, msg []byte) {
	deleteFriend := &msg_struct.C2SDeleteFriend{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], deleteFriend, "msg_struct.C2SDeleteFriend") {
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SDeleteFriend_has_error, 0)
		return
	}

	log.Tracef(fmt.Sprintf("用户%d开始删除好友%d", c.GetAccountId(), deleteFriend.FriendId))
	if !c.IsFriend(deleteFriend.FriendId) {
		c.SendErrorCode(msg_err_code.MsgErrCode_delete_friend_is_not_friend, 0)
		return
	}

	/* 更新数据库 */
	// 事务开始
	tx := c.orm.NewSession()
	defer tx.Close()
	err := tx.Begin()
	if err != nil || tx == nil {
		log.Errorf("%d删除%d好友时，开始数据库事务失败,error:%v", c.GetAccountId(), deleteFriend.FriendId, err)
		return
	}

	var errCode int32
	defer func() {
		if errCode != 0 {
			if tx != nil {
				e := tx.Rollback()
				if e != nil {
					log.Errorf("%d删除%d好友时, 回滚事务 tx.Rollback() 出现严重错误: %v", c.GetAccountId(), deleteFriend.FriendId, e)
				}
			}
		}
	}()

	c.BeNotFriend(deleteFriend.FriendId)
	/* 当前用户更新联系人列表 */
	cl := &msg_struct.ContactList{}
	c.lock.RLock()
	for k, _ := range c.mapContact {
		contact := &msg_struct.Contact{AccountId: k}
		cl.Contacts = append(cl.Contacts, contact)
	}
	c.lock.RUnlock()

	buf, _ := function.ProtoMarshal(cl, "msg_struct.ContactList")
	if _, err := tx.DB().Exec("update account set contact_list = ? where account_id = ?",
		buf, c.GetAccountId()); err != nil {
		errCode = 1
		c.BeFriend(deleteFriend.FriendId)
		log.Errorf("update account set contact_list = xxx where account_id = %d, err = %v", c.GetAccountId(), err)
		return
	}

	/* 删除好友记录插入数据库 */
	_, err = tx.Insert(&models.FriendDelete{
		SenderId:   c.GetAccountId(),
		ReceiverId: deleteFriend.FriendId,
		DeleteTime: time.Now(),
	})
	if nil != err {
		errCode = 2
		c.BeFriend(deleteFriend.FriendId)
		log.Errorf("%d删除%d好友时，insert friend delete出错，err = %v", c.GetAccountId(), deleteFriend.FriendId, err)
		return
	}

	// 事务结束
	err = tx.Commit()
	if err != nil {
		errCode = 3
		log.Errorf("%d删除%d好友时，提交事务 tx.Commit() 出现严重错误: %v", c.GetAccountId(), deleteFriend.FriendId, err)
		c.BeFriend(deleteFriend.FriendId)
		c.SendErrorCode(msg_err_code.MsgErrCode_delete_friend_server_internal_error, 0)
		return
	}

	/* 更新内存 */
	c.account.ContactList = buf

	/* 更新redis */
	_, err = c.rdsFriend.RemoveSetMembers(c.GetStrAccount(), strconv.FormatInt(deleteFriend.FriendId, 10))
	if nil != err {
		log.Errorf("%d删除%d好友时，从数据库删除成功，但从redis删除失败%v", c.GetAccountId(), deleteFriend.FriendId, err)
		c.SendErrorCode(msg_err_code.MsgErrCode_delete_friend_server_internal_error, 0)
		return
	}

	/* 通知MQ */
	if err := redisPubSub.SendPbMsg(deleteFriend.FriendId, msg_id.NetMsgId_MQDeleteFriend, &msg_struct.MQDeleteFriend{
		FriendId: c.GetAccountId(),
	}); nil != err {
		log.Errorf("%d删除%d好友时，通过MQ通知好友出错")
	}

	p := &msg_struct.S2CRecvDeleteFriend{FriendId: deleteFriend.FriendId}
	c.SendPbMsg(msg_id.NetMsgId_S2CRecvDeleteFriend, p)

	log.Tracef(fmt.Sprintf("用户%d删除好友%d完毕", c.GetAccountId(), deleteFriend.FriendId))

	eventArg := eventmanager.NewEventArg()
	eventArg.SetUserData("client", c)
	eventArg.SetUserData("pbdata", p)
	eventmanager.Publish(eventmanager.DeleteFriendEvent, eventArg)
}

// C2SConfirmDeleteFriend 确认删除好友信息
// A 删除 B, B 发送 C2SConfirmDeleteFriend 确认删除 A
func C2SConfirmDeleteFriend(c *Client, msg []byte) {
	confirmDeleteFriend := &msg_struct.C2SConfirmDeleteFriend{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], confirmDeleteFriend, "msg_struct.C2SConfirmDeleteFriend") {
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SConfirmDeleteFriend_has_error, 0)
		return
	}

	log.Tracef(fmt.Sprintf("用户%d开始确认被好友%d删除", c.GetAccountId(), confirmDeleteFriend.FriendId))
	if !c.IsFriend(confirmDeleteFriend.FriendId) {
		c.SendErrorCode(msg_err_code.MsgErrCode_confirm_friend_is_not_friend, 0)
		return
	}

	_, _ = c.rdsFriend.RemoveSetMembers(c.GetStrAccount(), strconv.FormatInt(confirmDeleteFriend.FriendId, 10))

	if c.DeleteFriend(confirmDeleteFriend.FriendId) {
		p := &msg_struct.S2CConfirmDeleteFriend{FriendId: confirmDeleteFriend.FriendId}
		c.SendPbMsg(msg_id.NetMsgId_S2CConfirmDeleteFriend, p)

		eventArg := eventmanager.NewEventArg()
		eventArg.SetUserData("client", c)
		eventArg.SetUserData("pbdata", p)
		eventmanager.Publish(eventmanager.ConfirmDeleteFriendEvent, eventArg)
		log.Tracef(fmt.Sprintf("用户%d确认被好友%d删除完毕", c.GetAccountId(), confirmDeleteFriend.FriendId))
	}
}

// C2SAddFriendBlackList 好友加入黑名单
func C2SAddFriendBlackList(c *Client, msg []byte) {
	addBlackList := &msg_struct.C2SAddBlackList{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], addBlackList, "msg_struct.C2SAddBlackList") {
		return
	}

	/* 校验两者是否为好友关系 */
	if !c.IsFriend(addBlackList.FriendId) {
		return
	}

	/* 校验好友是否已被拉入黑名单 */
	if c.IsBlackList(addBlackList.FriendId) {
		c.SendPbMsg(msg_id.NetMsgId_S2CAddFriendBlackList, &msg_struct.S2CAddBlackList{
			FriendId: addBlackList.FriendId,
		})
		return
	}

	/* 好友拉入黑名单，存入内存 */
	c.BeBlackList(addBlackList.FriendId)

	/* 好友拉入黑名单，存入redis */
	if err := BeBlackListRds(c, addBlackList.FriendId); nil != err {
		/* 流程出错，还原内存 */
		c.BeNotBlackList(addBlackList.FriendId)
		return
	}

	blackList_bin, bRet := getBlackListBin(c)
	if !bRet {
		/* 流程出错，还原内存 */
		c.BeNotBlackList(addBlackList.FriendId)
		/* 流程出错，还原redis */
		_ = BeNotBlackListRds(c, addBlackList.FriendId)
		return
	}

	/* 好友拉入黑名单，存入数据库 */
	if err := SaveBlackListDB(c, blackList_bin); nil != err {
		/* 流程出错，还原内存 */
		c.BeNotBlackList(addBlackList.FriendId)
		/* 流程出错，还原redis */
		_ = BeNotBlackListRds(c, addBlackList.FriendId)
		return
	}

	c.UpdateBlackListInfoInMemory()

	p := &msg_struct.S2CAddBlackList{FriendId: addBlackList.FriendId}
	c.SendPbMsg(msg_id.NetMsgId_S2CAddFriendBlackList, p)

	eventArg := eventmanager.NewEventArg()
	eventArg.SetUserData("client", c)
	eventArg.SetUserData("pbdata", p)
	eventmanager.Publish(eventmanager.C2SAddFriendBlackListEvent, eventArg)
}

// C2SMoveOutFriendBlackList 黑名单好友移出黑名单
func C2SMoveOutFriendBlackList(c *Client, msg []byte) {
	moveOutFriendBlackList := &msg_struct.C2SMoveOutFriendBlackList{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], moveOutFriendBlackList, "msg_struct.C2SMoveOutFriendBlackList") {
		return
	}

	if !c.IsFriend(moveOutFriendBlackList.FriendId) {
		return
	}

	if !c.IsBlackList(moveOutFriendBlackList.FriendId) {
		c.SendPbMsg(msg_id.NetMsgId_S2CMoveOutFriendBlackList, &msg_struct.S2CMoveOutFriendBlackList{
			FriendId: moveOutFriendBlackList.FriendId,
		})
		return
	}

	/* 好友移出黑名单，存入内存 */
	c.BeNotBlackList(moveOutFriendBlackList.FriendId)

	/* 好友移出黑名单，存入redis */
	if err := BeNotBlackListRds(c, moveOutFriendBlackList.FriendId); nil != err {
		/* 流程出错，还原内存 */
		c.BeBlackList(moveOutFriendBlackList.FriendId)
		return
	}

	blackList_bin, bRet := getBlackListBin(c)
	if !bRet {
		/* 流程出错，还原内存 */
		c.BeBlackList(moveOutFriendBlackList.FriendId)
		/* 流程出错，还原redis */
		_ = BeBlackListRds(c, moveOutFriendBlackList.FriendId)
		return
	}

	/* 好友移出黑名单，存入数据库 */
	if err := SaveBlackListDB(c, blackList_bin); nil != err {
		/* 流程出错，还原内存 */
		c.BeBlackList(moveOutFriendBlackList.FriendId)
		/* 流程出错，还原redis */
		_ = BeBlackListRds(c, moveOutFriendBlackList.FriendId)
		return
	}

	c.UpdateBlackListInfoInMemory()

	p := &msg_struct.S2CMoveOutFriendBlackList{
		FriendId: moveOutFriendBlackList.FriendId,
	}
	/* 回复客户端，黑名单好友移出黑名单列表 */
	c.SendPbMsg(msg_id.NetMsgId_S2CMoveOutFriendBlackList, p)

	eventArg := eventmanager.NewEventArg()
	eventArg.SetUserData("client", c)
	eventArg.SetUserData("pbdata", p)
	eventmanager.Publish(eventmanager.S2CMoveOutFriendBlackListEvent, eventArg)
}

func C2SGetFriendBlackList(c *Client, msg []byte) {
	getFriendBlackList := &msg_struct.C2SGetFriendBlackList{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], getFriendBlackList, "msg_struct.C2SGetFriendBlackList") {
		return
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CGetFriendBlackList, &msg_struct.S2CGetFriendBlackList{
		BlackListIds: c.GetBlackListIds(),
		Ret:          "Success",
	})
}

// C2SSetFriendRemarkName 设置好友备注名
func C2SSetFriendRemarkName(c *Client, msg []byte) {
	setFriendRemarkName := &msg_struct.C2SSetFriendRemarkName{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], setFriendRemarkName, "msg_struct.C2SSetFriendRemarkName") {
		log.Errorf("%d用户设置备注名，", c.GetAccountId())
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SSetFriendRemarkName_has_error, 0)
		return
	}

	if !c.IsFriend(setFriendRemarkName.GetFriendId()) {
		log.Errorf("%d用户设置备注名，", c.GetAccountId())
		c.SendErrorCode(msg_err_code.MsgErrCode_set_friend_remark_name_is_not_friend, 0)
		return
	}

	if "" == setFriendRemarkName.GetFriendRemarkName() {
		log.Errorf("%d用户设置备注名，", c.GetAccountId())
		c.SendErrorCode(msg_err_code.MsgErrCode_set_friend_remark_name_remark_name_is_empty, 0)
		return
	}

	/* 校验报文通过 */
	c.SetFriendRemarkName(setFriendRemarkName.GetFriendId(), setFriendRemarkName.GetFriendRemarkName())

	c.SaveContactInfo()
	c.SaveFriendInfoToRedis()

	p := &msg_struct.S2CSetFriendRemarkName{
		FriendId:         setFriendRemarkName.GetFriendId(),
		FriendRemarkName: setFriendRemarkName.GetFriendRemarkName()}
	c.SendPbMsg(msg_id.NetMsgId_S2CSetFriendRemarkName, p)

	eventArg := eventmanager.NewEventArg()
	eventArg.SetUserData("client", c)
	eventArg.SetUserData("pbdata", p)
	eventmanager.Publish(eventmanager.S2CSetFriendRemarkNameEvent, eventArg)

}

// getBlackListBin 好友黑名单列表序列化protobuf
func getBlackListBin(c *Client) ([]byte, bool) {
	blackList := &msg_struct.BlackList{}
	for _, blackListId := range c.GetBlackListIds() {
		blackList.Contacts = append(blackList.Contacts, &msg_struct.Contact{
			AccountId:  blackListId,
			RemarkName: c.GetFriendRemarkName(blackListId),
		})
	}
	return function.ProtoMarshal(blackList, "msg_struct.BlackList")
}

// SaveBlackListDB 好友黑名单列表，存入数据库
func SaveBlackListDB(c *Client, blackList_bin []byte) error {
	_, err := c.orm.DB().Exec("UPDATE account SET black_list = ? WHERE account_id = ?", blackList_bin, c.GetAccountId())
	return err
}

// BeBlackListRds 好友加入黑名单，存入redis
func BeBlackListRds(c *Client, friendId int64) error {
	_, err := c.rdsBlackList.AddSetMembers(c.GetStrAccount(), friendId)
	return err
}

// BeNotBlackListRds 黑名单好友移出黑名单，存入redis
func BeNotBlackListRds(c *Client, friendId int64) error {
	_, err := c.rdsBlackList.RemoveSetMembers(c.GetStrAccount(), friendId)
	return err
}

// IsSomeoneInOtherBlackList 判断someone在不在好友other的黑名单中
func IsSomeoneInOtherBlackList(c *Client, someoneId, otherId int64) (bool, error) {
	setMember, err := c.rdsBlackList.IsSetMember(strconv.FormatInt(otherId, 10), strconv.FormatInt(someoneId, 10))
	if nil != err {
		return false, err
	}
	return setMember == 1, nil
}

func (c *Client) DeleteFriend(friendId int64) bool {
	if !c.IsFriend(friendId) {
		c.SendErrorCode(msg_err_code.MsgErrCode_confirm_friend_is_not_friend, 0)
		return false
	}
	/* 更新数据库 */
	// 事务开始
	tx := c.orm.NewSession()
	defer tx.Close()
	err := tx.Begin()
	if err != nil || tx == nil {
		log.Errorf("%d确认被%d删除好友时，开始数据库事务失败,error:%v", c.GetAccountId(), friendId, err)
		c.SendErrorCode(msg_err_code.MsgErrCode_confirm_friend_server_internal_error, 0)
		return false
	}

	var errCode int32
	defer func() {
		if errCode != 0 {
			c.SendErrorCode(msg_err_code.MsgErrCode_confirm_friend_server_internal_error, 0)
			if tx != nil {
				e := tx.Rollback()
				if e != nil {
					log.Errorf("%d确认被%d删除好友时, 回滚事务 tx.Rollback() 出现严重错误: %v", c.GetAccountId(), friendId, e)
				}
			}
		}
	}()

	c.BeNotFriend(friendId)
	/* 当前用户更新联系人列表 */
	cl := &msg_struct.ContactList{}
	ids := c.GetFriendIds()
	for _, id := range ids {
		contact := &msg_struct.Contact{AccountId: id}
		cl.Contacts = append(cl.Contacts, contact)
	}

	buf, _ := function.ProtoMarshal(cl, "msg_struct.ContactList")
	if _, err := tx.DB().Exec("update account set contact_list = ? where account_id = ?",
		buf, c.GetAccountId()); err != nil {
		c.BeFriend(friendId)
		log.Errorf("update account set contact_list = xxx where account_id = %d, err = %v", c.GetAccountId(), err)
		return false
	}

	/* 标记此删除好友信息已被确认 */
	_, err = tx.DB().Exec("UPDATE friend_delete SET status = 1 WHERE receiver_id = ? and sender_id = ? and status = 0",
		c.GetAccountId(), friendId)
	if nil != err {
		errCode = 2
		c.BeFriend(friendId)
		log.Errorf("%d确认被%d删除好友时，更新delete_friend表失败，err=%v", c.GetAccountId(), friendId, err)
		return false
	}

	// 事务结束
	err = tx.Commit()
	if err != nil {
		errCode = 3
		c.BeFriend(friendId)
		log.Errorf("%d确认被%d删除好友时，提交事务 tx.Commit() 出现严重错误: %v", c.GetAccountId(), friendId, err)
		return false
	}

	/* 更新内存 */
	c.account.ContactList = buf
	return true
}

// GetMutualFriends 获取userId和friendId的共同好友
func (c *Client) GetMutualFriends(userId, friendId int64) ([]int64, error) {
	userIdStr := strconv.FormatInt(userId, 10)
	friendIdStr := strconv.FormatInt(friendId, 10)
	num, err := redis.Int(c.rdsFriend.Do("Sinterstore",
		c.rdsFriend.AddPrefix(userIdStr+":"+friendIdStr),
		c.rdsFriend.AddPrefix(userIdStr),
		c.rdsFriend.AddPrefix(friendIdStr)))
	if nil != err {
		log.Error("用户%d获取userId%d和friendId%d的共同好友时出错%v", c.GetAccountId(), userId, friendId, err)
		return nil, err
	}

	inters, err := redis.Values(c.rdsFriend.Do("SPOP", c.rdsFriend.AddPrefix(userIdStr+":"+friendIdStr), num))
	if nil != err {
		return nil, err
	}

	var mutualFriendIds []int64
	for _, inter := range inters {
		mutualFriendId, err := strconv.ParseInt(string(inter.([]byte)), 10, 64)
		if nil == err {
			mutualFriendIds = append(mutualFriendIds, mutualFriendId)
		}
	}

	return mutualFriendIds, nil
}
