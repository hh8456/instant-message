package client

/*
	redis哈希表时间复杂度：
		HSET		O(1)	// 设置一个键值对
		HGET		O(1)	// 获取一个键值对
		HGETALL 	O(N)	// 获取所有键值对	N为哈希表大小
		HMGET		O(N)	// 获取多个键值对	N为给定域数量
		HMSET		O(N)	// 设置多个键值对	N为键值对数量

	redis集合时间复杂度：
		SADD		O(N)	// 设置一个元素	N为元素数量
		SISMEMBER	O(1)	// 判断元素是否为成员
		SINTER		O(N*M)	// 返回所有元素	N为基数最小的集合，M为给定集合
*/

import (
	"encoding/json"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"hash/crc32"
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"instant-message/common-library/models"
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

	"github.com/go-xorm/xorm"

	"github.com/gogo/protobuf/proto"
	"github.com/hh8456/go-common/redisObj"
	"github.com/pjebs/optimus-go"
)

const (
	primeNum = 233323327
	randNum  = 684764274
)

const (
	GROUP_OWNER_UN_CHAT_GROUP     = iota // 群主解散群
	GROUP_MEMBER_QUIT_CHAT_GROUP         // 群成员主动退群
	GROUP_OWNER_KICK_GROUP_MEMBER        // 群主踢人出群
)

var (
	_optimusPrime optimus.Optimus
)

func init() {
	_optimusPrime = optimus.NewCalculated(primeNum, randNum)
	// 建立聊天群
	registerLogicCall(msg_id.NetMsgId_C2SCreateChatGroup, C2SCreateChatGroup)
	// 用户主动加入聊天群
	registerLogicCall(msg_id.NetMsgId_C2SReqJoinChatGroup, C2SReqJoinChatGroup)
	// 群主同意用户加入聊天群申请
	registerLogicCall(msg_id.NetMsgId_C2SAgreeSomeOneReqJoinChatGroup, C2SAgreeJoinChatGroup)
	// 群主拒绝用户加入聊天群申请
	registerLogicCall(msg_id.NetMsgId_C2SRefuseSomeOneReqJoinChatGroup, C2SRefuseJoinChatGroup)
	// 拉取群成员列表
	registerLogicCall(msg_id.NetMsgId_C2SChatGroupInfo, C2SChatGroupInfo)
	// 群成员邀请好友进群
	registerLogicCall(msg_id.NetMsgId_C2SInviteJoinChatGroup, C2SInviteJoinChatGroup)
	// 群成员退群
	registerLogicCall(msg_id.NetMsgId_C2SCancelChatGroup, C2SCancelChatGroup)
	// 群主解散群
	registerLogicCall(msg_id.NetMsgId_C2SUnChatGroup, C2SUnChatGroup)
	// 群主踢人
	registerLogicCall(msg_id.NetMsgId_C2SChatGroupKick, C2SChatGroupKick)
	// 发送群聊信息
	registerLogicCall(msg_id.NetMsgId_C2SGroupChat, C2SGroupChat)
	// 用户把收到的最后一条群消息时间戳上报给服务器
	registerLogicCall(msg_id.NetMsgId_C2SChatGroupLastMsgTimeStamp, C2SChatGroupLastMsgTimeStamp)
	// 群成员获取群离线消息
	registerLogicCall(msg_id.NetMsgId_C2SGetOfflineGroupChatMsg, C2SGetOfflineGroupChatMsg)
	// 群成员撤回群聊消息
	registerLogicCall(msg_id.NetMsgId_C2SWithdrawGroupMessage, C2SWithdrawGroupMessage)
}

// C2SCreateChatGroup 建立聊天群
func C2SCreateChatGroup(c *Client, msg []byte) {
	reqMsg := &msg_struct.C2SCreateChatGroup{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], reqMsg, "msg_struct.C2SCreateChatGroup") {
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SCreateChatGroup_has_error, 0)
		return
	}

	friendIds := reqMsg.GetFirendIds()

	/* 建群邀请好友判断 */
	mapInvitees := map[int64]int64{}
	mapHashInvitees := make(map[string]interface{})
	ChatGroupOwnerBytes, err := json.Marshal(&msg_struct.ChatGroupMemberRds{
		UserId:       c.GetAccountId(),
		UserNickName: c.account.NickName,
		Flag:         1,
	})
	if nil != err {
		log.Errorf("建群时，序列化群主json，msg_struct.ChatGroupMemberRds时出错，err:%v", err)
		c.SendErrorCode(msg_err_code.MsgErrCode_create_chat_group_marshal_chat_group_owner_failed, 0)
		return
	}
	mapHashInvitees[c.GetStrAccount()] = string(ChatGroupOwnerBytes)

	for _, invitee := range friendIds {
		if invitee != 0 && invitee != c.GetAccountId() {
			if !c.IsFriend(invitee) {
				log.Errorf("建群时，被邀请者%d不是邀请者%d的好友", invitee, c.GetAccountId())
				c.SendErrorCode(msg_err_code.MsgErrCode_create_chat_group_invitee_and_inviter_is_not_friend, 0)
				return
			}

			briefInfoFromRds, err := c.GetOtherBriefInfoFromRds(invitee)
			if nil != err {
				log.Errorf("建群时，群主%d获取好友%d的简略信息出错,err=%v", c.GetAccountId(), invitee, err)
				return
			}
			inviteeBriefInfoBytes, err := json.Marshal(&msg_struct.ChatGroupMemberRds{
				UserId:       briefInfoFromRds.AccountId,
				UserNickName: briefInfoFromRds.Nickname,
				Flag:         2,
			})
			if nil != err {
				log.Errorf("建群时，序列化被邀请者%d的ChatGroupMemberRds失败，err=%v", invitee, err)
				continue
			}

			mapInvitees[invitee] = 0
			mapHashInvitees[strconv.FormatInt(invitee, 10)] = string(inviteeBriefInfoBytes)
		}
	}

	// 用分布式锁来避免用户频繁创建群
	rdsCrChatGr := redisObj.NewSessionWithPrefix("creating_chat_group")
	e := rdsCrChatGr.Setex(c.GetStrAccount(), time.Second*10, 1)
	if e != nil {
		c.SendErrorCode(msg_err_code.MsgErrCode_ban_on_create_chat_group_in_short_time, 0)
		return
	}

	defer rdsCrChatGr.Del(c.GetStrAccount())

	// 对字符串 ( 当前时间戳 + accountId ) 计算 crc32, 用来生成群 id
	ts := time.Now().Unix()
	strTs := strconv.Itoa(int(ts))
	strAccount := strconv.Itoa(int(c.GetAccountId()))
	crc32Key := strTs + strAccount
	chatGroupId := int64(crc32.ChecksumIEEE([]byte(crc32Key)))

	chatGroupMember := []*models.ChatGroupMember{}
	// 群主
	chatGroupMember = append(chatGroupMember, &models.ChatGroupMember{
		AccountId:             c.GetAccountId(),
		ChatGroupId:           chatGroupId,
		NickName:              c.account.NickName,
		JoinChatGroupDatetime: time.Now(),
		LastMsgTimestamp:      time.Now().UnixNano() / 1e3,
	})
	// 其他成员
	for k, _ := range mapInvitees {
		cgm := &models.ChatGroupMember{}
		cgm.AccountId = k
		cgm.ChatGroupId = chatGroupId
		// 需要从 redis 中获取成员昵称
		briefInfo, e := c.GetOtherBriefInfoFromRds(cgm.AccountId)
		if e == nil {
			cgm.NickName = briefInfo.Nickname
		}

		cgm.JoinChatGroupDatetime = time.Now()
		cgm.LastMsgTimestamp = time.Now().UnixNano() / 1e3

		chatGroupMember = append(chatGroupMember, cgm)
	}

	// 事务开始
	tx := c.orm.NewSession()
	defer tx.Close()
	err = tx.Begin()
	if err != nil || tx == nil {
		log.Errorf("开始数据库事务失败,error:%v", err)
		return
	}

	var errCode int32
	defer func() {
		if errCode != 0 {
			if tx != nil {
				e := tx.Rollback()
				if e != nil {
					log.Errorf("创建聊天群时, 回滚事务 tx.Rollback() 出现严重错误: %v", e)
				}
			}
		}
	}()

	newChatGroup := &models.ChatGroup{
		ChatGroupId: chatGroupId,
		Creator:     c.GetAccountId(),
		Name:        reqMsg.ChatGroupName,
		Pic:         []byte(reqMsg.PicChatGroup),
	}
	// 事务 step1. 在 chat_group 中插入一条记录
	_, err = tx.Insert(newChatGroup)
	if nil != err {
		errCode = 1
		log.Errorf("建立聊天群时, 往数据库表 chat_group 中插入记录失败, ",
			"出现这条日志要排查 crc32 算法是否产生了碰撞; 本次 crc32 "+
				"生成的群 group id: %d, error: %v", newChatGroup.ChatGroupId, err)
		c.SendErrorCode(msg_err_code.MsgErrCode_create_chat_group_insert_chat_group_fail, 0)
		return
	}

	// 事务 step2. 把建群时的几个用户 ID和群 ID 写入表 chat_group_member
	_, err = tx.Insert(chatGroupMember)
	if err != nil {
		errCode = 4
		log.Errorf("创建聊天群时, 提交事务 tx.Commit() 出现严重错误: %v", err)
		c.SendErrorCode(msg_err_code.MsgErrCode_create_chat_group_insert_chat_group_member_fail, 0)
		return
	}

	// 事务结束
	err = tx.Commit()
	if err != nil {
		errCode = 5
		log.Errorf("创建聊天群时, 提交事务 tx.Commit() 出现严重错误: %v", err)
		c.SendErrorCode(msg_err_code.MsgErrCode_create_chat_group_commit_error_when_create_group, 0)
		return
	}

	// 记录建群日志
	groupCreateLog := &models.ChatGroupCreateLog{
		GroupId:        newChatGroup.ChatGroupId,
		Creator:        newChatGroup.Creator,
		CreateDatetime: time.Now(),
	}
	c.orm.Insert(groupCreateLog)

	// 把群信息写入 redis
	InsertIntoRedisChatGroupInfo(c, newChatGroup)
	/* 所有群成员（包括群主）加入redis群成员列表 */
	err = InsertIntoRedisGroupMembers(c, newChatGroup.ChatGroupId, mapHashInvitees)
	if nil != err {
		log.Errorf("建群时，所有群成员插入redis出错，err=%v", err)
		return
	}
	/* 被邀请者加入redis群成员列表 */
	crChatGr := &msg_struct.MQCreateChatGroup{
		Ccg: &msg_struct.CreateChatGroup{
			ChatGroupId:   newChatGroup.ChatGroupId,
			ChatGroupName: reqMsg.GetChatGroupName(),
			PicChatGroup:  reqMsg.GetPicChatGroup(),
		},
	}
	for invitee, _ := range mapInvitees {
		/* 通知被邀请者进群 */
		if err := redisPubSub.SendPbMsg(invitee, msg_id.NetMsgId_MQCreateChatGroup, crChatGr); nil != err {
			log.Errorf("redisPubSub.SendPbMsg has error: %v", err)
		}
	}

	eventArg := eventmanager.NewEventArg()
	eventArg.SetInt64("groupid", newChatGroup.ChatGroupId)
	eventArg.SetUserData("client", c)
	eventmanager.Publish(eventmanager.JoinChatGroupEvent, eventArg)

	ccg := &msg_struct.CreateChatGroup{}
	ccg.ChatGroupName = reqMsg.GetChatGroupName()
	ccg.PicChatGroup = reqMsg.GetPicChatGroup()
	for k, _ := range mapInvitees {
		ccg.Members = append(ccg.Members, k)
	}
	ccg.ChatGroupId = newChatGroup.ChatGroupId
	ccg.ChatGroupAdminId = c.GetAccountId()

	// 通知客户端建群成功
	c.SendPbMsg(msg_id.NetMsgId_S2CCreateChatGroup, &msg_struct.S2CCreateChatGroup{Ccg: ccg})
}

