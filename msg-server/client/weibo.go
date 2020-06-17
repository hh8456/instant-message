package client

import (
	"errors"
	"fmt"
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"instant-message/common-library/models"
	"instant-message/common-library/packet"
	"instant-message/common-library/proto/msg_err_code"
	"instant-message/common-library/proto/msg_id"
	"instant-message/common-library/proto/msg_struct"
	"instant-message/common-library/snowflake"
	"instant-message/im-library/redisPubSub"
	"instant-message/msg-server/eventmanager"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gomodule/redigo/redis"
)

const (
	WEIBO_TEXT_MAX_LENTH  = 2048 // 朋友圈发送的文字最大长度
	WEIBO_PIC_MAX_LENTH   = 9    // 朋友圈图片的最多数目
	WEIBO_RADIO_MAX_LENTH = 2    // 朋友圈视频的最多数目
	WEIBO_REPLY_MAX_LENTH = 2048 // 朋友圈评论的最大长度
)

func init() {
	// 发朋友圈
	registerLogicCall(msg_id.NetMsgId_C2SPublishWeibo, C2SPublishWeibo)
	// 点赞朋友圈
	registerLogicCall(msg_id.NetMsgId_C2SLikeWeibo, C2SLikeWeibo)
	// 取消点赞朋友圈
	registerLogicCall(msg_id.NetMsgId_C2SCancelLikeWeibo, C2SCancelLikeWeibo)
	// 评论朋友圈
	registerLogicCall(msg_id.NetMsgId_C2SReplyWeibo, C2SReplyWeibo)
	// 朋友圈删除评论
	registerLogicCall(msg_id.NetMsgId_C2SDeleteReplyWeibo, C2SDeleteReplyWeibo)
	// 删除朋友圈, debug
	registerLogicCall(msg_id.NetMsgId_C2SDeleteWeibo, C2SDeleteWeibo)
	// 查看自己朋友圈
	registerLogicCall(msg_id.NetMsgId_C2SBrowseSomeoneWeibo, C2SBrowseSomeoneWeibo)
	// 用户查看自己和所有好友的朋友圈, debug
	registerLogicCall(msg_id.NetMsgId_C2SBrowseAllWeibo, C2SBrowseAllWeibo)
}

// C2SPublishWeibo 用户发朋友圈
func C2SPublishWeibo(c *Client, msg []byte) {
	publishWeibo := &msg_struct.C2SPublishWeibo{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], publishWeibo, "msg_struct.C2SPublishWeibo") {
		log.Errorf("反序列化用户%d发朋友圈报文失败", c.GetAccountId())
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SPublishWeibo_has_error, 0)
		return
	}

	/* 校验朋友圈类型 */
	if msg_id.MsgWeibo_name[int32(publishWeibo.ContentType)] == "" {
		log.Errorf("用户%d发朋友圈，朋友圈类型%d不合法", c.GetAccountId(), publishWeibo.ContentType)
		c.SendErrorCode(msg_err_code.MsgErrCode_publish_weibo_content_is_illegal, 0)
		return
	}

	if int64(msg_id.MsgWeibo_text_only) == publishWeibo.ContentType&int64(msg_id.MsgWeibo_text_only) {
		// 此处表示朋友圈类型含有文字
		if WEIBO_TEXT_MAX_LENTH < len(publishWeibo.Text) {
			log.Errorf("用户%d发朋友圈，文字长度%d超过限制%d", c.GetAccountId(), len(publishWeibo.Text), WEIBO_TEXT_MAX_LENTH)
			c.SendErrorCode(msg_err_code.MsgErrCode_publish_weibo_text_out_of_limit, 0)
			return
		}
	}

	if int64(msg_id.MsgWeibo_picture_only) == publishWeibo.ContentType&int64(msg_id.MsgWeibo_picture_only) {
		// 此处表示朋友圈类型含有图片
		if WEIBO_PIC_MAX_LENTH < len(publishWeibo.Url) || 0 >= len(publishWeibo.Url) {
			log.Errorf("用户%d发朋友圈，图片个数%d超过限制%d", c.GetAccountId(), len(publishWeibo.Url), WEIBO_PIC_MAX_LENTH)
			c.SendErrorCode(msg_err_code.MsgErrCode_publish_weibo_url_out_of_limit, 0)
			return
		}
	}

	if int64(msg_id.MsgWeibo_radio_only) == publishWeibo.ContentType&int64(msg_id.MsgWeibo_radio_only) {
		// 此处表示朋友圈类型含有视频
		if WEIBO_RADIO_MAX_LENTH < len(publishWeibo.Url) || 0 >= len(publishWeibo.Url) {
			log.Errorf("用户%d发朋友圈，视频个数%d超过限制%d", c.GetAccountId(), len(publishWeibo.Url), WEIBO_RADIO_MAX_LENTH)
			c.SendErrorCode(msg_err_code.MsgErrCode_publish_weibo_url_out_of_limit, 0)
			return
		}
	}

	/* 图片、视频url信息转换为二进制 */
	urls_bin, bRet := function.ProtoMarshal(&msg_struct.WeiboUrl{
		Urls: publishWeibo.Url,
	}, "msg_struct.WeiboUrl")
	if !bRet {
		log.Errorf("%d发朋友圈时，msg_struct.WeiboUrl protobuf 序列化失败", c.GetAccountId())
		c.SendErrorCode(msg_err_code.MsgErrCode_publish_weibo_server_internal_error, 0)
		return
	}

	timeStamp := time.Now().UnixNano() / 1e3

	weibo := &msg_struct.Weibo{
		WeiboId:           timeStamp,
		PublisherId:       c.GetAccountId(),
		PublisherNickName: c.GetName(),
		Type:              publishWeibo.ContentType,
		Text:              publishWeibo.Text,
		UrlBin:            urls_bin,
		SrvTimestamp:      timeStamp,
		BDelete:           false,
	}
	weibo_bin, bRet := function.ProtoMarshal(weibo, "msg_struct.Weibo")
	if !bRet {
		log.Errorf("%d发朋友圈时，msg_struct.Weibo protobuf序列化失败", c.GetAccountId())
		c.SendErrorCode(msg_err_code.MsgErrCode_publish_weibo_server_internal_error, 0)
		return
	}

	/* 朋友圈存入数据库 */
	if err := publishWeiboDB(c, weibo, time.Now()); nil != err {
		log.Errorf("%d用户发朋友圈时，存入数据库失败，err:%v", c.GetAccountId(), err)
		c.SendErrorCode(msg_err_code.MsgErrCode_publish_weibo_server_internal_error, 0)
		return
	}

	/* 朋友圈存入redis */
	if err := publishWeiboRds(c, weibo.WeiboId, weibo.SrvTimestamp, weibo_bin); nil != err {
		log.Errorf("%d用户发朋友圈时，存入redis失败，err:%v", c.GetAccountId(), err)
		c.SendErrorCode(msg_err_code.MsgErrCode_publish_weibo_server_internal_error, 0)
		return
	}

	/* 通过MQ发送消息到朋友 */
	friendIds := c.GetFriendIds()
	for _, friendId := range friendIds {
		weiboIdStr := strconv.FormatInt(weibo.GetWeiboId(), 10)
		if err := c.rdsWeiboZset.SortedSetAddSingle(strconv.FormatInt(friendId, 10), c.GetStrAccount()+":"+weiboIdStr, timeStamp); nil != err {
			log.Errorf("用户%d发朋友圈时，插入好友%d的 redis zset失败%v", c.GetAccountId(), friendId, err)
			continue
		}

		_ = redisPubSub.SendPbMsg(friendId, msg_id.NetMsgId_MQPublishWeibo, &msg_struct.MQPublishWeibo{
			Weibo: weibo,
		})
	}

	// TODO:此处需在onlineChatGroupManager.go文件处理
	eventArg := eventmanager.NewEventArg()
	eventArg.SetInt64("publishWeiboId", publishWeibo.WeiboId)
	eventArg.SetUserData("publishWeiboData", weibo)
	eventmanager.Publish(eventmanager.PublishWeibo, eventArg)

	/* 回复客户端，发朋友圈成功 */
	c.SendPbMsg(msg_id.NetMsgId_S2CRecvPublishWeibo, &msg_struct.S2CRecvPublishWeibo{
		WeiboId: weibo.WeiboId,
	})
}

