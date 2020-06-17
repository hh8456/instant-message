package client

import (
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"instant-message/common-library/packet"
	"instant-message/common-library/proto/msg_id"
	"instant-message/common-library/proto/msg_struct"
	"strconv"

	"github.com/gomodule/redigo/redis"
)

const (
	FRIEND_ALL = iota
	FRIEND_HALF_YEAR
	FRIEND_ONE_MONTH
	FRIEND_THREE_DAYS
	ALLOW_STRANGERS_LOOK_WEIBO = "allow_strangers_look_weibo"
	STRANGER_LOOK_WEIBO_NUM    = 10                 // 允许陌生人查看朋友圈条数
	FRIEND_LOOK_THREE_DAYS     = 60 * 60 * 24 * 3   // 允许朋友查看的朋友圈范围（3天）
	FRIEND_LOOK_ONE_MONTH      = 60 * 60 * 24 * 30  // 允许朋友查看的朋友圈范围（30天）
	FRIEND_LOOK_HALF_YEAR      = 60 * 60 * 24 * 180 // 允许朋友查看的朋友圈范围（180天）
)

func init() {
	// 获取不看好友朋友圈的好友列表
	registerLogicCall(msg_id.NetMsgId_C2SGetDontLookFriendWeiboList, C2SGetDontLookFriendWeiboList)
	// 用户设置看/不看某人的朋友圈
	registerLogicCall(msg_id.NetMsgId_C2SDontLookFriendWeibo, C2SDontLookFriendWeibo)
	// 获取禁止好友看我朋友圈的好友列表
	registerLogicCall(msg_id.NetMsgId_C2SGetForbiddenFriendLookWeiboList, C2SGetForbiddenFriendLookWeiboList)
	// 用户设置允许/不允许某人看自己的朋友圈
	registerLogicCall(msg_id.NetMsgId_C2SForbiddenFriendLookWeibo, C2SForbiddenFriendLookWeibo)
	// 获取是否允许陌生人查看朋友圈开关
	registerLogicCall(msg_id.NetMsgId_C2SIsAllowStrangerLookWeibo, C2SIsAllowStrangerLookWeibo)
	// 用户设置允许/不允许陌生人查看朋友圈
	registerLogicCall(msg_id.NetMsgId_C2SAllowStrangerLookWeibo, C2SAllowStrangerLookWeibo)
	// 获取允许朋友查看朋友圈范围
	registerLogicCall(msg_id.NetMsgId_C2SGetFriendLookWeiboStrategy, C2SGetFriendLookWeiboStrategy)
	// 用户设置允许朋友查看朋友圈范围
	registerLogicCall(msg_id.NetMsgId_C2SFriendLookWeiboStrategy, C2SFriendLookWeiboStrategy)
}

// C2SGetDontLookFriendWeiboList 获取不看好友朋友圈的好友列表
func C2SGetDontLookFriendWeiboList(c *Client, msg []byte) {
	dontLookFriendWeiboList := &msg_struct.C2SGetDontLookFriendWeiboList{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], dontLookFriendWeiboList, "C2SGetDontLookFriendWeiboList") {
		return
	}

	dontlookFriendWeiboIds, _ := getDontlookFriendWeiboIdsRds(c)

	c.SendPbMsg(msg_id.NetMsgId_S2CGetDontLookFriendWeiboList, &msg_struct.S2CGetDontLookFriendWeiboList{
		FriendBriefInfo: dontlookFriendWeiboIds,
		Ret:             "Success",
	})
}

// C2SDontLookFriendWeibo 用户设置不看某好友的朋友圈
func C2SDontLookFriendWeibo(c *Client, msg []byte) {
	dontLookFriendWeibo := &msg_struct.C2SDontLookFriendWeibo{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], dontLookFriendWeibo, "msg_struct.C2SDontLookFriendWeibo") {
		return
	}

	realFriends := c.IsFriends(dontLookFriendWeibo.GetFriendIds())
	if nil == realFriends {
		return
	}

	/* 用户设置不看某人的朋友圈，存入redis */
	err := dontLookFriendWeiboRds(c, realFriends, dontLookFriendWeibo.BNotLook)
	if nil != err {
		return
	}

	if dontLookFriendWeibo.GetBNotLook() {
		log.Tracef("用户%d设置不看好友%d朋友圈", c.GetAccountId(), realFriends)
	} else {
		log.Tracef("用户%d设置看好友%d朋友圈", c.GetAccountId(), realFriends)
	}

	dontlookFriendWeiboBriefInfos, err := getDontlookFriendWeiboIdsRds(c)
	if nil != err {
		return
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CDontLookFriendWeibo, &msg_struct.S2CDontLookFriendWeibo{
		FriendBriefInfos: dontlookFriendWeiboBriefInfos,
		Ret:              "Success",
	})
}