// C2SReqJoinChatGroup 用户主动加入聊天群
func C2SReqJoinChatGroup(c *Client, msg []byte) {
	reqJoinChatGroup := &msg_struct.C2SReqJoinChatGroup{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], reqJoinChatGroup, "msg_struct.C2SReqJoinChatGroup") {
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SReqJoinChatGroup_has_error, 0)
		return
	}

	/* 校验群ID */
	if exist, err := c.orm.Exist(&models.ChatGroup{
		ChatGroupId: reqJoinChatGroup.Rjcg.ChatGroupId,
	}); nil != err || !exist {
		c.SendErrorCode(msg_err_code.MsgErrCode_send_req_join_chat_group_group_id_error, 0)
		return
	}

	groupMemberRds := GetChatGroupMember(c, reqJoinChatGroup.Rjcg.ChatGroupId, reqJoinChatGroup.Rjcg.UserId)
	if nil != groupMemberRds {
		c.SendErrorCode(msg_err_code.MsgErrCode_send_req_join_chat_group_is_group_member, 0)
		return
	}

	// 事务开始
	tx := c.orm.NewSession()
	defer tx.Close()
	err := tx.Begin()
	if err != nil || tx == nil {
		log.Errorf("开始数据库事务失败,error:%v", err)
		return
	}

	var errCode int32
	defer func() {
		if errCode != 0 {
			if tx != nil {
				e := tx.Rollback()
				if e != nil {
					log.Errorf("用户申请加群时, 回滚事务 tx.Rollback() 出现严重错误: %v", e)
				}
			}
		}
	}()

	// 事务 step1. 查询群主ID
	reqChatGroup := &models.ChatGroup{}
	bGet, err := tx.Where("chat_group_id = ?", reqJoinChatGroup.Rjcg.ChatGroupId).Get(reqChatGroup)
	if nil != err || !bGet {
		errCode = 1
		c.SendErrorCode(msg_err_code.MsgErrCode_send_req_join_chat_group_query_group_manager_failed, 0)
		return
	}

	reqUid := snowflake.GetSnowflakeId()

	// 事务 step2. 插入表chat_group_req_join_log
	if _, err := tx.Insert(&models.ChatGroupReqJoinLog{
		ChatGroupId:    reqChatGroup.ChatGroupId,
		RequesterId:    c.GetAccountId(),
		RequestMessage: reqJoinChatGroup.Rjcg.ChatGroupReqMessage,
		Processed:      0, // 0表示未处理过，详见表设计
		ReqUid:         reqUid,
		ReqDate:        time.Now(),
	}); nil != err {
		errCode = 2
		c.SendErrorCode(msg_err_code.MsgErrCode_send_req_join_chat_group_query_group_manager_failed, 0)
		return
	}

	// 事务结束
	err = tx.Commit()
	if err != nil {
		errCode = 3
		log.Errorf("用户申请加群时, 提交事务 tx.Commit() 出现严重错误: %v", err)
		return
	}

	reqJoinChatGroup.Rjcg.UserNickName = c.account.NickName
	if err := redisPubSub.SendPbMsg(reqChatGroup.Creator, msg_id.NetMsgId_MQReqJoinChatGroup,
		&msg_struct.MQReqJoinChatGroup{
			Rjcg:   reqJoinChatGroup.Rjcg,
			ReqUid: reqUid,
		}); nil != err {
		c.SendErrorCode(msg_err_code.MsgErrCode_mq_group_manager_req_join_chat_group_failed, 0)
		return
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CSomeOneReqJoinChatGroup, &msg_struct.S2CRecvReqJoinChatGroup{
		Rjcg: reqJoinChatGroup.Rjcg,
	})
}