// C2SDeleteWeibo 用户删除朋友圈
func C2SDeleteWeibo(c *Client, msg []byte) {
	deleteWeibo := &msg_struct.C2SDeleteWeibo{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], deleteWeibo, "msg_struct.C2SDeleteWeibo") {
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SDeleteWeibo_has_error, 0)
		return
	}

	/* 校验weiboId */
	weibo, err := getWeiboRds(c, c.GetAccountId(), deleteWeibo.WeiboId)
	if nil != err {
		c.SendErrorCode(msg_err_code.MsgErrCode_delete_weibo_weibo_id_is_error, 0)
		return
	}

	/* 删除朋友圈的用户必须是本人 */
	if c.GetAccountId() != weibo.PublisherId {
		c.SendErrorCode(msg_err_code.MsgErrCode_delete_weibo_weibo_id_is_error, 0)
		return
	}

	weibo.BDelete = true

	/* 删除朋友圈，存入数据库 */
	err = deleteWeiboDB(c, weibo, time.Now())
	if nil != err {
		c.SendErrorCode(msg_err_code.MsgErrCode_delete_weibo_server_internal_error, 0)
		return
	}

	inters, err := getLikeAndReplyWeiboRdsIds(c, weibo.WeiboId, c.GetAccountId())
	if nil != err {
		log.Errorf("用户%d删除朋友圈%d，获取朋友圈评论和点赞的好友失败%v", err)
		return
	}

	/* 删除朋友圈，存入redis */
	err = deleteWeiboRds(c, weibo)
	if nil != err {
		c.SendErrorCode(msg_err_code.MsgErrCode_delete_weibo_server_internal_error, 0)
		return
	}

	// 通知点赞和评论此条朋友圈的好友
	for _, inter := range inters.([]interface{}) {
		likeAndReplyWeiboRdsId, _ := strconv.ParseInt(string(inter.([]byte)), 10, 64)
		if !c.IsFriend(likeAndReplyWeiboRdsId) {
			continue
		}

		_ = redisPubSub.SendPbMsg(likeAndReplyWeiboRdsId, msg_id.NetMsgId_MQDeleteWeibo, &msg_struct.MQDeleteWeibo{
			FriendId: weibo.PublisherId,
			WeiboId:  weibo.WeiboId,
		})
	}

	/* 回复客户端，此条朋友圈已删除 */
	c.SendPbMsg(msg_id.NetMsgId_S2CRecvDeleteWeibo, &msg_struct.S2CRecvDeleteWeibo{
		Ret: "Success",
	})
}

// C2SLikeWeibo 用户点赞朋友圈
func C2SLikeWeibo(c *Client, msg []byte) {
	likeWeibo := &msg_struct.C2SLikeWeibo{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], likeWeibo, "msg_struct.C2SLikeWeibo") {
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SLickWeibo_has_error, 0)
		return
	}

	// TODO: 先暂时从mysql增删改查，完成主体功能之后，在优化查询时，加入mysql从库，从mysql从库进行查询功能
	weibo, err := getWeiboDB(c, likeWeibo.WeiboId)
	if nil != err {
		c.SendErrorCode(msg_err_code.MsgErrCode_like_weibo_weibo_id_is_error, 0)
		return
	}

	if c.GetAccountId() != likeWeibo.WeiboPublisherId && !c.IsFriend(likeWeibo.WeiboPublisherId) {
		c.SendErrorCode(msg_err_code.MsgErrCode_like_weibo_weibo_liker_publisher_is_not_friend, 0)
		return
	}

	timeNow := time.Now()
	timeStampNow := timeNow.UnixNano() / 1e3

	/* 点赞朋友圈存入数据库 */
	if err := likeWeiboDB(c, weibo, timeNow); nil != err {
		c.SendErrorCode(msg_err_code.MsgErrCode_like_weibo_server_internal_error, 0)
		return
	}

	/* 点赞朋友圈存入redis */
	if err := likeWeiboRds(c, weibo, timeStampNow); nil != err {
		c.SendErrorCode(msg_err_code.MsgErrCode_like_weibo_server_internal_error, 0)
		return
	}

	/* 通过MQ转发给其他玩家点赞信息 */
	inters, err := getLikeAndReplyWeiboRdsIds(c, weibo.WeiboId, weibo.AccountId)
	if nil != err {
		c.SendErrorCode(msg_err_code.MsgErrCode_like_weibo_server_internal_error, 0)
		return
	}

	for _, inter := range inters.([]interface{}) {
		likeAndReplyWeiboRdsId, _ := strconv.ParseInt(string(inter.([]byte)), 10, 64)
		if !c.IsFriend(likeAndReplyWeiboRdsId) {
			continue
		}

		_ = redisPubSub.SendPbMsg(likeAndReplyWeiboRdsId, msg_id.NetMsgId_MQLikeWeibo, &msg_struct.MQLikeWeibo{
			LikeWeibo: &msg_struct.LikeWeibo{
				WeiboId:          weibo.WeiboId,
				WeiboPublisherId: likeWeibo.WeiboPublisherId,
				LikerId:          c.GetAccountId(),
			},
		})
	}

	_ = redisPubSub.SendPbMsg(weibo.AccountId, msg_id.NetMsgId_MQLikeWeibo, &msg_struct.MQLikeWeibo{
		LikeWeibo: &msg_struct.LikeWeibo{
			WeiboId:          weibo.WeiboId,
			WeiboPublisherId: likeWeibo.WeiboPublisherId,
			LikerId:          c.GetAccountId(),
		},
	})

	// TODO:此处需在onlineChatGroupManager.go文件处理
	//eventArg := eventmanager.NewEventArg()
	//eventArg.SetInt64("publishWeiboId", publishWeibo.WeiboId)
	//eventArg.SetUserData("publishWeiboData", weibo)
	//eventmanager.Publish(eventmanager.PublishWeibo, eventArg)

	/* 回复客户端，点赞朋友圈信息成功 */
	c.SendPbMsg(msg_id.NetMsgId_S2CRecvLikeWeibo, &msg_struct.S2CRecvLikeWeibo{
		Ret: "Success",
	})
}

// C2SCancelLikeWeibo 用户取消点赞朋友圈
func C2SCancelLikeWeibo(c *Client, msg []byte) {
	cancelLikeWeibo := &msg_struct.C2SCancelLikeWeibo{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], cancelLikeWeibo, "msg_struct.C2SCancelLikeWeibo") {
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SCancelLikeWeibo_has_error, 0)
		return
	}

	weibo, err := getWeiboRds(c, cancelLikeWeibo.WeiboPublisherId, cancelLikeWeibo.WeiboId)
	if nil != err {
		c.SendErrorCode(msg_err_code.MsgErrCode_cancel_like_weibo_weibo_id_is_error, 0)
		return
	}

	/* 校验用户是否为此条朋友圈点过赞 */
	inters, err := getLikeWeiboRdsIds(c, cancelLikeWeibo.WeiboPublisherId, cancelLikeWeibo.WeiboId)
	if nil != err {
		c.SendErrorCode(msg_err_code.MsgErrCode_cancel_like_weibo_server_internal_error, 0)
		return
	}

	bLiked := false
	for _, inter := range inters {
		if string(inter.([]byte)) == c.GetStrAccount() {
			bLiked = true
			break
		}
	}
	if !bLiked {
		c.SendErrorCode(msg_err_code.MsgErrCode_cancel_like_weibo_server_internal_error, 0)
		return
	}

	/* 从redis删除朋友圈点赞信息 */
	if err = cancelLikeWeiboRds(c, weibo); nil != err {
		c.SendErrorCode(msg_err_code.MsgErrCode_cancel_like_weibo_server_internal_error, 0)
		return
	}

	/* 从数据库删除朋友圈点赞信息 */
	if err = cancelLikeWeiboDB(c, weibo, time.Now()); nil != err {
		c.SendErrorCode(msg_err_code.MsgErrCode_cancel_like_weibo_server_internal_error, 0)
		return
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CCancelLikeWeibo, &msg_struct.S2CCancelLikeWeibo{
		Ret: "Success",
	})
}