// C2SGetForbiddenFriendLookWeiboList 获取禁止好友看我朋友圈的好友列表
func C2SGetForbiddenFriendLookWeiboList(c *Client, msg []byte) {
	forbiddenFriendLookWeiboList := &msg_struct.C2SGetForbiddenFriendLookWeiboList{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], forbiddenFriendLookWeiboList, "C2SGetForbiddenFriendLookWeiboList") {
		return
	}

	dontlookFriendWeiboIds, err := getForbiddenFriendLookWeiboList(c)
	if nil != err {
		return
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CGetForbiddenFriendLookWeiboList, &msg_struct.S2CGetForbiddenFriendLookWeiboList{
		FriendBriefInfo: dontlookFriendWeiboIds,
		Ret:             "Success",
	})
}

// C2SForbiddenFriendLookWeibo 用户设置不允许某人看自己的朋友圈
func C2SForbiddenFriendLookWeibo(c *Client, msg []byte) {
	forbiddenFriendLookWeibo := &msg_struct.C2SForbiddenFriendLookWeibo{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], forbiddenFriendLookWeibo, "msg_struct.C2SForbiddenFriendLookWeibo") {
		return
	}

	realFriends := c.IsFriends(forbiddenFriendLookWeibo.GetFriendIds())
	if nil == realFriends {
		return
	}

	err := forbiddenFriendLookWeiboRds(c, realFriends, forbiddenFriendLookWeibo.BForbidden)
	if nil != err {
		log.Errorf("forbiddenFriendLookWeiboRds error: %v", err)
		return
	}

	if forbiddenFriendLookWeibo.GetBForbidden() {
		log.Tracef("用户%d设置不允许好友 %v 看自己朋友圈", c.GetAccountId(), realFriends)
	} else {
		log.Tracef("用户%d设置允许好友 %v 看自己朋友圈", c.GetAccountId(), realFriends)
	}

	forbiddenFriendLookWeiboList, _ := getForbiddenFriendLookWeiboList(c)

	c.SendPbMsg(msg_id.NetMsgId_S2CForbiddenFriendLookWeibo, &msg_struct.S2CForbiddenFriendLookWeibo{
		FriendBriefInfos: forbiddenFriendLookWeiboList,
		Ret:              "Success",
	})
}

// C2SIsAllowStrangerLookWeibo 获取是否允许陌生人查看朋友圈开关
func C2SIsAllowStrangerLookWeibo(c *Client, msg []byte) {
	isAllowStrangerLookWeibo := &msg_struct.C2SIsAllowStrangerLookWeibo{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], isAllowStrangerLookWeibo, "msg_struct.C2SIsAllowStrangerLookWeibo") {
		return
	}

	bAllow, err := c.rdsStrangerLookWeiboSet.IsSetMember(ALLOW_STRANGERS_LOOK_WEIBO, c.GetStrAccount())
	if nil != err {
		return
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CIsAllowStrangerLookWeibo, &msg_struct.S2CIsAllowStrangerLookWeibo{
		BAllow: bAllow == 0,
		Ret:    "Success",
	})
}

// C2SAllowStrangerLookWeibo 是否允许陌生人查看朋友圈
func C2SAllowStrangerLookWeibo(c *Client, msg []byte) {
	allowStrangerLookWeibo := &msg_struct.C2SAllowStrangerLookWeibo{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], allowStrangerLookWeibo, "msg_struct.C2SAllowStrangerLookWeibo") {
		return
	}

	/* 用户设置是否允许陌生人查看朋友圈，存入redis */
	var err error
	if !allowStrangerLookWeibo.GetAllow() {
		_, err = c.rdsStrangerLookWeiboSet.AddSetMembers(ALLOW_STRANGERS_LOOK_WEIBO, c.GetStrAccount())
		if nil != err {
			return
		}
		log.Tracef("用户%d设置不允许陌生人查看自己%d条朋友圈", c.GetAccountId(), STRANGER_LOOK_WEIBO_NUM)
	} else {
		_, err = c.rdsStrangerLookWeiboSet.RemoveSetMembers(ALLOW_STRANGERS_LOOK_WEIBO, c.GetStrAccount())
		if nil != err {
			if redis.ErrNil != err {
				return
			}
		}
		log.Tracef("用户%d设置允许陌生人查看自己%d条朋友圈", c.GetAccountId(), STRANGER_LOOK_WEIBO_NUM)
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CAllowStrangerLookWeibo, &msg_struct.S2CAllowStrangerLookWeibo{
		Ret: "Success",
	})
}