// C2SAgreeJoinChatGroup 群主同意用户加群
func C2SAgreeJoinChatGroup(c *Client, msg []byte) {
	agreeSomeOneReqJoinChatGroup := &msg_struct.C2SAgreeSomeOneReqJoinChatGroup{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], agreeSomeOneReqJoinChatGroup, "msg_struct.C2SAgreeSomeOneReqJoinChatGroup") {
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SAgreeSomeOneReqJoinChatGroup_has_error, 0)
		return
	}

	/* 校验群ID和群主身份 */
	chatGroup := &models.ChatGroup{}
	if bGet, err := c.orm.Where("chat_group_id = ?", agreeSomeOneReqJoinChatGroup.ChatGroupId).Get(chatGroup); nil != err || !bGet {
		c.SendErrorCode(msg_err_code.MsgErrCode_send_agree_some_one_req_join_chat_group_query_group_id_failed, 0)
		return
	}
	if c.GetAccountId() != chatGroup.Creator {
		c.SendErrorCode(msg_err_code.MsgErrCode_send_agree_some_one_req_join_chat_group_sender_is_not_manager, 0)
		return
	}

	isSetMember, err := c.rdsGroupChatRecord.IsSetMember(
		strconv.FormatInt(agreeSomeOneReqJoinChatGroup.ChatGroupId, 10),
		strconv.FormatInt(c.GetAccountId(), 10))
	if nil != err || 1 == isSetMember {
		c.orm.DB().Exec("UPDATE chat_group_req_join_log SET processed = 1 WHERE req_uid = ?", agreeSomeOneReqJoinChatGroup.ReqUid)
	} else {
		// 事务开始
		tx := c.orm.NewSession()
		defer tx.Close()
		err = tx.Begin()
		if err != nil || tx == nil {
			log.Errorf("开始数据库事务失败,error:%v", err)
			return
		}

		var errCode int32
		defer func() {
			if errCode != 0 {
				if tx != nil {
					e := tx.Rollback()
					if e != nil {
						log.Errorf("用户申请加群时, 回滚事务 tx.Rollback() 出现严重错误: %v", e)
					}
				}
			}
		}()

		// 事务 step1. 从chat_group_req_join_log核实申请记录
		reqJoinLog := &models.ChatGroupReqJoinLog{}
		bGet, err := tx.Where("req_uid = ?", agreeSomeOneReqJoinChatGroup.ReqUid).Get(reqJoinLog)
		if nil != err || !bGet {
			errCode = 1
			c.SendErrorCode(msg_err_code.MsgErrCode_send_agree_some_one_req_join_chat_group_query_reques_uid_failed, 0)
			return
		}

		// 事务 step2. 把用户id插入chat_group_member
		if _, err := tx.Insert(&models.ChatGroupMember{
			AccountId:             agreeSomeOneReqJoinChatGroup.ReqId,
			ChatGroupId:           agreeSomeOneReqJoinChatGroup.ChatGroupId,
			NickName:              agreeSomeOneReqJoinChatGroup.ReqNickName,
			JoinChatGroupDatetime: time.Now(),
			LastMsgTimestamp:      time.Now().UnixNano() / 1e3,
		}); nil != err {
			errCode = 2
			c.SendErrorCode(msg_err_code.MsgErrCode_send_agree_some_one_req_join_chat_group_insert_chat_group_member_failed, 0)
			return
		}

		// 事务 step3. 更新表chat_group_req_join_log
		reqJoinLog.Processed = 1 // 1表示处理成功
		if _, err = tx.Update(reqJoinLog); nil != err {
			errCode = 3
			c.SendErrorCode(msg_err_code.MsgErrCode_send_agree_some_one_req_join_chat_group_update_chat_group_req_join_log_failed, 0)
			return
		}

		// 事务结束
		err = tx.Commit()
		if err != nil {
			errCode = 4
			log.Errorf("群主同意用户申请加群时, 提交事务 tx.Commit() 出现严重错误: %v", err)
			return
		}
	}

	/* 通知申请者，群主同意加群消息 */
	if err := redisPubSub.SendPbMsg(agreeSomeOneReqJoinChatGroup.ReqId, msg_id.NetMsgId_MQAgreeSomeOneReqJoinChatGroup, &msg_struct.MQAgreeSomeOneReqJoinChatGroup{
		ChatGroupId: agreeSomeOneReqJoinChatGroup.ChatGroupId,
	}); nil != err {
		c.SendErrorCode(msg_err_code.MsgErrCode_send_agree_some_one_req_join_chat_group_notice_requester_failed, 0)
		return
	}

	/* 通知群其他成员,某用户进入聊天群 */
	setMembers, err := c.rdsGroupChatRecord.GetSetMembers("set:" + strconv.FormatInt(agreeSomeOneReqJoinChatGroup.ChatGroupId, 10))
	if nil != err {
		c.SendErrorCode(msg_err_code.MsgErrCode_send_agree_some_one_req_join_chat_group_get_members_failed, 0)
		return
	}

	managerAgreeSomeOneReqJoinChatGroup := &msg_struct.MQManagerAgreeSomeOneReqJoinChatGroup{
		ChatGroupId:  agreeSomeOneReqJoinChatGroup.ChatGroupId,
		UserId:       agreeSomeOneReqJoinChatGroup.ReqId,
		UserNickName: agreeSomeOneReqJoinChatGroup.ReqNickName,
	}

	for _, member := range setMembers {
		redisPubSub.SendPbMsg(member.(int64), msg_id.NetMsgId_MQManagerAgreeSomeOneReqJoinChatGroup, managerAgreeSomeOneReqJoinChatGroup)
	}

	/* 新成员加入redis中 */
	userBriefInfos := map[string]*msg_struct.UserBriefInfo{}
	userBriefInfos[strconv.FormatInt(agreeSomeOneReqJoinChatGroup.ReqId, 10)] = &msg_struct.UserBriefInfo{
		AccountId: agreeSomeOneReqJoinChatGroup.ReqId,
		Nickname:  agreeSomeOneReqJoinChatGroup.ReqNickName,
	}
	//err = InsertIntoRedisGroupMembers(c, agreeSomeOneReqJoinChatGroup.ChatGroupId, userBriefInfos)

	c.SendPbMsg(msg_id.NetMsgId_S2CRecvManagerAgreeSomeOneReqJoinChatGroup, &msg_struct.S2CRecvManagerAgreeSomeOneReqJoinChatGroup{
		ChatGroupId: agreeSomeOneReqJoinChatGroup.ChatGroupId,
		UserId:      agreeSomeOneReqJoinChatGroup.ReqId,
	})

	eventArg := eventmanager.NewEventArg()
	eventArg.SetInt64("groupid", agreeSomeOneReqJoinChatGroup.ChatGroupId)
	eventArg.SetUserData("client", c)
	eventmanager.Publish(eventmanager.JoinChatGroupEvent, eventArg)
}

// C2SRefuseJoinChatGroup 群主拒绝用户加群
func C2SRefuseJoinChatGroup(c *Client, msg []byte) {
	refuseSomeOneReqJoinChatGroup := &msg_struct.C2SRefuseSomeOneReqJoinChatGroup{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], refuseSomeOneReqJoinChatGroup, "msg_struct.C2SRefuseSomeOneReqJoinChatGroup") {
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SRefuseSomeOneReqJoinChatGroup_has_error, 0)
		return
	}

	existAccount, errAccount := c.orm.Exist(&models.Account{
		AccountId: refuseSomeOneReqJoinChatGroup.UserId,
	})
	existGroup, errGroup := c.orm.Exist(&models.ChatGroup{
		ChatGroupId: refuseSomeOneReqJoinChatGroup.ChatGroupId,
	})
	if !existAccount || !existGroup || nil != errAccount || nil != errGroup {
		c.SendErrorCode(msg_err_code.MsgErrCode_send_refuse_req_some_one_req_join_chat_group_error, 0)
		return
	}

	/* 更新表chat_group_req_join_log */
	_, err := c.orm.DB().Exec("UPDATE chat_group_req_join SET processed = 1 WHERE req_uid = ?", refuseSomeOneReqJoinChatGroup.ReqUid)
	if nil != err {
		c.SendErrorCode(msg_err_code.MsgErrCode_send_refuse_some_one_req_join_chat_group_update_chat_group_req_join_log_failed, 0)
		return
	}

	redisPubSub.SendPbMsg(refuseSomeOneReqJoinChatGroup.UserId, msg_id.NetMsgId_MQRefuseSomeOneReqJoinChatGroup,
		&msg_struct.MQRefuseSomeOneReqJoinChatGroup{
			ChatGroupId:                   refuseSomeOneReqJoinChatGroup.ChatGroupId,
			ReqUid:                        refuseSomeOneReqJoinChatGroup.ReqUid,
			RefuseReqJoinChatGroupMessage: refuseSomeOneReqJoinChatGroup.RefuseReqJoinChatGroupMessage,
		})
}