// C2SReplyWeibo 用户评论朋友圈
func C2SReplyWeibo(c *Client, msg []byte) {
	replyWeibo := &msg_struct.C2SReplyWeibo{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], replyWeibo, "msg_struct.C2SReplyWeibo") {
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SReplyWeibo_has_error, 0)
		return
	}

	/* 校验参数 */
	weibo, err := getWeiboRds(c, replyWeibo.ReplyWeibo.WeiboPublisherId, replyWeibo.ReplyWeibo.WeiboId)
	if nil != err {
		c.SendErrorCode(msg_err_code.MsgErrCode_reply_weibo_weibo_id_is_error, 0)
		return
	}

	/* 校验发评论的用户和发朋友圈的用户(或者是被评论的评论发布者)，是否为好友关系 */
	replyWeibo.ReplyWeibo.CommentatorId = c.GetAccountId()
	if !c.IsFriend(replyWeibo.ReplyWeibo.CommentatorId) && c.GetAccountId() != replyWeibo.ReplyWeibo.CommentatorId &&
		!c.IsFriend(replyWeibo.ReplyWeibo.BeCommentatorId) {
		c.SendErrorCode(msg_err_code.MsgErrCode_reply_weibo_commentor_is_not_becommentor_friend, 0)
		return
	}

	/* 校验评论长度 */
	if WEIBO_REPLY_MAX_LENTH < len(replyWeibo.ReplyWeibo.CommentContent) && 0 >= len(replyWeibo.ReplyWeibo.CommentContent) {
		c.SendErrorCode(msg_err_code.MsgErrCode_reply_weibo_comment_content_is_too_long, 0)
		return
	}

	replyWeibo.ReplyWeibo.CommentedId = snowflake.GetSnowflakeId()
	replyWeibo.ReplyWeibo.CommentTimeStamp = time.Now().UnixNano() / 1e3

	/* 评论朋友圈，存入数据库 */
	if err := replyWeiboDB(c, weibo, replyWeibo.ReplyWeibo, time.Now()); nil != err {
		c.SendErrorCode(msg_err_code.MsgErrCode_reply_weibo_server_internal_error, 0)
		return
	}

	/* 评论朋友圈，存入redis */
	if err := replyWeiboRds(c, weibo, replyWeibo.ReplyWeibo); nil != err {
		c.SendErrorCode(msg_err_code.MsgErrCode_reply_weibo_server_internal_error, 0)
		return
	}

	/* 通过MQ转发给其他玩家评论信息 */
	inters, err := getLikeAndReplyWeiboRdsIds(c, weibo.WeiboId, weibo.PublisherId)
	if nil != err {
		c.SendErrorCode(msg_err_code.MsgErrCode_like_weibo_server_internal_error, 0)
		return
	}

	likeAndReplyWeiboIds := inters.([]interface{})
	for _, inter := range likeAndReplyWeiboIds {
		likeAndReplyWeiboRdsId, _ := strconv.ParseInt(string(inter.([]byte)), 10, 64)
		if !c.IsFriend(likeAndReplyWeiboRdsId) {
			continue
		}

		isFriend, _ := c.rdsFriend.IsSetMember(strconv.FormatInt(likeAndReplyWeiboRdsId, 10), c.GetStrAccount())
		if 0 == isFriend {
			continue
		}

		if likeAndReplyWeiboRdsId != weibo.PublisherId {
			_ = redisPubSub.SendPbMsg(likeAndReplyWeiboRdsId, msg_id.NetMsgId_MQReplyWeibo, &msg_struct.MQReplyWeibo{
				ReplyWeibo: replyWeibo.ReplyWeibo,
			})
		}
	}

	_ = redisPubSub.SendPbMsg(weibo.PublisherId, msg_id.NetMsgId_MQReplyWeibo, &msg_struct.MQReplyWeibo{
		ReplyWeibo: replyWeibo.ReplyWeibo,
	})

	// TODO:此处需在onlineChatGroupManager.go文件处理
	//eventArg := eventmanager.NewEventArg()
	//eventArg.SetInt64("publishWeiboId", publishWeibo.WeiboId)
	//eventArg.SetUserData("publishWeiboData", weibo)
	//eventmanager.Publish(eventmanager.PublishWeibo, eventArg)

	c.SendPbMsg(msg_id.NetMsgId_S2CRecvReplyWeibo, &msg_struct.S2CRecvReplyWeibo{
		CommentId: replyWeibo.ReplyWeibo.CommentedId,
	})
}

// C2SDeleteReplyWeibo 用户删除朋友圈评论
func C2SDeleteReplyWeibo(c *Client, msg []byte) {
	deleteWeiboReply := &msg_struct.C2SDeleteWeiboReply{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], deleteWeiboReply, "msg_struct.C2SDeleteWeiboReply") {
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SDeleteWeiboReply_has_error, 0)
		return
	}

	/* 校验参数 */
	weibo, err := getWeiboRds(c, deleteWeiboReply.WeiboPublisherId, deleteWeiboReply.WeiboId)
	if nil != err {
		c.SendErrorCode(msg_err_code.MsgErrCode_delete_weibo_reply_weibo_id_is_error, 0)
		return
	}

	bDeleted := false
	replyWeiboBytes, err := getReplyWeiboRds(c, weibo)
	for _, replyWeiboByte := range replyWeiboBytes {
		replyWeibo := &msg_struct.ReplyWeibo{}
		if !function.ProtoUnmarshal(replyWeiboByte, replyWeibo, "msg_struct.ReplyWeibo") {
			log.Errorf("用户%d删除朋友圈%d评论%d，反序列化 msg_struct.ReplyWeibo 错误：%v", c.GetAccountId(), deleteWeiboReply.WeiboId, deleteWeiboReply.CommentId)
			continue
		}

		if replyWeibo.WeiboId == deleteWeiboReply.WeiboId && replyWeibo.CommentedId == deleteWeiboReply.CommentId {
			bDeleted = true

			/* 从数据库删除用户评论 */
			err := deleteWeiboReplyDB(c, weibo, deleteWeiboReply, time.Now())
			if nil != err {
				log.Errorf("用户%d删除朋友圈%d评论%d，从数据库删除失败%v", c.GetAccountId(), weibo.WeiboId, deleteWeiboReply.CommentId, err)
				break
			}

			/* 从redis删除用户评论 */
			err = deleteWeiboSomeoneReplyRds(c, weibo, replyWeiboByte)
			if nil != err {
				log.Errorf("用户%d删除朋友圈%d评论%d，从redis删除失败%v", c.GetAccountId(), weibo.WeiboId, deleteWeiboReply.CommentId, err)
				break
			}

			break
		}
	}

	if !bDeleted {
		// c.SendPbMsg()
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CDeleteReplyWeibo, &msg_struct.S2CRecvDeleteWeibo{
		Ret: "Success",
	})
}

