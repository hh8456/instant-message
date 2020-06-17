package client

import (
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"instant-message/common-library/proto/msg_struct"
	"strconv"

	"github.com/gogo/protobuf/proto"
)

const (
	RdsKeyPrefixPrivateChatRecord                = "private_chat_record"
	RdsKeyPrefixUid                              = "uid"
	RdsKeyPrefixBriefInfo                        = "brief_info"
	RdsKeyPrefixGroupChatRecord                  = "group_chat_record"
	RdsKeyPrefixChatGroupInfo                    = "chat_group_info"
	RdsKeyPrefixPushInfo                         = "push_info"
	RdsKeyPrefixUserNameInfo                     = "user_name_info"
	RdsKeyPrefixPhoneInfo                        = "phone_info"
	RdsKeyPrefixUserOnline                       = "user_online"
	RdsKeyPrefixHeadPortraitUrl                  = "head_portrait"
	RdsKeyPrefixHuanXinString                    = "huanxin:string"
	RdsKeyPrefixWeiboCover                       = "weibo_cover"
	RdsKeyPrefixWeiboCoverLiker                  = "weibo_cover_liker:set"
	RdsKeyPrefixWeiboList                        = "weibo:list"
	RdsKeyPrefixWeiboSet                         = "weibo:set"
	RdsKeyPrefixWeiboString                      = "weibo:string"
	RdsKeyPrefixWeiboZset                        = "weibo:sortedSet"
	RdsKeyPrefixWeiboLikeSet                     = "weibo_like:set"
	RdsKeyPrefixWeiboLikeHash                    = "weibo_like:hash"
	RdsKeyPrefixWeiboReplyList                   = "weibo_reply:list"
	RdsKeyPrefixWeiboReplySet                    = "weibo_reply:set"
	RdsKeyPrefixFriendSet                        = "friend:set"
	RdsKeyPrefixBlackListSet                     = "blackList:set"
	RdsKeyPrefixDontLookFriendWeiboSet           = "dont_look_friend_weibo:set"
	RdsKeyPrefixForbiddenFriendLookWeiboSet      = "forbidden_friend_look_weibo:set"
	RdsKeyPrefixAllowStragerLookWeiboSet         = "allow_stranger_look_weibo:set"
	RdsKeyPrefixAllowFriendLookWeiboString       = "allow_friend_look_weibo:string"
	RdsKeyPrefixManagerLoginString               = "manager:string"
	RdsKeyPrefixVerificationRegisterString       = "verification_register:string"
	RdsKeyPrefixVerificationForgetPasswordString = "verification_forget_password:string"
	RdsKeyPrefixThirdPlatformSet                 = "third_platform:set"
	RdsKeyPrefixRegionRegularString              = "manager:region_regular:string"
	RdsKeyPrefixHighAgentSet                     = "agent:set"
	RdsKeyPrefixThirdPlatformLoginToken          = "third_platform_login_token:string"
	RdsKeyRegisterAccount                        = "registerstatus"
	RdskeyLoginAccount                           = "loginstatus"
)

// 保存个人简略信息到 redis
func (c *Client) SaveUsrBriefInfoToRedis() {
	ubi := &msg_struct.UserBriefInfo{}
	ubi.AccountId = c.account.AccountId
	ubi.Nickname = c.account.NickName
	ubi.UserName = c.account.UserName
	ubi.HeadPortrait = c.account.HeadPortrait

	buf, _ := function.ProtoMarshal(ubi, "msg_struct.UserBriefInfo")

	err := c.rdsClientBriefInfo.Set(c.GetStrAccount(), string(buf))
	if err != nil {
		log.Errorf(" 保存用户简略信息到 redis 错误: %v", err)
	}
}

// SaveFriendInfoToRedis 保存好友列表到 redis
func (c *Client) SaveFriendInfoToRedis() {
	for _, contact := range c.mapContact {
		_, err := c.rdsFriend.AddSetMembers(c.GetStrAccount(), strconv.FormatInt(contact.AccountId, 10))
		if nil != err {
			log.Errorf("用户%d好友列表存入 redis出错：%v", err)
		}
	}
}