// C2SChatGroupInfo 拉取群成员列表
func C2SChatGroupInfo(c *Client, msg []byte) {
	chatGroupInfo := &msg_struct.C2SChatGroupInfo{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], chatGroupInfo, "msg_struct.C2SChatGroupInfo") {
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SChatGroupInfo_has_error, 0)
		return
	}

	/* 校验群ID */
	if exist, err := c.orm.Exist(&models.ChatGroup{
		ChatGroupId: chatGroupInfo.ChatGroupId,
	}); nil != err || !exist {
		c.SendErrorCode(msg_err_code.MsgErrCode_send_chat_group_info_group_id_error, 0)
		return
	}

	/* 群信息 */
	chatGroup := &models.ChatGroup{}
	bGet, err := c.orm.Where("chat_group_id = ?", chatGroupInfo.ChatGroupId).Get(chatGroup)
	if nil != err || !bGet {
		c.SendErrorCode(msg_err_code.MsgErrCode_send_chat_group_info_get_group_info_failed, 0)
		return
	}

	/* 群成员列表 */
	getAll, err := c.rdsGroupChatRecord.HashGetAll("hash:" + strconv.FormatInt(chatGroupInfo.ChatGroupId, 10))
	if nil != err || nil == getAll {
		c.SendErrorCode(msg_err_code.MsgErrCode_send_chat_group_info_get_group_member_failed, 0)
		return
	}

	/* 从redis hash读出来的数据，在遍历时，第一次为key值，第二次为value值，依次循环，此处只取value值，所以奇数次跳过 */
	chatGroupMemberRds := make([]*msg_struct.ChatGroupMemberRds, 0)
	for index, member := range getAll {
		if 0 == index%2 {
			continue
		}

		groupMemberRds := &msg_struct.ChatGroupMemberRds{}
		if err := json.Unmarshal(member.([]byte), groupMemberRds); nil != err {
			log.Errorf("读取redis hash好友列表，转为msg_struct.ChatGroupMemberRds结构体出错")
			continue
		}

		nickName := ""
		briefInfo, err := c.GetOtherBriefInfoFromRds(groupMemberRds.UserId)
		if nil != err {
			nickName = groupMemberRds.UserNickName
		} else {
			nickName = briefInfo.Nickname
		}

		chatGroupMemberRds = append(chatGroupMemberRds, &msg_struct.ChatGroupMemberRds{
			UserId:              groupMemberRds.UserId,
			UserNickName:        nickName,
			Flag:                groupMemberRds.Flag,
			UserHeadPortraitUrl: c.GetUserHeadPortraitUrl(groupMemberRds.UserId),
		})
	}

	/* 回复用户拉取群信息 */
	c.SendPbMsg(msg_id.NetMsgId_S2CChatGroupInfo, &msg_struct.S2CChatGroupInfo{
		ChatGroupId:           chatGroup.ChatGroupId,
		ChatGroupName:         chatGroup.Name,
		ChatGroupHeadPortrait: string(chatGroup.Pic),
		ChatGroupOwnerId:      chatGroup.Creator,
		ChatGroupMemberRds:    chatGroupMemberRds,
	})
}

// C2SInviteJoinChatGroup 群成员邀请好友进群
func C2SInviteJoinChatGroup(c *Client, msg []byte) {
	inviteJoinChatGroup := &msg_struct.C2SInviteJoinChatGroup{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], inviteJoinChatGroup, "msg_struct.C2SInviteJoinChatGroup") {
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SInviteJoinChatGroup_has_error, 0)
		return
	}

	/* 校验群ID */
	strChatGroupId := strconv.Itoa(int(inviteJoinChatGroup.ChatGroupId))
	_, err := c.rdsChatGroupInfo.Get(strChatGroupId)
	if nil != err {
		c.SendErrorCode(msg_err_code.MsgErrCode_send_invite_join_chat_group_group_id_error, 0)
		return
	}

	/* 校验好友ID */
	mapInviteFriendSuccess := make(map[int64]string)
	mapInvitees := make(map[string]interface{})
	chatGroupMembers := []*models.ChatGroupMember{}
	for _, friendId := range inviteJoinChatGroup.FriendIds {
		/* 校验好友关系 */
		if !c.IsFriend(friendId) {
			c.SendErrorCode(msg_err_code.MsgErrCode_send_invite_join_chat_group_is_not_friend_error, 0)
			return
		}

		friendInfo, err2 := c.GetOtherBriefInfoFromRds(friendId)
		if nil != err2 {
			c.SendErrorCode(msg_err_code.MsgErrCode_send_invite_join_chat_group_invitee_id_error, 0)
			return
		}

		/* 校验好友是否进群 */
		isFriendInChatGroup, err3 := c.rdsGroupChatRecord.GetHashSetField(
			"hash:"+strconv.FormatInt(inviteJoinChatGroup.ChatGroupId, 10),
			strconv.FormatInt(friendId, 10))
		if nil != err3 {
			if redis.ErrNil != err3 {
				log.Warnf("C2SInviteJoinChatGroup().GetHashSetField() has error%v, inviter=%d, invitee=%d:", err3, c.GetAccountId(), friendId)
				continue
			}
		}
		if "" != isFriendInChatGroup {
			c.SendErrorCode(msg_err_code.MsgErrCode_send_invite_join_chat_group_has_joined_chat_group_error, 0)
			return
		}

		bytes, err2 := json.Marshal(&msg_struct.ChatGroupMemberRds{
			UserId:       friendInfo.AccountId,
			UserNickName: friendInfo.Nickname,
			Flag:         2,
		})
		if nil != err2 {
			log.Errorf("邀请好友进群时，邀请者%s，被邀请者%s，出错:err=", c.GetAccountId(), friendId, err2)
			continue
		}

		/* 内存数据汇总 */
		chatGroupMembers = append(chatGroupMembers, &models.ChatGroupMember{
			AccountId:             friendId,
			ChatGroupId:           inviteJoinChatGroup.ChatGroupId,
			NickName:              friendInfo.Nickname,
			JoinChatGroupDatetime: time.Now(),
			LastMsgTimestamp:      time.Now().UnixNano() / 1e3,
		})
		mapInviteFriendSuccess[friendId] = friendInfo.Nickname
		mapInvitees[strconv.FormatInt(friendId, 10)] = string(bytes)
	}

	/* 好友加入群成员列表chat_group_member */
	if _, err = c.orm.Insert(chatGroupMembers); nil != err {
		c.SendErrorCode(msg_err_code.MsgErrCode_send_invite_join_chat_group_insert_chat_group_member_failed, 0)
		return
	}

	/* 好友加入群成员列表redis */
	err = InsertIntoRedisGroupMembers(c, inviteJoinChatGroup.ChatGroupId, mapInvitees)
	if nil != err {
		c.SendErrorCode(msg_err_code.MsgErrCode_send_invite_join_chat_group_insert_chat_group_member_failed, 0)
		return
	}

	/* 获取群成员列表 */
	members, err4 := c.rdsGroupChatRecord.HashGetAll("hash:" + strconv.FormatInt(inviteJoinChatGroup.ChatGroupId, 10))
	if nil != err4 {
		c.SendErrorCode(msg_err_code.MsgErrCode_send_invite_join_chat_group_get_chat_group_member_failed, 0)
		return
	}

	/* 通过 MQ 通知好友及其他群成员 */
	for index, member := range members {
		if 1 == index%2 {
			continue
		}

		/* 获取群成员Id（int64） */
		bytes := member.([]byte)
		strAccountId := string(bytes)
		accountId, _ := strconv.ParseInt(strAccountId, 10, 64)
		if c.GetAccountId() == accountId { // 不通知自己
			continue
		}

		for inviteeId, inviteeNickName := range mapInviteFriendSuccess {
			if err := redisPubSub.SendPbMsg(accountId, msg_id.NetMsgId_MQInviteJoinChatGroup, &msg_struct.MQInviteJoinChatGroup{
				InviteJoinChatGroup: &msg_struct.InviteJoinChatGroup{
					ChatGroupId:     inviteJoinChatGroup.ChatGroupId,
					InviterId:       c.GetAccountId(),
					InviterNickName: c.account.NickName,
					InviteeId:       inviteeId,
					InviteeNickName: inviteeNickName,
				},
			}); nil != err {
				log.Errorf("通知群成员%d，邀请者%d邀请其好友%d进群失败", member, c.GetAccountId(), inviteeId)
			}
		}
	}

	eventArg := eventmanager.NewEventArg()
	eventArg.SetInt64("groupid", inviteJoinChatGroup.ChatGroupId)
	eventArg.SetUserData("client", c)
	eventmanager.Publish(eventmanager.JoinChatGroupEvent, eventArg)

	c.SendPbMsg(msg_id.NetMsgId_S2CRecvInviteJoinChatGroup, &msg_struct.S2CRecvInviteJoinChatGroup{
		ChatGroupId: inviteJoinChatGroup.ChatGroupId,
	})
}