// C2SBrowseSomeoneWeibo 用户查看某人朋友圈
func C2SBrowseSomeoneWeibo(c *Client, msg []byte) {
	browseSomeoneWeibo := &msg_struct.C2SBrowseSomeoneWeibo{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], browseSomeoneWeibo, "msg_struct.C2SBrowseSomeoneWeibo") {
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SBrowseOwnWeibo_has_error, 0)
		return
	}

	weiboIds := make([]interface{}, 0)
	/* 先判断好友关系 */
	beFriend, _ := c.rdsFriend.IsSetMember(strconv.FormatInt(browseSomeoneWeibo.FriendId, 10), c.GetStrAccount())
	if 0 == beFriend && browseSomeoneWeibo.FriendId != c.GetAccountId() {
		/* 判断是不是允许好友访问 */
		if DoesAllowStrangerLookWeibo(c, browseSomeoneWeibo.FriendId) {
			weiboIds, _ = c.rdsWeiboZset.SortedSetRangebyScore(strconv.FormatInt(browseSomeoneWeibo.FriendId, 10), fmt.Sprintf("(%d", browseSomeoneWeibo.LastTimeStamp), "+inf")
		}
	} else {
		if 0 == browseSomeoneWeibo.LastTimeStamp {
			weiboIds, _ = c.rdsWeiboZset.SortedSetRangebyScore(strconv.FormatInt(browseSomeoneWeibo.FriendId, 10), fmt.Sprintf("(%d", browseSomeoneWeibo.LastTimeStamp), "+inf")
		} else {
			weiboIds, _ = c.rdsWeiboZset.SortedSetRangebyScore(strconv.FormatInt(browseSomeoneWeibo.FriendId, 10), "-inf", fmt.Sprintf("(%d", browseSomeoneWeibo.LastTimeStamp))
		}
	}

	weiboIdsBack := make([]interface{}, 0)
	lenth := len(weiboIds)
	for index := 0; index < lenth; index++ {
		weiboIdsBack = append(weiboIdsBack, weiboIds[lenth-index-1])
	}

	strategy, allowTime, _ := GetFriendLookStrategy(c, browseSomeoneWeibo.FriendId)

	browseWeibos := make([]*msg_struct.BrowseWeibo, 0)
	if DoesAllowFriendLookMyWeiboRds(c, browseSomeoneWeibo.FriendId, c.GetAccountId()) {
		for _, v := range weiboIdsBack {
			browseWeibo := &msg_struct.BrowseWeibo{}
			/* 朋友圈信息 */
			weiboIdStr := string(v.([]byte))
			weiboBin, err := c.rdsWeiboString.Get(weiboIdStr)
			if nil != err {
				continue
			}

			weiboPublisherId, _ := strconv.ParseInt(weiboIdStr[:strings.Index(weiboIdStr, ":")], 10, 64)
			if weiboPublisherId != browseSomeoneWeibo.FriendId {
				continue
			}

			weibo := &msg_struct.Weibo{}
			if !function.ProtoUnmarshal([]byte(weiboBin), weibo, "msg_struct.Weibo") {
				continue
			}

			/* 好友朋友圈策略 */
			if 0 != allowTime && weibo.WeiboId < time.Now().UnixNano()/1e3-allowTime*1e6 {
				continue
			}

			browseWeibo.Weibo = weibo

			/* 朋友圈点赞详情 */
			inters, err := getLikeWeiboRdsIds(c, weibo.PublisherId, weibo.WeiboId)
			if nil != err {
				log.Errorf("用户%d查看%d的朋友圈%d时，获取所有点赞id出错%v", c.GetAccountId(), weibo.PublisherId, weibo.WeiboId, err)
				continue
			}

			for _, inter := range inters {
				likeWeiboId, err := strconv.ParseInt(string(inter.([]byte)), 10, 64)
				if nil == err && (c.IsFriend(likeWeiboId) || c.GetAccountId() == likeWeiboId) {
					browseWeibo.LikerId = append(browseWeibo.LikerId, likeWeiboId)
				}
			}

			/* 朋友圈评论详情 */
			replyWeibos, err := getReplyWeiboRds(c, weibo)
			if nil != err {
				log.Errorf("用户%d查看%d的朋友圈%d时，获取所有评论出错%v", c.GetAccountId(), weibo.PublisherId, weibo.WeiboId, err)
				continue
			}
			for _, replyWeiboBytes := range replyWeibos {
				replyWeibo := &msg_struct.ReplyWeibo{}
				if !function.ProtoUnmarshal(replyWeiboBytes, replyWeibo, "msg_struct.ReplyWeibo") {
					log.Errorf("用户%d查看%d的朋友圈%d时，序列化评论出错", c.GetAccountId(), weibo.PublisherId, weibo.WeiboId)
					continue
				}
				browseWeibo.ReplayWeibo = append(browseWeibo.ReplayWeibo, replyWeibo)
			}

			browseWeibos = append(browseWeibos, browseWeibo)

			/* 陌生人朋友圈策略 */
			if browseSomeoneWeibo.FriendId != c.GetAccountId() && !c.IsFriend(browseSomeoneWeibo.FriendId) {
				if STRANGER_LOOK_WEIBO_NUM <= len(browseWeibos) {
					break
				}
			}
		}
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CBrowseSomeoneWeibo, &msg_struct.S2CBrowseSomeoneWeibo{
		BrowseWeibo: browseWeibos,
		Strategy:    strategy,
		Ret:         "Success",
	})
}

// C2SBrowseAllWeibo 用户查看自己和所有好友的朋友圈
func C2SBrowseAllWeibo(c *Client, msg []byte) {
	browseAllWeibo := &msg_struct.C2SBrowseAllWeibo{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], browseAllWeibo, "msg_struct.C2SBrowseAllWeibo") {
		return
	}

	weiboIds := make([]interface{}, 0)
	if 0 == browseAllWeibo.LastMsgTimestamp {
		weiboIds, _ = c.rdsWeiboZset.SortedSetRangebyScore(c.GetStrAccount(), fmt.Sprintf("(%d", browseAllWeibo.LastMsgTimestamp), "+inf")
	} else {
		weiboIds, _ = c.rdsWeiboZset.SortedSetRangebyScore(c.GetStrAccount(), "-inf", fmt.Sprintf("(%d", browseAllWeibo.LastMsgTimestamp))
	}

	weiboIdsBack := make([]interface{}, 0)
	lenth := len(weiboIds)
	for index := 0; index < lenth; index++ {
		weiboIdsBack = append(weiboIdsBack, weiboIds[lenth-index-1])
	}

	browseWeibos := make([]*msg_struct.BrowseWeibo, 0)
	for _, v := range weiboIdsBack {
		browseWeibo := &msg_struct.BrowseWeibo{}
		/* 朋友圈信息 */
		weiboIdStr := string(v.([]byte))
		weiboBin, err := c.rdsWeiboString.Get(weiboIdStr)
		if nil != err {
			continue
		}

		weibo := &msg_struct.Weibo{}
		if !function.ProtoUnmarshal([]byte(weiboBin), weibo, "msg_struct.Weibo") {
			continue
		}
		browseWeibo.Weibo = weibo

		if weibo.PublisherId != c.GetAccountId() {
			if !c.IsFriend(weibo.PublisherId) {
				continue
			}

			if c.IsBlackList(weibo.PublisherId) {
				continue
			}

			isMyBlackList, _ := IsSomeoneInOtherBlackList(c, c.GetAccountId(), weibo.PublisherId)
			if isMyBlackList {
				continue
			}

			if !DoesLookFriendWeiboRds(c, c.GetAccountId(), weibo.PublisherId) {
				continue
			}

			if !DoesAllowFriendLookMyWeiboRds(c, weibo.PublisherId, c.GetAccountId()) {
				continue
			}

			/* 判断此条朋友圈在不在好友允许查看的时间范围内 */
			_, allowTimeStamp, _ := GetFriendLookStrategy(c, weibo.PublisherId)
			if 0 != allowTimeStamp && weibo.WeiboId < time.Now().UnixNano()/1e3-allowTimeStamp*1000*1000 {
				continue
			}
		}

		/* 点赞朋友圈 */
		likeWeiboRdsIds, err := getLikeWeiboRdsIds(c, weibo.PublisherId, weibo.WeiboId)
		if nil != err {
			continue
		}
		likerIds := make([]int64, 0)
		for _, likeWeiboId := range likeWeiboRdsIds {
			likerId, _ := strconv.ParseInt(string(likeWeiboId.([]byte)), 10, 64)
			likerIds = append(likerIds, likerId)
		}
		browseWeibo.LikerId = likerIds

		/* 评论朋友圈 */
		weiboReplys := make([]*msg_struct.ReplyWeibo, 0)
		replyWeibos, err := getReplyWeiboRds(c, weibo)
		if nil != err {
			continue
		}
		for _, replyWeiboBytes := range replyWeibos {
			replyWeibo := &msg_struct.ReplyWeibo{}
			if !function.ProtoUnmarshal(replyWeiboBytes, replyWeibo, "msg_struct.ReplyWeibo") {
				log.Errorf("用户%d查看所有人的朋友圈%d时，序列化评论出错", c.GetAccountId(), weibo.PublisherId, weibo.WeiboId)
				continue
			}
			weiboReplys = append(weiboReplys, replyWeibo)
		}
		browseWeibo.ReplayWeibo = weiboReplys

		browseWeibos = append(browseWeibos, browseWeibo)
		if 12 == len(browseWeibos) {
			break
		}
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CBrowseAllWeibo, &msg_struct.S2CBrowseAllWeibo{
		BrowseWeibo: browseWeibos,
		Ret:         "Success",
	})
}