// C2SGetFriendLookWeiboStrategy 获取允许好友查看朋友圈时间范围
func C2SGetFriendLookWeiboStrategy(c *Client, msg []byte) {
	getFriendLookWeiboStrategy := &msg_struct.C2SGetFriendLookWeiboStrategy{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], getFriendLookWeiboStrategy, "msg_struct.C2SGetFriendLookWeiboStrategy") {
		return
	}

	strategyStr, _ := c.rdsFriendLookWeiboString.Get(c.GetStrAccount())

	strategy, _ := strconv.ParseInt(strategyStr, 10, 64)

	c.SendPbMsg(msg_id.NetMsgId_S2CGetFriendLookWeiboStrategy, &msg_struct.S2CGetFriendLookWeiboStrategy{
		Strategy: msg_struct.FriendLookWeiboStrategy(strategy),
		Ret:      "Success",
	})
}

// C2SFriendLookWeiboStrategy 允许朋友查看朋友圈范围
func C2SFriendLookWeiboStrategy(c *Client, msg []byte) {
	friendLookWeiboStrategy := &msg_struct.C2SFriendLookWeiboStrategy{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], friendLookWeiboStrategy, "msg_struct.C2SFriendLookWeiboStrategy") {
		return
	}

	var err error

	switch friendLookWeiboStrategy.Strategy {
	case FRIEND_ALL:
		_, err = c.rdsFriendLookWeiboString.Del(c.GetStrAccount())
	case FRIEND_HALF_YEAR:
		err = c.rdsFriendLookWeiboString.Set(c.GetStrAccount(), strconv.FormatInt(int64(msg_struct.FriendLookWeiboStrategy_halfYear), 10))
	case FRIEND_ONE_MONTH:
		err = c.rdsFriendLookWeiboString.Set(c.GetStrAccount(), strconv.FormatInt(int64(msg_struct.FriendLookWeiboStrategy_oneMonth), 10))
	case FRIEND_THREE_DAYS:
		err = c.rdsFriendLookWeiboString.Set(c.GetStrAccount(), strconv.FormatInt(int64(msg_struct.FriendLookWeiboStrategy_threeDays), 10))
	default:
		return
	}

	if nil != err {
		return
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CFriendLookWeiboStrategy, &msg_struct.S2CFriendLookWeiboStrategy{
		Ret: "Success",
	})
}

// getDontlookFriendWeiboIdsRds 获取不看好友朋友圈的好友列表
func getDontlookFriendWeiboIdsRds(c *Client) ([]*msg_struct.UserBriefInfo, error) {
	members, err := c.rdsDontLookWeiboSet.GetSetMembers(c.GetStrAccount())
	if nil != err {
		log.Errorf("用户%d获取不看好友朋友圈的好友列表，读取redis出错%v", c.GetAccountId(), err)
		return nil, err
	}

	friendIds := make([]int64, 0)
	for _, member := range members {
		friendId, _ := strconv.ParseInt(string(member.([]byte)), 10, 64)
		friendIds = append(friendIds, friendId)
	}

	friendBriefInfos, _ := c.QueryOthersBriefInfos(friendIds)

	return friendBriefInfos, nil
}

// dontLookFriendWeiboRds 不看好友friendId的朋友圈
func dontLookFriendWeiboRds(c *Client, friendIds []int64, bLook bool) error {
	var err error

	for _, friendId := range friendIds {
		if bLook {
			_, err = c.rdsDontLookWeiboSet.AddSetMembers(c.GetStrAccount(), friendId)
		} else {
			_, err = c.rdsDontLookWeiboSet.RemoveSetMembers(c.GetStrAccount(), friendId)
		}
	}

	return err
}