// C2SCancelChatGroup 群成员退群
func C2SCancelChatGroup(c *Client, msg []byte) {
	cancelChatGroup := &msg_struct.C2SCancelChatGroup{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], cancelChatGroup, "msg_struct.C2SCancelChatGroup") {
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SCancelChatGroup_has_error, 0)
		return
	}

	/* 校验群ID */
	chatGroup := &models.ChatGroup{}
	bGet, err := c.orm.Where("chat_group_id = ?", cancelChatGroup.ChatGroupId).Get(chatGroup)
	if nil != err || !bGet {
		c.SendErrorCode(msg_err_code.MsgErrCode_send_cancel_chat_group_group_id_error, 0)
		return
	}

	/* 校验用户是否是群成员 */
	if nil == GetChatGroupMember(c, cancelChatGroup.ChatGroupId, c.GetAccountId()) {
		c.SendErrorCode(msg_err_code.MsgErrCode_send_cancel_chat_group_is_not_member, 0)
		return
	}

	/* 校验退群用户是否是群主，群主只能解散群，不能退群 */
	if chatGroup.Creator == c.GetAccountId() {
		c.SendErrorCode(msg_err_code.MsgErrCode_send_cancel_chat_group_is_creator, 0)
		return
	}

	/* 从redis中删除群成员 */
	if !RemoveUserFromRedisGroupMember(c, c.GetAccountId(), cancelChatGroup.ChatGroupId) {
		log.Error("群成员退群时，从redis中移除群成员失败")
	}

	/* 从数据库中删除群成员 */
	if !RemoveUserFromDBGroupMember(c, c.GetAccountId(), cancelChatGroup.ChatGroupId) {
		log.Error("群成员退群时，从数据库中移除群成员失败")
	}

	RecordChatGroupMemberQuitLog(c.orm, chatGroup.ChatGroupId, c.GetAccountId(), chatGroup.Creator, GROUP_MEMBER_QUIT_CHAT_GROUP)

	group := &msg_struct.CancelChatGroup{
		ChatGroupId:  chatGroup.ChatGroupId,
		UserId:       c.GetAccountId(),
		UserNickName: c.account.NickName,
	}

	/* pulsar通知其他群成员退群消息 */
	members, err := GetRedisGroupMembers(c, cancelChatGroup.ChatGroupId)
	for index, member := range members {
		if 1 == index%2 {
			continue
		}

		memberId, _ := strconv.ParseInt(string(member.([]byte)), 10, 64)
		_ = redisPubSub.SendPbMsg(memberId, msg_id.NetMsgId_MQCancelChatGroup, &msg_struct.MQCancelChatGroup{
			CancelChatGroup: group,
		})
	}

	eventArg := eventmanager.NewEventArg()
	eventArg.SetInt64("accountid", c.GetAccountId())
	eventArg.SetInt64("groupid", cancelChatGroup.ChatGroupId)
	eventmanager.Publish(eventmanager.CancelChatGroupEvent, eventArg)

	// 回复客户端退群成功消息
	c.SendPbMsg(msg_id.NetMsgId_S2CRecvCancelChatGroup, &msg_struct.S2CRecvCancelChatGroup{
		ChatGroupId: chatGroup.ChatGroupId,
	})
}

// C2SUnChatGroup 群主解散群
func C2SUnChatGroup(c *Client, msg []byte) {
	unChatGroup := &msg_struct.C2SUnChatGroup{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], unChatGroup, "msg_struct.C2SUnChatGroup") {
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SUnChatGroup_has_error, 0)
		return
	}

	/* 校验群ID */
	chatGroup := &models.ChatGroup{}
	bGet, err := c.orm.Where("chat_group_id = ?", unChatGroup.ChatGroupId).Get(chatGroup)
	if nil != err || !bGet {
		c.SendErrorCode(msg_err_code.MsgErrCode_send_un_chat_group_group_id_error, 0)
		return
	}

	/* 校验解散群用户是否是群主 */
	if chatGroup.Creator != c.GetAccountId() {
		c.SendErrorCode(msg_err_code.MsgErrCode_send_un_chat_group_is_not_creator, 0)
		return
	}

	/* 从redis中获取所有群成员 */
	groupMembers, err := GetRedisGroupMembers(c, chatGroup.ChatGroupId)
	if nil != err {
		c.SendErrorCode(msg_err_code.MsgErrCode_send_un_chat_group_get_members_failed, 0)
		return
	}

	/* 从chat_group_member中删除群 */
	if !RemoveAllUserFromDB(c, chatGroup.ChatGroupId) {
		log.Errorf("群主%d解散群时，从数据库chat_group_member表删除群%d失败", c.GetAccountId(), chatGroup.ChatGroupId)
	}

	/* 从chat_group中删除群 */
	if !RemoveDBChatGroup(c, chatGroup.ChatGroupId) {
		log.Errorf("群主%d解散群时，从数据库chat_group表删除群%d数据失败", c.GetAccountId(), chatGroup.ChatGroupId)
	}

	/* 从redis中删除所有群成员 */
	if !RemoveAllUserFromRedisGroupMember(c, chatGroup.ChatGroupId) {
		log.Error("群主%d解散群时，从redis移除所有群%d成员失败", c.GetAccountId(), chatGroup.ChatGroupId)
	}

	/* 记录解散群日志 */
	if !RecordDestroyChatGroup(c, chatGroup) {
		log.Error("群主%d解散群时，数据库中记录群%d解散记录失败", c.GetAccountId(), chatGroup.ChatGroupId)
	}

	/* 通知所有群成员群解散消息 */
	for index, member := range groupMembers {
		if 1 == index%2 {
			continue
		}

		memberId, _ := strconv.ParseInt(string(member.([]byte)), 10, 64)
		RecordChatGroupMemberQuitLog(c.orm, chatGroup.ChatGroupId, memberId, chatGroup.Creator, GROUP_OWNER_UN_CHAT_GROUP)

		if c.GetAccountId() == memberId {
			continue
		}

		_ = redisPubSub.SendPbMsg(memberId, msg_id.NetMsgId_MQUnChatGroup, &msg_struct.MQUnChatGroup{
			ChatGroupId: chatGroup.ChatGroupId,
		})
	}

	eventArg := eventmanager.NewEventArg()
	eventArg.SetInt64("groupid", unChatGroup.ChatGroupId)
	eventmanager.Publish(eventmanager.UnChatGroupEvent, eventArg)

	// 回复客户端群主解散群消息
	c.SendPbMsg(msg_id.NetMsgId_S2CRecvUnChatGroup, &msg_struct.S2CRecvUnChatGroup{
		ChatGroupId: unChatGroup.ChatGroupId,
	})
}

// C2SChatGroupKick 群主踢人
func C2SChatGroupKick(c *Client, msg []byte) {
	chatGroupKick := &msg_struct.C2SChatGroupKick{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], chatGroupKick, "msg_struct.C2SChatGroupKick") {
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SChatGroupKick_has_error, 0)
		return
	}

	/* 校验群ID */
	chatGroup := &models.ChatGroup{}
	bGet, err := c.orm.Where("chat_group_id = ?", chatGroupKick.ChatGroupKick.ChatGroupId).Get(chatGroup)
	if nil != err || !bGet {
		c.SendErrorCode(msg_err_code.MsgErrCode_send_chat_group_kick_group_id_error, 0)
		return
	}

	/* 校验退群用户是否是群主 */
	if chatGroup.Creator != c.GetAccountId() {
		c.SendErrorCode(msg_err_code.MsgErrCode_send_chat_group_kick_is_not_creator, 0)
		return
	}

	/* 校验成员是否为群成员 */
	if nil == GetChatGroupMember(c, chatGroupKick.ChatGroupKick.ChatGroupId, chatGroupKick.ChatGroupKick.UserId) {
		c.SendErrorCode(msg_err_code.MsgErrCode_send_chat_group_user_is_not_member, 0)
		return
	}

	/* 获取群成员列表 */
	members, err := GetRedisGroupMembers(c, chatGroupKick.ChatGroupKick.ChatGroupId)
	if nil != err {
		c.SendErrorCode(msg_err_code.MsgErrCode_send_chat_group_kick_get_members_failed, 0)
		return
	}

	for index, member := range members {
		if 1 == index%2 {
			continue
		}

		memberId, _ := strconv.ParseInt(string(member.([]byte)), 10, 64)
		if memberId == c.GetAccountId() {
			continue
		}
		redisPubSub.SendPbMsg(memberId, msg_id.NetMsgId_MQChatGroupKick, &msg_struct.MQChatGroupKick{
			ChatGroupKick: chatGroupKick.ChatGroupKick,
		})
	}

	/* 从redis中删除群成员 */
	RemoveUserFromRedisGroupMember(c, chatGroupKick.ChatGroupKick.UserId, chatGroupKick.ChatGroupKick.ChatGroupId)

	/* 从chat_group_member中删除群成员 */
	RemoveUserFromDBGroupMember(c, chatGroupKick.ChatGroupKick.UserId, chatGroupKick.ChatGroupKick.ChatGroupId)

	/* 群主踢人信息记录到数据库 */
	RecordChatGroupMemberQuitLog(c.orm, chatGroup.ChatGroupId, chatGroupKick.ChatGroupKick.UserId, chatGroup.Creator, GROUP_OWNER_KICK_GROUP_MEMBER)

	eventArg := eventmanager.NewEventArg()
	eventArg.SetInt64("accountid", chatGroupKick.ChatGroupKick.UserId)
	eventArg.SetInt64("groupid", chatGroupKick.ChatGroupKick.ChatGroupId)
	eventmanager.Publish(eventmanager.CancelChatGroupEvent, eventArg)

	// 回复客户端群主踢人消息
	c.SendPbMsg(msg_id.NetMsgId_S2CRecvChatGroupKick, &msg_struct.S2CRecvChatGroupKick{
		ChatGroupId: chatGroupKick.ChatGroupKick.ChatGroupId,
	})
}