/* 非通信内容 */
/* 数据库 */
// publishWeiboDB 发朋友圈，存入数据库
func publishWeiboDB(c *Client, weibo *msg_struct.Weibo, dateTime time.Time) error {
	// 事务开始
	tx := c.orm.NewSession()
	defer tx.Close()
	err := tx.Begin()
	if err != nil || tx == nil {
		log.Errorf("%d点赞朋友圈id%d时，开始数据库事务失败,error:%v", c.GetAccountId(), weibo.WeiboId, err)
		return err
	}

	var errCode int32
	defer func() {
		if errCode != 0 {
			if tx != nil {
				e := tx.Rollback()
				if e != nil {
					log.Errorf("%d点赞朋友圈id%d时，回滚事务 tx.Rollback() 出现严重错误: %v", c.GetAccountId(), weibo.WeiboId, e)
				}
			}
		}
	}()

	// 插入 weibo 表
	if _, err := tx.Insert(&models.Weibo{
		AccountId:   c.GetAccountId(),
		PhotosBin:   weibo.UrlBin,
		MsgContent:  weibo.Text,
		WeiboId:     weibo.WeiboId,
		MsgDatetime: dateTime,
	}); nil != err {
		errCode = 1
		log.Errorf("%d发朋友圈时，存入weibo表失败：%s", c.GetAccountId(), err)
		return err
	}

	// 插入 weibo_action_log 表
	if _, err := tx.Insert(&models.WeiboActionLog{
		AccountId:      c.GetAccountId(),
		WeiboId:        weibo.WeiboId,
		Action:         int(msg_id.MsgActionWeibo_publish_weibo),
		FriendId:       c.GetAccountId(),
		ActionDatetime: dateTime,
	}); nil != err {
		errCode = 2
		log.Errorf("%d发朋友圈时，存入weibo_action_log表失败：%s", c.GetAccountId(), err)
		return err
	}

	// 事务结束
	err = tx.Commit()
	if err != nil {
		errCode = 3
		log.Errorf("%d发朋友圈时, 提交事务 tx.Commit() 出现严重错误: %v", c.GetAccountId(), err)
	}

	return nil
}

// getWeiboDB 从数据库获取朋友圈
func getWeiboDB(c *Client, weiboId int64) (*models.Weibo, error) {
	weibo := &models.Weibo{}
	_, err := c.orm.Where("weibo_id = ?", weiboId).Get(weibo)
	if nil != err {
		return nil, err
	}

	return weibo, nil
}

// likeWeiboDB 点赞朋友圈存入数据库
func likeWeiboDB(c *Client, weibo *models.Weibo, likeTime time.Time) error {
	// 事务开始
	tx := c.orm.NewSession()
	defer tx.Close()
	err := tx.Begin()
	if err != nil || tx == nil {
		log.Errorf("%d点赞朋友圈id%d时，开始数据库事务失败,error:%v", c.GetAccountId(), weibo.WeiboId, err)
		return err
	}

	var errCode int32
	defer func() {
		if errCode != 0 {
			if tx != nil {
				e := tx.Rollback()
				if e != nil {
					log.Errorf("%d点赞朋友圈id%d时，回滚事务 tx.Rollback() 出现严重错误: %v", c.GetAccountId(), weibo.WeiboId, e)
				}
			}
		}
	}()

	/* 插入 weibo_action_log 表 */
	_, err = tx.Insert(&models.WeiboActionLog{
		AccountId:      c.GetAccountId(),
		WeiboId:        weibo.WeiboId,
		Action:         int(msg_id.MsgActionWeibo_like_weibo),
		FriendId:       weibo.AccountId,
		ActionDatetime: likeTime,
	})
	if nil != err {
		errCode = 1
		log.Errorf("%d点赞朋友圈id%d时，插入表 weibo_action_log 失败:%v", c.GetAccountId(), weibo.WeiboId, err)
		return err
	}

	/* 插入 weibo_like 表 */
	_, err = tx.Insert(&models.WeiboLike{
		AccountId:        c.GetAccountId(),
		WeiboId:          weibo.WeiboId,
		LikerId:          c.GetAccountId(),
		ThumbupTimestamp: likeTime,
	})
	if nil != err {
		errCode = 2
		log.Errorf("%d点赞朋友圈id%d时，插入表 weibo_like 失败:%v", c.GetAccountId(), weibo.WeiboId, err)
		return err
	}

	/* 更新 weibo 表 点赞次数 */
	_, err = tx.DB().Exec("UPDATE weibo SET thumbup_times = thumbup_times + 1 WHERE weibo_id = ?", weibo.WeiboId)
	if nil != err {
		errCode = 3
		log.Errorf("%d点赞朋友圈id%d时，更新表 weibo thumbup_times 失败:%v", c.GetAccountId(), weibo.WeiboId, err)
		return err
	}

	// 事务结束
	err = tx.Commit()
	if err != nil {
		errCode = 4
		log.Errorf("%d点赞朋友圈id%d时，提交事务 tx.Commit() 出现严重错误: %v", c.GetAccountId(), weibo.WeiboId, err)
		return err
	}

	return nil
}

// cancelLikeWeiboDB 取消点赞朋友圈，存入数据库
func cancelLikeWeiboDB(c *Client, weibo *msg_struct.Weibo, unlikeTime time.Time) error {
	// 事务开始
	tx := c.orm.NewSession()
	defer tx.Close()
	err := tx.Begin()
	if err != nil || tx == nil {
		log.Errorf("%d取消点赞朋友圈id%d时，开始数据库事务失败,error:%v", c.GetAccountId(), weibo.WeiboId, err)
		return err
	}

	var errCode int32
	defer func() {
		if errCode != 0 {
			if tx != nil {
				e := tx.Rollback()
				if e != nil {
					log.Errorf("%d取消点赞朋友圈id%d时，回滚事务 tx.Rollback() 出现严重错误: %v", c.GetAccountId(), weibo.WeiboId, e)
				}
			}
		}
	}()

	/* 插入 weibo_action_log 表 */
	_, err = tx.Insert(&models.WeiboActionLog{
		AccountId:      c.GetAccountId(),
		WeiboId:        weibo.WeiboId,
		Action:         int(msg_id.MsgActionWeibo_cancel_like_weibo),
		FriendId:       weibo.PublisherId,
		ActionDatetime: unlikeTime,
	})
	if nil != err {
		errCode = 1
		log.Errorf("%d取消点赞朋友圈id%d时，插入表 weibo_action_log 失败:%v", c.GetAccountId(), weibo.WeiboId, err)
		return err
	}

	/* 从 weibo_like 表删除 */
	_, err = tx.DB().Exec("DELETE FROM weibo_like WHERE weibo_id = ? AND liker_id = ?", weibo.WeiboId, c.GetAccountId())
	if nil != err {
		errCode = 2
		log.Errorf("%d取消点赞朋友圈id%d时，从表 weibo_like 删除失败:%v", c.GetAccountId(), weibo.WeiboId, err)
		return err
	}

	/* 更新 weibo 表 点赞次数 */
	_, err = tx.DB().Exec("UPDATE weibo SET thumbup_times = thumbup_times - 1 WHERE weibo_id = ?", weibo.WeiboId)
	if nil != err {
		errCode = 3
		log.Errorf("%d取消点赞朋友圈id%d时，更新表 weibo thumbup_times 失败:%v", c.GetAccountId(), weibo.WeiboId, err)
		return err
	}

	// 事务结束
	err = tx.Commit()
	if err != nil {
		errCode = 4
		log.Errorf("%d取消点赞朋友圈id%d时，提交事务 tx.Commit() 出现严重错误: %v", c.GetAccountId(), weibo.WeiboId, err)
		return err
	}

	return nil
}