// 从 redis 中读取好友个人信息
func (c *Client) GetFriendInfoFromRedis() []*msg_struct.FriendInfo {
	var friendInfo []*msg_struct.FriendInfo
	friendIds := c.GetFriendStrIds()
	binFriendInfos, _ := c.rdsClientBriefInfo.MGet(friendIds)
	for i := 0; i < len(binFriendInfos); i++ {
		if binFriendInfos[i] != "" {
			userBI := &msg_struct.UserBriefInfo{}
			if function.ProtoUnmarshal([]byte(binFriendInfos[i]), userBI, "msg_struct.UserBriefInfo") {
				f := &msg_struct.FriendInfo{
					AccountId:   userBI.AccountId,
					NickName:    userBI.Nickname,
					RemarkName:  c.GetFriendRemarkName(userBI.GetAccountId()),
					PortraitUrl: c.GetUserHeadPortraitUrl(userBI.GetAccountId()),
				}

				friendInfo = append(friendInfo, f)
				log.Tracef("account: %d 登录时, 读取到好友 account: %d, 昵称: %s",
					c.GetAccountId(), f.AccountId, f.NickName)
			}
		}
	}

	return friendInfo
}

// 用户加入聊天群; 从 redis 中读取群信息并加载到内存
func (c *Client) JoinChatGroup(chatGroupId int64) {
	strChatGroupId := strconv.Itoa(int(chatGroupId))
	binChatGroup, _ := c.rdsChatGroupInfo.Get(strChatGroupId)
	if binChatGroup != "" {
		p := &msg_struct.ChatGroup{}
		if function.ProtoUnmarshal([]byte(binChatGroup),
			p, "msg_struct.ChatGroup") {
			c.AddChatGroup(p)
		}
	}
}

// 启动时从 redis 中读取群信息
func (c *Client) LoadChatGroupInfo() {
	rows, err := c.orm.DB().Query("select chat_group_id, last_msg_timestamp from chat_group_member where account_id = ?", c.GetAccountId())
	if err != nil {
		log.Errorf("读取 chat_group_member 表出错: %v", err)
		return
	}

	defer rows.Close()

	chatGroupIds := []string{}
	mapChatGroupLastMsgTimestamp := map[int64]int64{}
	for rows.Next() {
		var groupId, lastMsgTimestamp int64
		err = rows.Scan(&groupId, &lastMsgTimestamp)
		if err != nil {
			log.Errorf("读取 chat_group_member 的 chat_group_id, last_msg_timestamp 字段出错: %v", err)
			return
		}
		mapChatGroupLastMsgTimestamp[groupId] = lastMsgTimestamp
		chatGroupIds = append(chatGroupIds, strconv.Itoa(int(groupId)))

	}

	binChatGroups, _ := c.rdsChatGroupInfo.MGet(chatGroupIds)
	for i := 0; i < len(binChatGroups); i++ {
		if binChatGroups[i] != "" {
			p := &msg_struct.ChatGroup{}
			if function.ProtoUnmarshal([]byte(binChatGroups[i]),
				p, "msg_struct.ChatGroup") {
				c.AddChatGroup(p)
			}
		}
	}

	c.mapChatGroupLastMsgTimestamp = mapChatGroupLastMsgTimestamp
}

func (c *Client) GetBriefInfoFromRds() (*msg_struct.UserBriefInfo, error) {
	str, err := c.rdsClientBriefInfo.Get(c.GetStrAccount())
	if err != nil {
		return nil, err
	}

	p := &msg_struct.UserBriefInfo{}
	proto.Unmarshal([]byte(str), p)
	return p, nil
}

func (c *Client) GetOtherBriefInfoFromRds(accountId int64) (*msg_struct.UserBriefInfo, error) {
	strAccount := strconv.Itoa(int(accountId))
	str, err := c.rdsClientBriefInfo.Get(strAccount)
	if err != nil {
		return nil, err
	}

	p := &msg_struct.UserBriefInfo{}
	proto.Unmarshal([]byte(str), p)
	return p, nil
}

func (c *Client) AddChatGroup(chatGroup *msg_struct.ChatGroup) {
	c.lock.Lock()
	c.mapChatGroup[chatGroup.ChatGroupId] = chatGroup
	c.lock.Unlock()
}