// C2SGroupChat 接收客户端发送的群聊消息
func C2SGroupChat(c *Client, msg []byte) {
	// 校验收到数据合法性
	/* 反序列化客户端发送的群消息 */
	groupChat := &msg_struct.C2SGroupChat{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], groupChat, "msg_struct.C2SGroupChat") {
		log.Errorf("反序列化群消息失败:%s", fmt.Sprintf("SenderId:=%d", c.GetAccountId()))
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SGroupChat_has_error, 0)
		return
	}

	if nil == groupChat.ChatMsg {
		log.Errorf("接收群消息msg_struct.C2SGroupChat.ChatMsg is nil,SenderId = %d", c.GetAccountId())
		c.SendErrorCode(msg_err_code.MsgErrCode_send_group_chat_msg_chat_msg_is_nil, 0)
		return
	}

	/* word文字类型，pic图片类型，voice音频 */
	if !("word" == groupChat.ChatMsg.MsgType || "pic" == groupChat.ChatMsg.MsgType || "voice" == groupChat.ChatMsg.MsgType || "video" == groupChat.ChatMsg.MsgType || "file" == groupChat.ChatMsg.MsgType || "card" == groupChat.ChatMsg.MsgType) {
		log.Errorf("合法性检测未通过，msg_struct.C2SGroupChat.MsgType不是预期值")
		c.SendErrorCode(msg_err_code.MsgErrCode_send_group_chat_msg_chat_msg_is_illege, 0)
		return
	}

	if nil == GetChatGroupInfoFromRedis(c, groupChat.ChatMsg.ReceiveGroupId) {
		log.Errorf("合法性检测未通过，msg_struct.C2SGroupChat.ReceiveGroupId不合法")
		c.SendErrorCode(msg_err_code.MsgErrCode_send_group_chat_msg_chat_msg_is_illege, 0)
		return
	}

	if nil == GetChatGroupMember(c, groupChat.ChatMsg.ReceiveGroupId, c.GetAccountId()) {
		log.Errorf("%d发送群Id%d群聊信息时，已经不是该群成员", c.GetAccountId(), groupChat.ChatMsg.ReceiveGroupId)
		c.SendErrorCode(msg_err_code.MsgErrCode_send_group_chat_sender_is_not_chat_group_member, 0)
		return
	}

	if "" == groupChat.ChatMsg.Text {
		log.Errorf("合法性检测未通过，msg_struct.C2SGroupChat.Text不合法")
		c.SendErrorCode(msg_err_code.MsgErrCode_send_group_chat_msg_chat_msg_is_illege, 0)
		return
	}

	// 至此，客户端消息验证通过
	groupChat.ChatMsg.SenderId = c.GetAccountId()
	groupChat.ChatMsg.SenderName = c.GetName()
	groupChat.ChatMsg.SrvTimeStamp = time.Now().UnixNano() / 1e3
	groupChat.ChatMsg.SenderheadPortraitUrl = c.GetUserHeadPortraitUrl(groupChat.ChatMsg.SenderId)

	if "voice" == groupChat.GetChatMsg().GetMsgType() {
		voiceMsg := groupChat.GetChatMsg().GetVoiceMessage()
		if nil == voiceMsg {
			log.Errorf("用户%d发送语音消息到群%d，但语音为空", groupChat.GetChatMsg().GetSenderId(), groupChat.GetChatMsg().GetReceiveGroupId())
			c.SendErrorCode(msg_err_code.MsgErrCode_send_group_chat_msg_type_not_match_message, 0)
			return
		}
		if "" == voiceMsg.GetAudioUrl() {
			log.Errorf("用户%d发送语音消息到群%d，但语音url为空", groupChat.GetChatMsg().GetSenderId(), groupChat.GetChatMsg().GetReceiveGroupId())
			c.SendErrorCode(msg_err_code.MsgErrCode_send_group_chat_msg_voice_url_is_empty, 0)
			return
		}
		if 60 < voiceMsg.GetAudioSecond() {
			log.Errorf("用户%d发送语音消息到群%d，但语音长度过长", groupChat.GetChatMsg().GetSenderId(), groupChat.GetChatMsg().GetReceiveGroupId())
			c.SendErrorCode(msg_err_code.MsgErrCode_send_group_chat_msg_voice_too_long, 0)
			return
		}
	}

	if "file" == groupChat.GetChatMsg().GetMsgType() {
		fileMessage := groupChat.GetChatMsg().GetFileMessage()
		if nil == fileMessage {
			log.Errorf("用户%d发送文件消息到群%d，但文件为空", groupChat.GetChatMsg().GetSenderId(), groupChat.GetChatMsg().GetReceiveGroupId())
			c.SendErrorCode(msg_err_code.MsgErrCode_send_group_chat_msg_type_not_match_message, 0)
			return
		}
		if "" == fileMessage.GetFileUrl() {
			log.Errorf("用户%d发送文件消息到群%d，但文件url为空", groupChat.GetChatMsg().GetSenderId(), groupChat.GetChatMsg().GetReceiveGroupId())
			c.SendErrorCode(msg_err_code.MsgErrCode_send_group_chat_msg_file_url_is_empty, 0)
			return
		}
		if "" == fileMessage.FileThumbnail {
			log.Errorf("用户%d发送文件消息到群%d，但文件缩略图url为空", groupChat.GetChatMsg().GetSenderId(), groupChat.GetChatMsg().GetReceiveGroupId())
			c.SendErrorCode(msg_err_code.MsgErrCode_send_group_chat_msg_file_thumbnail_url_is_empty, 0)
			return
		}
	}

	if "video" == groupChat.GetChatMsg().GetMsgType() {
		videoMessage := groupChat.GetChatMsg().GetVideoMessage()
		if nil == videoMessage {
			log.Errorf("用户%d发送视频消息到群%d，但视频为空", groupChat.GetChatMsg().GetSenderId(), groupChat.GetChatMsg().GetReceiveGroupId())
			c.SendErrorCode(msg_err_code.MsgErrCode_send_group_chat_msg_type_not_match_message, 0)
			return
		}
		if "" == videoMessage.VideoUrl {
			log.Errorf("用户%d发送视频消息到群%d，但视频为空", groupChat.GetChatMsg().GetSenderId(), groupChat.GetChatMsg().GetReceiveGroupId())
			c.SendErrorCode(msg_err_code.MsgErrCode_send_group_chat_msg_video_url_is_empty, 0)
			return
		}
		if "" == videoMessage.VideoThumbnail {
			log.Errorf("用户%d发送视频消息到群%d，但视频为空", groupChat.GetChatMsg().GetSenderId(), groupChat.GetChatMsg().GetReceiveGroupId())
			c.SendErrorCode(msg_err_code.MsgErrCode_send_group_chat_msg_video_thumbnail_url_is_empty, 0)
			return
		}
	}

	if "card" == groupChat.ChatMsg.MsgType {
		cardMessage := groupChat.ChatMsg.GetCardMessage()
		if nil == cardMessage {
			log.Errorf("用户%d发送名片消息到群%d，但名片为空", groupChat.ChatMsg.GetSenderId(), groupChat.ChatMsg.GetReceiveGroupId())
			c.SendErrorCode(msg_err_code.MsgErrCode_private_chat_card_message_is_empty, 0)
			return
		}
		headPortraitUrl := c.GetUserHeadPortraitUrl(cardMessage.Id)
		if "" != headPortraitUrl {
			groupChat.ChatMsg.CardMessage.Portrait = headPortraitUrl
		} else {
			log.Warnf("用户%d发送名片消息到好友%d，获取用户%d的头像失败，采用客户端上传的头像", c.GetAccountId(), groupChat.ChatMsg.GetReceiveGroupId(), cardMessage.Id)
		}
		briefInfo, _ := c.GetOtherBriefInfoFromRds(cardMessage.Id)
		if nil == briefInfo {
			log.Errorf("用户%d发送名片消息到好友%d，获取用户%d的昵称失败，采用客户端上传的昵称", c.GetAccountId(), groupChat.ChatMsg.GetReceiveGroupId(), cardMessage.Id)
		} else {
			groupChat.ChatMsg.CardMessage.NickName = briefInfo.Nickname
		}
	}

	/* 群消息序列化为buf */
	groupBuf, bSet := function.ProtoMarshal(groupChat.ChatMsg, "msg_struct.GroupMessage")
	if !bSet {
		log.Errorf("序列化群消息失败，SenderId = %d", groupChat.ChatMsg.GetSenderId())
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_marshal_msg_struct_GroupMessage_has_error, 0)
		return
	}
	rdsTxt := string(groupBuf)

	/* 消息存放到redis */
	if err := c.rdsGroupChatRecord.SortedSetAddSingle(
		strconv.FormatInt(groupChat.ChatMsg.ReceiveGroupId, 10),
		rdsTxt,
		strconv.FormatInt(groupChat.ChatMsg.SrvTimeStamp, 10),
	); nil != err {
		log.Errorf("群消息存入redis失败，SenderId = %d, error = %v", groupChat.ChatMsg.GetSenderId(), err)
		c.SendErrorCode(msg_err_code.MsgErrCode_send_group_chat_msg_chat_msg_store_redis_failed, 0)
		return
	}

	groupMembers, err := GetRedisGroupMembers(c, groupChat.ChatMsg.ReceiveGroupId)
	if nil != err {
		log.Errorf("%d发送群聊消息到群ID%d时，获取群成员列表失败", c.GetAccountId(), groupChat.ChatMsg.ReceiveGroupId)
		return
	}

	// 20191216 尹延注释下方代码（撤回消息，不设置时间限制）
	// 保存发出去的消息
	// key := c.GetStrAccount() + ":" + strconv.FormatInt(groupChat.ChatMsg.GetSrvTimeStamp(), 10)
	// c.chatRecord.Set(key, &rdsTxt, time.Minute*5)

	var iosReceiver, androidReceiver []string
	for index, groupMember := range groupMembers {
		if 1 == index%2 {
			continue
		}

		memberId, _ := strconv.ParseInt(string(groupMember.([]byte)), 10, 64)
		if IsUserOnline(c, memberId) {
			continue
		}

		deviceInfo := GetUserPushInfo(c, memberId)
		if nil == deviceInfo {
			log.Errorf("%d发送群ID%d群聊消息时,%d用户未上传推送信息，推送失败", c.GetAccountId(), groupChat.ChatMsg.ReceiveGroupId, memberId)
			continue
		}
		if "ios" == deviceInfo.DeviceProducter && "" != deviceInfo.DeviceInfo {
			iosReceiver = append(iosReceiver, deviceInfo.DeviceInfo)
		} else if "" != deviceInfo.DeviceInfo {
			androidReceiver = append(androidReceiver, deviceInfo.DeviceInfo)
		}
	}

	if nil != androidReceiver || nil != iosReceiver {
		if _, err := jPushService.Push(&jPushService.JPushServiceParam{
			AndroidReceiver: androidReceiver,
			IOSReceiver:     iosReceiver,
		}); nil != err {
			log.Errorf("%d发送群聊信息到群ID%d时，发送推送信息失败", c.GetAccountId(), groupChat.ChatMsg.ReceiveGroupId)
		}
	}

	// 向其他在线成员发送群聊消息
	eventArg := eventmanager.NewEventArg()
	eventArg.SetUserData("chatgroupmsg", groupChat.ChatMsg)
	eventmanager.Publish(eventmanager.C2SGroupChatEvent, eventArg)

	// 回复sender
	c.SendPbMsg(msg_id.NetMsgId_S2CGroupChat, &msg_struct.S2CGroupChat{
		ReceiverId:   groupChat.ChatMsg.ReceiveGroupId,
		SrvTimeStamp: groupChat.ChatMsg.SrvTimeStamp,
		CliTimeStamp: groupChat.CliTimeStamp,
	})
}