// replyWeiboDB 评论朋友圈存入数据库
func replyWeiboDB(c *Client, weibo *msg_struct.Weibo, replyWeibo *msg_struct.ReplyWeibo, replyTime time.Time) error {
	// 事务开始
	tx := c.orm.NewSession()
	defer tx.Close()
	err := tx.Begin()
	if err != nil || tx == nil {
		log.Errorf("%d评论朋友圈id%d时，开始数据库事务失败,error:%v", c.GetAccountId(), weibo.WeiboId, err)
		return err
	}

	var errCode int32
	defer func() {
		if errCode != 0 {
			if tx != nil {
				e := tx.Rollback()
				if e != nil {
					log.Errorf("%d评论朋友圈id%d时，回滚事务 tx.Rollback() 出现严重错误: %v", c.GetAccountId(), weibo.WeiboId, e)
				}
			}
		}
	}()

	/* 插入 weibo_action_log 表 */
	_, err = tx.Insert(&models.WeiboActionLog{
		AccountId:      c.GetAccountId(),
		WeiboId:        weibo.WeiboId,
		Action:         int(msg_id.MsgActionWeibo_comment_weibo),
		FriendId:       weibo.PublisherId,
		ActionDatetime: replyTime,
	})
	if nil != err {
		errCode = 1
		log.Errorf("%d评论朋友圈id%d时，插入表 weibo_action_log 失败:%v", c.GetAccountId(), weibo.WeiboId, err)
		return err
	}

	/* 插入 weibo_reply 表 */
	_, err = tx.Insert(&models.WeiboReply{
		AccountId:     weibo.PublisherId,
		WeiboId:       weibo.WeiboId,
		CommentId:     replyWeibo.CommentedId,
		SenderId:      replyWeibo.CommentatorId,
		BeCommentId:   replyWeibo.BeCommentedId,
		ReceiverId:    replyWeibo.BeCommentatorId,
		MsgContent:    replyWeibo.CommentContent,
		ReplyDatetime: replyTime,
	})
	if nil != err {
		errCode = 2
		log.Errorf("%d评论朋友圈id%d时，插入表 weibo_reply 失败:%v", c.GetAccountId(), weibo.WeiboId, err)
		return err
	}

	// 事务结束
	err = tx.Commit()
	if err != nil {
		errCode = 3
		log.Errorf("%d评论朋友圈id%d时，提交事务 tx.Commit() 出现严重错误: %v", c.GetAccountId(), weibo.WeiboId, err)
		return err
	}

	return nil
}

// deleteWeiboReplyDB 删除朋友圈评论
func deleteWeiboReplyDB(c *Client, weibo *msg_struct.Weibo, deleteWeiboReply *msg_struct.C2SDeleteWeiboReply, deleteReplyTime time.Time) error {
	// 事务开始
	tx := c.orm.NewSession()
	defer tx.Close()
	err := tx.Begin()
	if err != nil || tx == nil {
		log.Errorf("%d删除朋友圈%d评论%d时，开始数据库事务失败,error:%v", c.GetAccountId(), weibo.WeiboId, err)
		return err
	}

	var errCode int32
	defer func() {
		if errCode != 0 {
			if tx != nil {
				e := tx.Rollback()
				if e != nil {
					log.Errorf("%d评论朋友圈id%d时，回滚事务 tx.Rollback() 出现严重错误: %v", c.GetAccountId(), weibo.WeiboId, e)
				}
			}
		}
	}()

	/* 插入 weibo_action_log 表 */
	_, err = tx.Insert(&models.WeiboActionLog{
		AccountId:      c.GetAccountId(),
		WeiboId:        weibo.WeiboId,
		Action:         int(msg_id.MsgActionWeibo_delete_comment_weibo),
		FriendId:       weibo.PublisherId,
		ActionDatetime: deleteReplyTime,
	})
	if nil != err {
		errCode = 1
		log.Errorf("%d删除朋友圈%d评论%d时，插入表 weibo_action_log 失败:%v", c.GetAccountId(), weibo.WeiboId, err)
		return err
	}

	/* 标记 weibo_reply 表此条评论为删除 */
	_, err = tx.DB().Exec("UPDATE weibo_reply SET status = 1 WHERE weibo_id = ? AND comment_id = ?", weibo.WeiboId, deleteWeiboReply.CommentId)
	if nil != err {
		errCode = 2
		log.Errorf("%d删除朋友圈%d评论%d时，插入表 weibo_action_log 失败:%v", c.GetAccountId(), weibo.WeiboId, err)
		return err
	}

	// 事务结束
	err = tx.Commit()
	if err != nil {
		errCode = 3
		log.Errorf("%d评论朋友圈id%d时，提交事务 tx.Commit() 出现严重错误: %v", c.GetAccountId(), weibo.WeiboId, err)
		return err
	}

	return nil
}

// deleteWeiboDB 删除朋友圈存入数据库
func deleteWeiboDB(c *Client, weibo *msg_struct.Weibo, deleteTime time.Time) error {

	// 事务开始
	tx := c.orm.NewSession()
	defer tx.Close()
	err := tx.Begin()
	if err != nil || tx == nil {
		log.Errorf("%d删除朋友圈id=%d时，开始数据库事务失败,error:%v", c.GetAccountId(), weibo.WeiboId, err)
		return err
	}

	var errCode int32
	defer func() {
		if errCode != 0 {
			if tx != nil {
				e := tx.Rollback()
				if e != nil {
					log.Errorf("%d删除朋友圈id=%d时，回滚事务 tx.Rollback() 出现严重错误: %v", c.GetAccountId(), weibo.WeiboId, e)
				}
			}
		}
	}()

	/* 从数据库 weibo 表获取此朋友圈 */
	weiboDB := &models.Weibo{}
	bGet, err := tx.Where("weibo_id = ?", weibo.WeiboId).Get(weiboDB)
	if nil != err {
		errCode = 1
		log.Errorf("%d删除朋友圈id=%d时，从 weibo 表读取错误: %v", c.GetAccountId(), weibo.WeiboId, err)
		return err
	}
	if !bGet {
		errCode = 2
		log.Errorf("%d删除朋友圈id=%d时，从 weibo 表未读取到此条朋友圈", c.GetAccountId(), weibo.WeiboId)
	}

	/* 从数据库 weibo 表删除此朋友圈 */
	_, err = tx.DB().Exec("DELETE FROM weibo WHERE weibo_id = ?", weibo.WeiboId)
	if nil != err {
		errCode = 3
		log.Errorf("%d删除朋友圈id=%d时，删除 weibo 表出错: %v", c.GetAccountId(), weibo.WeiboId, err)
		return err
	}

	/* 插入 weibo_deleted 表 */
	_, err = tx.Insert(&models.WeiboDeleted{
		AccountId:    weiboDB.AccountId,
		PhotosBin:    weiboDB.PhotosBin,
		MsgContent:   weiboDB.MsgContent,
		WeiboId:      weiboDB.WeiboId,
		MsgDatetime:  weiboDB.MsgDatetime,
		ThumbupTimes: weiboDB.ThumbupTimes,
		DelDatetime:  deleteTime,
	})
	if nil != err {
		errCode = 4
		log.Errorf("%d删除朋友圈id=%d时，插入 weibo_deleted 表出错: %v", c.GetAccountId(), weibo.WeiboId, err)
		return err
	}

	/* 标记 weibo_reply 表中此条朋友圈已被删除 */
	_, err = tx.DB().Exec("UPDATE weibo_reply SET deleted = 1 WHERE weibo_id = ?", weibo.WeiboId)
	if nil != err {
		errCode = 5
		log.Errorf("%d删除朋友圈id=%d时，标记 weibo_reply 表中此条朋友圈已被删除 出错: %v", c.GetAccountId(), weibo.WeiboId, err)
		return err
	}

	/* 标记 weibo_like 表中此条朋友圈已被删除 */
	_, err = tx.DB().Exec("UPDATE weibo_like SET deleted = 1 WHERE weibo_id = ?", weibo.WeiboId)
	if nil != err {
		errCode = 6
		log.Errorf("%d删除朋友圈id=%d时，标记 weibo_like 表中此条朋友圈已被删除 出错: %v", c.GetAccountId(), weibo.WeiboId, err)
		return err
	}

	// 事务结束
	err = tx.Commit()
	if err != nil {
		errCode = 7
		log.Errorf("%d删除朋友圈id=%d时，提交事务 tx.Commit() 出现严重错误: %v", c.GetAccountId(), weibo.WeiboId, err)
		return err
	}

	return nil
}