// getForbiddenFriendLookWeiboList 获取禁止好友看我朋友圈的好友列表
func getForbiddenFriendLookWeiboList(c *Client) ([]*msg_struct.UserBriefInfo, error) {
	members, err := c.rdsForbiddenFriendLookWeiboSet.GetSetMembers(c.GetStrAccount())
	if nil != err {
		log.Errorf("用户%d获取禁止好友看我朋友圈的好友列表，读取redis出错%v", c.GetAccountId(), err)
		return nil, err
	}

	friendIds := make([]int64, 0)
	for _, member := range members {
		friendId, _ := strconv.ParseInt(string(member.([]byte)), 10, 64)
		friendIds = append(friendIds, friendId)
	}

	friendBriefInfos, _ := c.QueryOthersBriefInfos(friendIds)

	return friendBriefInfos, nil
}

// forbiddenFriendLookWeiboRds 禁止好友看“我”的朋友圈
func forbiddenFriendLookWeiboRds(c *Client, friendIds []int64, bForbidden bool) error {
	var err error

	for _, friendId := range friendIds {
		if bForbidden {
			// 禁止好友看“我”的朋友圈
			_, err = c.rdsForbiddenFriendLookWeiboSet.AddSetMembers(c.GetStrAccount(), friendId)
		} else {
			// 允许好友看“我”的朋友圈
			_, err = c.rdsForbiddenFriendLookWeiboSet.RemoveSetMembers(c.GetStrAccount(), friendId)
		}
	}

	return err
}

// DoesLookFriendWeiboRds “我”是否看friendId的朋友圈 返回true：看friendId朋友圈;false：不看friendId朋友圈
func DoesLookFriendWeiboRds(c *Client, lookerId, friendId int64) bool {
	num, err := c.rdsDontLookWeiboSet.IsSetMember(strconv.FormatInt(lookerId, 10), strconv.FormatInt(friendId, 10))
	if nil != err {
		log.Errorf("从 redis 判断%d是否看%d的朋友圈出错%v", lookerId, friendId, err)
		return false
	}
	return num == 0
}

// DoesAllowFriendLookMyWeiboRds 我是否被好友禁止看朋友圈
func DoesAllowFriendLookMyWeiboRds(c *Client, weiboPublisherId, lookerId int64) bool {
	num, err := c.rdsForbiddenFriendLookWeiboSet.IsSetMember(strconv.FormatInt(weiboPublisherId, 10), strconv.FormatInt(lookerId, 10))
	if nil != err {
		return false
	}
	return num == 0
}

// DoesAllowStrangerLookWeibo userId是否允许陌生人看“他”的朋友圈
func DoesAllowStrangerLookWeibo(c *Client, userId int64) bool {
	num, err := c.rdsStrangerLookWeiboSet.IsSetMember(ALLOW_STRANGERS_LOOK_WEIBO, strconv.FormatInt(userId, 10))
	if nil != err {
		log.Errorf("从 redis 判断%d是否允许陌生人查看朋友圈", userId)
		return false
	}
	return num == 0
}

// getFriendLookStrategy 获取某人允许好友查看朋友圈时间范围
func GetFriendLookStrategy(c *Client, weiboPublisherId int64) (msg_struct.FriendLookWeiboStrategy, int64, error) {
	strategyStr, err := c.rdsFriendLookWeiboString.Get(strconv.FormatInt(weiboPublisherId, 10))
	if nil != err {
		return msg_struct.FriendLookWeiboStrategy_undefined, 0, err
	}

	strategy, err := strconv.ParseInt(strategyStr, 10, 64)
	if nil != err {
		return msg_struct.FriendLookWeiboStrategy_undefined, 0, err
	}

	switch msg_struct.FriendLookWeiboStrategy(strategy) {
	case msg_struct.FriendLookWeiboStrategy_undefined:
		return msg_struct.FriendLookWeiboStrategy(strategy), 0, nil
	case msg_struct.FriendLookWeiboStrategy_halfYear:
		return msg_struct.FriendLookWeiboStrategy(strategy), FRIEND_LOOK_HALF_YEAR, nil
	case msg_struct.FriendLookWeiboStrategy_oneMonth:
		return msg_struct.FriendLookWeiboStrategy(strategy), FRIEND_LOOK_ONE_MONTH, nil
	case msg_struct.FriendLookWeiboStrategy_threeDays:
		return msg_struct.FriendLookWeiboStrategy(strategy), FRIEND_LOOK_THREE_DAYS, nil
	}

	return msg_struct.FriendLookWeiboStrategy_undefined, 0, nil
}