// C2SChatGroupLastMsgTimeStamp 客户端获取群最后一条消息记录时间戳
func C2SChatGroupLastMsgTimeStamp(c *Client, msg []byte) {
	reqMsg := &msg_struct.C2SChatGroupLastMsgTimeStamp{}
	if function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], reqMsg, "msg_struct.C2SChatGroupLastMsgTimeStamp") {
		chatGroupId := reqMsg.ChatGroupId
		lastMsgTimeStamp := reqMsg.SrvTimeStamp
		//当前时间//当前时间, 允许 10 秒的误差
		//current := time.Now().UnixNano() / 1e3
		// 此处+10s的目的：防止A 服务器和B 服务器之间出现时间差，出于容错性考虑，10秒的误差，咋个都能兼容机器时间误差，如果客户端上报的时间戳还是大于服务器，那么就认为客户端的错误
		current := time.Now().Add(time.Second*10).UnixNano() / 1e3

		var chatGroupLastMsgTimeStamp int64
		b, ts := c.GetChatGroupLastMsgTimestamp(chatGroupId)
		if b {
			chatGroupLastMsgTimeStamp = ts

			if false == (lastMsgTimeStamp < current && lastMsgTimeStamp > chatGroupLastMsgTimeStamp) {
				log.Errorf("account id: %d 上报收到的最后一条群 ( chatGroupId: %d )聊时间戳 %d 不合法, 应该 < 当前时间戳: %d, 并且 > chat_group_member.last_msg_timestamp: %d",
					c.account.AccountId,
					chatGroupId,
					lastMsgTimeStamp,
					current,
					chatGroupLastMsgTimeStamp)

				return
			}

			//更新client最后群聊消息时间
			c.SetChatGroupLastMsgTimestamp(chatGroupId, lastMsgTimeStamp)
			log.Tracef("update chat group last timestamp,account id:%d,current timestamp:%d,received timestamp:%d,saved timestamp:%d\n",
				c.account.AccountId,
				current,
				lastMsgTimeStamp,
				lastMsgTimeStamp)
		}
	}
}

// C2SGetOfflineGroupChatMsg 群成员获取群离线消息
func C2SGetOfflineGroupChatMsg(c *Client, msg []byte) {
	c2SGetOfflineGroupChatMsg := &msg_struct.C2SGetOfflineGroupChatMsg{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], c2SGetOfflineGroupChatMsg, "msg_struct.C2SGetOfflineGroupChatMsg") {
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SGetOfflineGroupChatMsg_has_error, 0)
		return
	}

	/* 校验群Id合法性 */
	if !c.rdsChatGroupInfo.Exists(strconv.FormatInt(c2SGetOfflineGroupChatMsg.ChatGroupId, 10)) {
		c.SendErrorCode(msg_err_code.MsgErrCode_send_get_offline_group_chat_msg_group_id_illegal, c2SGetOfflineGroupChatMsg.MsgTimestamp)
		return
	}

	/* 获取群离线消息 */
	chatGroupOfflineMsgs, err := c.rdsGroupChatRecord.SortedSetRangebyScore(
		strconv.FormatInt(c2SGetOfflineGroupChatMsg.ChatGroupId, 10),
		// 此处服务器做容错性处理
		// 比如A 服务器比B 服务器有时间误差，那么防止丢失消息，需要取客户端上传时间戳之前的时间来获取数据，并下发给客户端，以此来保证消息不会丢失
		// 此处提前10s
		fmt.Sprintf("(%d", c2SGetOfflineGroupChatMsg.ChatGroupLastMessageTimestamp-int64(time.Second*10/1e3)),
		"+inf")
	if nil != err {
		c.SendErrorCode(msg_err_code.MsgErrCode_send_get_offlien_group_chat_msg_get_offline_msg_failed, c2SGetOfflineGroupChatMsg.MsgTimestamp)
		return
	}

	/* 轮循离线消息并做处理 */
	s2CGetOfflineGroupChatMsg := &msg_struct.S2CGetOfflineGroupChatMsg{}
	s2CGetOfflineGroupChatMsg.ChatGroupId = c2SGetOfflineGroupChatMsg.ChatGroupId
	s2CGetOfflineGroupChatMsg.MsgTimestamp = c2SGetOfflineGroupChatMsg.MsgTimestamp
	for _, chatGroupOfflineMsg := range chatGroupOfflineMsgs {
		groupMessage := &msg_struct.GroupMessage{}
		if !function.ProtoUnmarshal(chatGroupOfflineMsg.([]byte), groupMessage, "msg_struct.GroupMessage") {
			log.Errorf("解析redis群离线消息失败:%v\n", chatGroupOfflineMsg.(string))
			continue
		}

		s2CGetOfflineGroupChatMsg.Msgs = append(s2CGetOfflineGroupChatMsg.Msgs, groupMessage)
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CGetOfflineGroupChatMsg, s2CGetOfflineGroupChatMsg)
}