/* redis */
// publishWeiboRds 发朋友圈，存入redis
func publishWeiboRds(c *Client, weiboId, timeStamp int64, weibo_bin []byte) error {
	weiboIdStr := strconv.FormatInt(weiboId, 10)
	err := c.rdsWeiboString.Set(c.GetStrAccount()+":"+weiboIdStr, string(weibo_bin))
	if nil != err {
		return err
	}

	_, err = c.rdsWeiboList.Do("LPUSH", c.rdsWeiboList.AddPrefix(c.GetStrAccount()), weiboIdStr)
	if nil != err {
		return err
	}

	_, err = c.rdsWeiboSet.AddSetMembers(c.GetStrAccount(), weiboIdStr)
	if nil != err {
		return err
	}

	err = c.rdsWeiboZset.SortedSetAddSingle(c.GetStrAccount(), c.GetStrAccount()+":"+weiboIdStr, timeStamp)
	if nil != err {
		return err
	}

	return nil
}

// getWeiboRds 从redis获取userId的朋友圈weiboId详细信息
func getWeiboRds(c *Client, userId, weiboId int64) (*msg_struct.Weibo, error) {
	weiboStr, err := c.rdsWeiboString.Get(strconv.FormatInt(userId, 10) + ":" + strconv.FormatInt(weiboId, 10))
	if nil != err {
		log.Errorf("用户:%d获取%d的朋友圈:%d详细信息，从 redis 读取失败:%v", c.GetAccountId(), userId, weiboId, err)
		return nil, err
	}

	weibo := &msg_struct.Weibo{}
	if !function.ProtoUnmarshal([]byte(weiboStr), weibo, "msg_struct.Weibo") {
		log.Errorf("用户:%d获取%d的朋友圈:%d详细信息，反序列化为msg_struct.Weibo失败:%v", c.GetAccountId(), userId, weiboId, err)
		return nil, err
	}
	return weibo, nil
}

// likeWeiboRds 点赞存入redis
func likeWeiboRds(c *Client, weibo *models.Weibo, timeStamp int64) error {
	weiboPublisherIdStr := strconv.FormatInt(weibo.AccountId, 10)
	weiboIdStr := strconv.FormatInt(weibo.WeiboId, 10)
	_, err := c.rdsWeiboLikeHash.HashSet(weiboPublisherIdStr+":"+weiboIdStr, c.GetStrAccount(), timeStamp)
	if nil != err {
		return err
	}

	_, err = c.rdsWeiboLikeSet.AddSetMembers(weiboPublisherIdStr+":"+weiboIdStr, c.GetStrAccount())
	if nil != err {
		return err
	}

	return nil
}

// cancelLikeWeiboRds 取消点赞朋友圈，存入redis
func cancelLikeWeiboRds(c *Client, weibo *msg_struct.Weibo) error {
	weiboPublisherIdStr := strconv.FormatInt(weibo.PublisherId, 10)
	weiboIdStr := strconv.FormatInt(weibo.WeiboId, 10)
	_, err := c.rdsWeiboLikeHash.DeleteHashSetField(weiboPublisherIdStr+":"+weiboIdStr, c.GetStrAccount(), c.GetStrAccount())
	if nil != err {
		return err
	}

	_, err = c.rdsWeiboLikeSet.RemoveSetMembers(weiboPublisherIdStr+":"+weiboIdStr, c.GetStrAccount())
	if nil != err {
		return err
	}

	return nil
}

// deleteWeiboSomeoneReplyRds 用户删除朋友圈点赞
func deleteWeiboAllLikeRds(c *Client, weiboIdStr string) error {
	_, err := c.rdsWeiboLikeSet.Del(c.GetStrAccount() + ":" + weiboIdStr)
	if nil != err {
		log.Errorf("%d删除朋友圈id=%d时，从redis[%s]中删除失败 ,error:%v", c.GetAccountId(), c.rdsWeiboLikeSet.AddPrefix(c.GetStrAccount()+":"+weiboIdStr), err)
		return err
	}

	_, err = c.rdsWeiboLikeHash.Del(c.GetStrAccount() + ":" + weiboIdStr)
	if nil != err {
		log.Errorf("%d删除朋友圈id=%d时，从redis[%s]中删除失败 ,error:%v", c.GetAccountId(), c.rdsWeiboLikeHash.AddPrefix(c.GetStrAccount()+":"+weiboIdStr), err)
		return err
	}
	return nil
}

// getLikeWeiboRdsIds 获取某条朋友圈所有的点赞用户id
func getLikeWeiboRdsIds(c *Client, weiboPublisherId, weiboId int64) ([]interface{}, error) {
	return c.rdsWeiboLikeSet.GetSetMembers(strconv.FormatInt(weiboPublisherId, 10) + ":" + strconv.FormatInt(weiboId, 10))
}

// replyWeiboRds 评论存入redis
func replyWeiboRds(c *Client, weibo *msg_struct.Weibo, replyWeibo *msg_struct.ReplyWeibo) error {
	weiboPublishIdStr := strconv.FormatInt(weibo.PublisherId, 10)
	weiboIdStr := strconv.FormatInt(weibo.WeiboId, 10)
	_, err := c.rdsWeiboReplySet.AddSetMembers(weiboPublishIdStr+":"+weiboIdStr, c.GetStrAccount())
	if nil != err {
		log.Errorf("%d评论用户id=%d发布的朋友圈id=%d时，回复用户id=%d的评论，评论内容为%s，存入redis %s 失败%v",
			c.GetAccountId(), weibo.PublisherId, weibo.WeiboId, replyWeibo.CommentedId, replyWeibo.CommentContent, c.rdsWeiboReplySet.GetPrefix(), err)
		return err
	}

	replyWeibo_bin, bRet := function.ProtoMarshal(replyWeibo, "msg_struct.ReplyWeibo")
	if !bRet || nil == replyWeibo_bin {
		return errors.New("protobuf marshal struct msg_struct.ReplyWeibo failed")
	}

	_, err = c.rdsWeiboReplyList.Do("RPUSH", c.rdsWeiboReplyList.AddPrefix(weiboPublishIdStr+":"+weiboIdStr), replyWeibo_bin)
	if nil != err {
		log.Errorf("%d评论用户id=%d发布的朋友圈id=%d时，回复用户id=%d的评论，评论内容为%s，存入redis %s 失败%v",
			c.GetAccountId(), weibo.PublisherId, weibo.WeiboId, replyWeibo.CommentedId, replyWeibo.CommentContent, c.rdsWeiboReplyList.GetPrefix(), err)
		return err
	}

	return nil
}

// getReplyWeiboRds 获取某条朋友圈的所有评论（此处要求发布评论用户、被评论用户均为查看者好友）
func getReplyWeiboRds(c *Client, weibo *msg_struct.Weibo) ([][]byte, error) {
	weiboPublishIdStr := strconv.FormatInt(weibo.PublisherId, 10)
	weiboIdStr := strconv.FormatInt(weibo.WeiboId, 10)
	inters, err := redis.Values(c.rdsWeiboReplyList.Do("LRANGE", c.rdsWeiboReplyList.AddPrefix(weiboPublishIdStr+":"+weiboIdStr), "0", "-1"))
	if nil != err {
		return nil, err
	}

	var replyWeiboBytes [][]byte
	for _, inter := range inters {
		replyWeibo := &msg_struct.ReplyWeibo{}
		Bytes := inter.([]byte)
		if !function.ProtoUnmarshal(Bytes, replyWeibo, "msg_struct.ReplyWeibo") {
			log.Errorf("用户%d查看%d朋友圈%d评论时，反序列化 msg_struct.ReplyWeibo 失败", c.GetAccountId(), weibo.PublisherId, weibo.WeiboId)
			continue
		}
		if (c.GetAccountId() == replyWeibo.CommentatorId || c.IsFriend(replyWeibo.CommentatorId)) &&
			(c.GetAccountId() == replyWeibo.BeCommentatorId || c.IsFriend(replyWeibo.BeCommentatorId)) {
			replyWeiboBytes = append(replyWeiboBytes, Bytes)
		}
	}

	return replyWeiboBytes, nil
}