// C2SWithdrawGroupMessage 群成员撤回群聊消息
func C2SWithdrawGroupMessage(c *Client, msg []byte) {
	withdrawGroupMessage := &msg_struct.C2SWithdrawGroupMessage{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], withdrawGroupMessage, "msg_struct.C2SWithdrawGroupMessage") {
		log.Errorf("用户%d撤回群聊消息时，反序列化 msg_struct.C2SWithdrawGroupMessage 失败", c.GetAccountId())
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SWithdrawGroupMessage_has_error, 0)
		return
	}

	inter, err := withDrawMsg(c, withdrawGroupMessage.GetServerTimeStamp(), 0, withdrawGroupMessage.GetGroupId())
	if nil != err {
		log.Errorf(err.Error())
		c.SendErrorCode(msg_err_code.MsgErrCode_withDraw_group_message_group_chat_withdraw_fail, 0)
		return
	}
	if nil == inter {
		log.Errorf("用户%d撤回群聊消息%d时，未返回错误，但接口为空")
		c.SendErrorCode(msg_err_code.MsgErrCode_withDraw_group_message_group_chat_withdraw_fail, 0)
		return
	}
	groupMessage := inter.(*msg_struct.GroupMessage)

	if c.GetAccountId() != groupMessage.GetSenderId() {
		log.Errorf("用户%d撤回群聊消息%d时，消息发送者%d不是消息撤回者%d",
			c.GetAccountId(), withdrawGroupMessage.GetServerTimeStamp(), groupMessage.GetSenderId(), c.GetAccountId())
		c.SendErrorCode(msg_err_code.MsgErrCode_withDraw_message_withDrawer_is_not_sender, 0)
		return
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CWithdrawGroupMessage, &msg_struct.S2CWithdrawGroupMessage{
		ServerTimeStamp: withdrawGroupMessage.GetServerTimeStamp(),
	})

	// 向其他在线成员发送群聊消息
	eventArg := eventmanager.NewEventArg()
	eventArg.SetUserData("chatgroupmsg", groupMessage)
	eventmanager.Publish(eventmanager.C2SGroupChatEvent, eventArg)
}

/* 群管理 redis相关操作 */

// InsertIntoRedisChatGroupInfo 建群时，将群相关信息插入到redis
func InsertIntoRedisChatGroupInfo(c *Client, chatGroup *models.ChatGroup) {
	strId := strconv.Itoa(int(chatGroup.ChatGroupId))
	p := &msg_struct.ChatGroup{}
	p.ChatGroupId = chatGroup.ChatGroupId
	p.Creator = chatGroup.Creator
	p.Name = chatGroup.Name
	p.Pic = string(chatGroup.Pic)
	buf, _ := proto.Marshal(p)
	c.rdsChatGroupInfo.Set(strId, string(buf))
}

// GetChatGroupInfoFromRedis 获取群相关信息
func GetChatGroupInfoFromRedis(c *Client, groupId int64) *msg_struct.ChatGroup {
	groupInfo, err := c.rdsChatGroupInfo.Get(strconv.FormatInt(groupId, 10))
	if nil != err {
		log.Errorf("%d获取群Id%d相关信息失败", c.GetAccountId(), groupId)
		return nil
	}

	chatGroup := &msg_struct.ChatGroup{}
	if !function.ProtoUnmarshal([]byte(groupInfo), chatGroup, "msg_struct.ChatGroup") {
		log.Errorf("%d获取群Id%d相关信息，protobuf解析失败", c.GetAccountId(), groupId)
		return nil
	}

	return chatGroup
}

// InsertIntoRedisGroupMembers 用户加入聊天群 userBriefInfo(key：用户ID，value：UserBriefInfo，允许一次性添加多个成员)
func InsertIntoRedisGroupMembers(c *Client, groupId int64, userBriefInfo map[string]interface{}) error {
	return c.rdsGroupChatRecord.HashMultipleSet("hash:"+strconv.FormatInt(groupId, 10), userBriefInfo)
}

// GetChatGroupMember 获取群某成员信息
func GetChatGroupMember(c *Client, groupId, userId int64) *msg_struct.ChatGroupMemberRds {
	hashSetField, err := c.rdsGroupChatRecord.GetHashSetField(
		"hash:"+strconv.FormatInt(groupId, 10),
		strconv.FormatInt(userId, 10))
	if nil != err {
		return nil
	}

	groupMemberRds := &msg_struct.ChatGroupMemberRds{}
	err = json.Unmarshal([]byte(hashSetField), groupMemberRds)
	if nil != err {
		return nil
	}

	return groupMemberRds
}

// GetRedisGroupMembers 获取所有群成员列表
func GetRedisGroupMembers(c *Client, groupId int64) ([]interface{}, error) {
	return c.rdsGroupChatRecord.HashGetAll("hash:" + strconv.FormatInt(groupId, 10))
}

// RemoveUserFromRedisGroupMember 从redis中移除群成员
func RemoveUserFromRedisGroupMember(c *Client, userId, groupId int64) bool {
	removeNum, err := c.rdsGroupChatRecord.DeleteHashSetField(
		"hash:"+strconv.FormatInt(groupId, 10),
		strconv.FormatInt(userId, 10))
	return nil == err && 1 == removeNum
}

// RemoveAllUserFromRedisGroupMember 从redis中移除某群所有群成员
func RemoveAllUserFromRedisGroupMember(c *Client, groupId int64) bool {
	_, err := c.rdsGroupChatRecord.Del("hash:" + strconv.FormatInt(groupId, 10))
	return nil == err
}

/* 群管理 数据库相关操作 */

// RemoveUserFromDBGroupMember 从数据库中移除群成员
func RemoveUserFromDBGroupMember(c *Client, userId, groupId int64) bool {
	_, err := c.orm.DB().Exec("DELETE FROM chat_group_member WHERE account_id = ? and chat_group_id = ?", userId, groupId)
	return nil == err
}

// RemoveDBChatGroup 从数据库chat_group中移除群
func RemoveDBChatGroup(c *Client, groupId int64) bool {
	_, err := c.orm.DB().Exec("DELETE FROM chat_group WHERE chat_group_id = ?", groupId)
	return nil == err
}

// RemoveAllUserFromDB 从数据库chat_group_member中移除群及群成员
func RemoveAllUserFromDB(c *Client, groupId int64) bool {
	_, err := c.orm.DB().Exec("DELETE FROM chat_group_member WHERE chat_group_id = ?", groupId)
	return nil == err
}

// RecordDestroyChatGroup 数据库中记录解散群日志
func RecordDestroyChatGroup(c *Client, group *models.ChatGroup) bool {
	insertNum, err := c.orm.Insert(&models.ChatGroupDestroyLog{
		GroupId:         group.ChatGroupId,
		Creater:         group.Creator,
		DestroyDatetime: time.Now(),
	})
	return nil == err && 1 == insertNum
}

// RecordChatGroupMemberQuitLog 数据库中记录群成员离开群日志，0：群主解散群；1：群成员主动退群；2：群主踢人
func RecordChatGroupMemberQuitLog(mysqlSession *xorm.Engine, chatGroupId, chatGroupMemberId, chatGroupOwnerId int64, flag int) {
	_, err := mysqlSession.Insert(&models.ChatGroupMemberQuitLog{
		ChatGroupId:      chatGroupId,
		ChatGroupOwnerId: chatGroupOwnerId,
		GroupMemberId:    chatGroupMemberId,
		Timestamp:        time.Now(),
		Flag:             flag,
	})
	if nil != err {
		log.Errorf("记录群成员%d离开群主为%d的群%d时失败:%v", chatGroupMemberId, chatGroupOwnerId, chatGroupId, err)
	}
}