// deleteWeiboSomeoneReplyRds 删除朋友圈某一条评论
func deleteWeiboSomeoneReplyRds(c *Client, weibo *msg_struct.Weibo, weiboReplyBytes []byte) error {
	weiboPublisherIdStr := strconv.FormatInt(weibo.PublisherId, 10)
	weiboIdStr := strconv.FormatInt(weibo.WeiboId, 10)
	_, err := c.rdsWeiboReplySet.RemoveSetMembers(weiboPublisherIdStr+":"+weiboIdStr, c.GetStrAccount())
	if nil != err {
		log.Errorf("%d删除朋友圈id=%d时，从redis[%s]中删除失败 ,error:%v", c.GetAccountId(), c.rdsWeiboReplySet.AddPrefix(c.GetStrAccount()+":"+weiboIdStr), err)
		return err
	}

	_, err = c.rdsWeiboReplyList.Do("LREM", c.rdsWeiboReplyList.AddPrefix(weiboPublisherIdStr+":"+weiboIdStr), 0, weiboReplyBytes)
	if nil != err {
		log.Errorf("%d删除朋友圈id=%d时，从redis[%s]中删除失败 ,error:%v", c.GetAccountId(), c.rdsWeiboReplyList.AddPrefix(c.GetStrAccount()+":"+weiboIdStr), err)
		return err
	}
	return nil
}

// deleteWeiboAllReplyRds 删除朋友圈所有评论
func deleteWeiboAllReplyRds(c *Client, weiboIdStr string) error {
	if _, err := c.rdsWeiboReplySet.Del(c.GetStrAccount() + ":" + weiboIdStr); nil != err {
		log.Errorf("用户%d删除朋友圈%s时，删除 redis %s 所有评论出错%v",
			c.GetAccountId(), weiboIdStr, c.rdsWeiboReplySet.GetPrefix()+c.GetStrAccount()+":"+weiboIdStr, err)
		return err
	}

	if _, err := c.rdsWeiboReplyList.Del(c.GetStrAccount() + ":" + weiboIdStr); nil != err {
		log.Errorf("用户%d删除朋友圈%s时，删除 redis %s 所有评论出错%v",
			c.GetAccountId(), weiboIdStr, c.rdsWeiboReplySet.GetPrefix()+c.GetStrAccount()+":"+weiboIdStr, err)
		return err
	}

	return nil
}

// deleteWeiboRds 删除朋友圈，操作redis数据
func deleteWeiboRds(c *Client, weibo *msg_struct.Weibo) error {
	weiboIdStr := strconv.FormatInt(weibo.WeiboId, 10)
	/* 删除此条朋友圈 */
	_, err := c.rdsWeiboList.Do("LREM", c.rdsWeiboList.AddPrefix(c.GetStrAccount()), "1", weiboIdStr)
	if nil != err {
		log.Errorf("%d删除朋友圈id=%d时，从redis[%s]中删除失败 ,error:%v", c.GetAccountId(), c.rdsWeiboList.AddPrefix(c.GetStrAccount()), err)
		return err
	}

	_, err = c.rdsWeiboSet.RemoveSetMembers(c.GetStrAccount(), weiboIdStr)
	if nil != err {
		log.Errorf("%d删除朋友圈id=%d时，从redis[%s]中删除失败 ,error:%v", c.GetAccountId(), c.rdsWeiboSet.AddPrefix(c.GetStrAccount()), err)
		return err
	}

	_, err = c.rdsWeiboString.Del(c.GetStrAccount() + ":" + weiboIdStr)
	if nil != err {
		log.Errorf("%d删除朋友圈id=%d时，从redis[%s]中删除失败 ,error:%v", c.GetAccountId(), c.rdsWeiboString.AddPrefix(c.GetStrAccount()+":"+weiboIdStr), err)
		return err
	}

	_, err = c.rdsWeiboZset.SortedSetRem(c.GetStrAccount(), c.GetStrAccount()+":"+weiboIdStr)
	if nil != err {
		log.Errorf("%d删除朋友圈id=%d时，从redis[%s]中删除失败 ,error:%v", c.GetAccountId(), c.rdsWeiboZset.AddPrefix(c.GetStrAccount()+":"+weiboIdStr), err)
		return err
	}

	/* 删除此条朋友圈的所有点赞 */
	if err := deleteWeiboAllLikeRds(c, weiboIdStr); nil != err {
		return err
	}

	/* 删除此条朋友圈的所有评论 */
	if err := deleteWeiboAllReplyRds(c, weiboIdStr); nil != err {
		return err
	}
	return nil
}

// getLikeAndReplyWeiboRdsIds 获取某条朋友圈所有的点赞用户id集合和评论用户id集合的并集
func getLikeAndReplyWeiboRdsIds(c *Client, weiboId, weiboPublisherId int64) (interface{}, error) {
	weiboPublishIdStr := strconv.FormatInt(weiboPublisherId, 10)
	weiboIdStr := strconv.FormatInt(weiboId, 10)
	return c.rdsWeiboSet.Do("SUNION",
		c.rdsWeiboReplySet.AddPrefix(weiboPublishIdStr+":"+weiboIdStr),
		c.rdsWeiboLikeSet.AddPrefix(weiboPublishIdStr+":"+weiboIdStr))
}

// getSomeoneWeiboRds 查看某人朋友圈
func getSomeoneWeiboRds(c *Client, weiboPublisherId, curBrowseTimeStamp, lastTimeStamp int64) ([][]byte, msg_struct.FriendLookWeiboStrategy, error) {
	var (
		weiboNum           = 12
		allowLookTimeStamp int64
		strategy           = msg_struct.FriendLookWeiboStrategy_undefined
	)
	weiboPublisherIdStr := strconv.FormatInt(weiboPublisherId, 10)

	if c.GetAccountId() != weiboPublisherId {
		if c.IsFriend(weiboPublisherId) {
			/* 查看者是好友 */
			/* 校验查看者是不是被查看者的黑名单好友 */
			num, err := c.rdsBlackList.IsSetMember(weiboPublisherIdStr, c.GetStrAccount())
			if nil != err {
				log.Errorf("用户%d查看%d朋友圈，判断查看者是不是被查看者的黑名单好友出错%v", c.GetAccountId(), weiboPublisherId, err)
				return nil, strategy, err
			}
			if 1 == num {
				log.Errorf("用户%d查看%d朋友圈，查看者是被查看者的黑名单好友", c.GetAccountId(), weiboPublisherId)
				return nil, strategy, nil
			}

			/* 校验查看者是否被允许查看朋友圈 */
			if !DoesAllowFriendLookMyWeiboRds(c, weiboPublisherId, c.GetAccountId()) {
				log.Errorf("用户%d查看%d朋友圈，查看者不被允许查看朋友圈", c.GetAccountId(), weiboPublisherId)
				return nil, strategy, nil
			}

			strategy, allowLookTimeStamp, err = GetFriendLookStrategy(c, weiboPublisherId)
			if nil != err && redis.ErrNil != err {
				log.Errorf("用户%d查看%d朋友圈，获取好友查看策略出错%v", c.GetAccountId(), weiboPublisherId, err)
				return nil, strategy, err
			}
		} else {
			/* 查看者不是好友 */
			if !DoesAllowStrangerLookWeibo(c, weiboPublisherId) {
				return nil, strategy, nil
			}
			weiboNum = 10
		}
	}

	inters, err := c.rdsWeiboSet.GetSetMembers(weiboPublisherIdStr)
	if nil != err {
		return nil, strategy, err
	}

	var weiboIdStrs []string
	for _, inter := range inters {
		weiboIdStr := string(inter.([]byte))
		weiboId, _ := strconv.ParseInt(weiboIdStr, 10, 64)
		if curBrowseTimeStamp < weiboId {
			weiboIdStrs = append(weiboIdStrs, weiboIdStr)
		}
	}
	// 倒序排序
	sort.Sort(sort.Reverse(sort.StringSlice(weiboIdStrs)))

	var weiboBins [][]byte
	for _, weiboIdStr := range weiboIdStrs {
		weiboId, _ := strconv.ParseInt(weiboIdStr, 10, 64)
		if 0 != lastTimeStamp && weiboId > lastTimeStamp-allowLookTimeStamp {
			continue
		}

		weiboStr, err := c.rdsWeiboString.Get(weiboPublisherIdStr + ":" + weiboIdStr)
		if nil != err {
			continue
		}
		weiboBins = append(weiboBins, []byte(weiboStr))

		if weiboNum == len(weiboBins) {
			break
		}
	}

	return weiboBins, strategy, nil
}
