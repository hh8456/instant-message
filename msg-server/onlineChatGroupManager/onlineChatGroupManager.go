package onlineChatGroupManager

import (
	"fmt"
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"instant-message/common-library/proto/msg_id"
	"instant-message/common-library/proto/msg_struct"
	"instant-message/im-library/redisPubSub"
	"instant-message/msg-server/client"
	"instant-message/msg-server/clientManager"
	"instant-message/msg-server/eventmanager"
	"strconv"
	"sync"
	"time"

	"github.com/hh8456/go-common/redisObj"
	"github.com/hh8456/redisSession"
)

const systemName = "onlineChatGroupManager"

var (
	lock                     sync.RWMutex
	mmLocalOnlineGroupMember map[int64]map[int64]interface{} //  groupId( 本地聊天群id ) - 在线用户id
	rdsSessChatGroupPos      *redisSession.RedisSession      // 在线聊天群(在线群成员数 > 0)分布在哪些 topic 下
	localTopic               string
)

func Init(topic string) {
	subscribeEvent()
	localTopic = topic
	rdsSessChatGroupPos = redisObj.NewSessionWithPrefix("chat_group_position")

	// 获取所有的群位置 set
	keys, e := rdsSessChatGroupPos.Keys("*")
	if e != nil {
		panic(fmt.Sprintf("执行 redis 指令 keys chat_group_position:* 出错: %s", e.Error()))
	}

	// 启动时,把 redis set < 本地聊天群id > 中的残留信息清空
	rdsTmp := redisObj.NewSessionWithPrefix("")
	if 0 != len(keys) {
		for i := 0; i < len(keys); i++ {
			rdsTmp.RemoveSetMembers(fmt.Sprintf("%s", keys[i]), localTopic) // keys[i] 是聊天群 id, localTopic 是本地监听地址
		}
	}

	// 为 redis 中表示群位置的 set, 设置过期时间
	go func() {
		// XXX 暂时把过期时间设短,便于测试期间发现问题, 稳定后再设置长一些
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				{
					lock.RLock()
					for groupId, _ := range mmLocalOnlineGroupMember {
						rdsSessChatGroupPos.Expire(strconv.FormatInt(groupId, 10), time.Minute*2)
					}
					lock.RUnlock()
				}
			}
		}
	}()
}

func init() {
	mmLocalOnlineGroupMember = map[int64]map[int64]interface{}{}
}

func subscribeEvent() {
	eventmanager.Subscribe(systemName, eventmanager.ClientLoginEvent, onClientLoginEvent)   // 用户登录
	eventmanager.Subscribe(systemName, eventmanager.ClientLogoutEvent, onClientLogoutEvent) // 用户下线
	// XXX 尹延补充 用户加入聊天群, 作为群主建群时, 作为成员主动加入或者被别人邀请加入
	eventmanager.Subscribe(systemName, eventmanager.JoinChatGroupEvent, onJoinChatGroupEvent)
	// XXX 尹延补充 群主解散群, XXX 使用群聊接口通知,不需要对每个玩家通知
	eventmanager.Subscribe(systemName, eventmanager.UnChatGroupEvent, onUnChatGroupEvent)
	// XXX 尹延补充 用户退出群, 主动退出或者被群主踢出群, XXX 使用群聊接口通知,不需要对每个玩家通知
	eventmanager.Subscribe(systemName, eventmanager.CancelChatGroupEvent, onCancelChatGroupEvent)
	eventmanager.Subscribe(systemName, eventmanager.C2SGroupChatEvent, onC2SGroupChatEvent) // 发送群聊信息
}

func onClientLoginEvent(eventArg *eventmanager.EventArg) {
	userData := eventArg.GetUserData("client")
	cli := userData.(*client.Client)
	// 用户的群id
	groupIds := cli.GetChatGroupIds()
	accountId := cli.GetAccountId()

	localChatGroupMemberLogin(groupIds, accountId)
}

func onClientLogoutEvent(eventArg *eventmanager.EventArg) {
	userData := eventArg.GetUserData("client")
	cli := userData.(*client.Client)
	// 如果 app 端和 web 端,其中一端在线,那么就不需要从群缓存中把 accountId 清除
	if clientManager.GetClient(cli.GetAccountId(), clientManager.TerminalTypeApp) != nil ||
		clientManager.GetClient(cli.GetAccountId(), clientManager.TerminalTypeApp) != nil {
		// 用户的群id
		groupIds := cli.GetChatGroupIds()
		accountId := cli.GetAccountId()
		localChatGroupMemberLogout(groupIds, accountId)

	}
}

// 加入群
func onJoinChatGroupEvent(eventArg *eventmanager.EventArg) {
	groupId := eventArg.GetInt64("groupid")
	cli := eventArg.GetUserData("client").(*client.Client)
	cli.JoinChatGroup(groupId)
	accountId := cli.GetAccountId()
	b := false
	lock.Lock()
	if m, find := mmLocalOnlineGroupMember[groupId]; find {
		m[accountId] = nil
	} else {
		m_ := map[int64]interface{}{}
		m_[accountId] = nil
		mmLocalOnlineGroupMember[groupId] = m_
		b = true
	}
	lock.Unlock()

	if b {
		rdsSessChatGroupPos.AddSetMembers(strconv.FormatInt(groupId, 10), localTopic)
		rdsSessChatGroupPos.Expire(strconv.FormatInt(groupId, 10), time.Minute*2)
	}
}

// 群主解散群
func onUnChatGroupEvent(eventArg *eventmanager.EventArg) {
	groupId := eventArg.GetInt64("groupid")
	lock.Lock()
	delete(mmLocalOnlineGroupMember, groupId)
	lock.Unlock()
	rdsSessChatGroupPos.RemoveSetMembers(strconv.FormatInt(groupId, 10), localTopic)
}

// 用户退群
func onCancelChatGroupEvent(eventArg *eventmanager.EventArg) {
	accountId := eventArg.GetInt64("accountid")
	groupId := eventArg.GetInt64("groupid")
	b := false
	lock.Lock()
	if m, find := mmLocalOnlineGroupMember[groupId]; find {
		delete(m, accountId)
		if len(m) == 0 {
			delete(mmLocalOnlineGroupMember, groupId)
			b = true
		}
	}

	lock.Unlock()

	if b {
		rdsSessChatGroupPos.RemoveSetMembers(strconv.FormatInt(groupId, 10), localTopic)
	}
}

// 发送群聊信息
func onC2SGroupChatEvent(eventArg *eventmanager.EventArg) {
	userData := eventArg.GetUserData("chatgroupmsg")
	chatGroupMsg := userData.(*msg_struct.GroupMessage)
	SendToLocalChatGroup(chatGroupMsg.ReceiveGroupId, msg_id.NetMsgId_S2CRecvGroupChat, chatGroupMsg)
	sendToRemoteChatGroup(chatGroupMsg.ReceiveGroupId, msg_id.NetMsgId_MQGroupChat, chatGroupMsg)
}

// 本地群成员上线
func localChatGroupMemberLogin(groupIds []int64, accountId int64) {
	ids := make([]int64, 0, 4)
	lock.Lock()
	for _, groupId := range groupIds {
		//mmLocalOnlineGroupMember map[int64]map[int64]interface{} //  groupId( 聊天群id ) - 在线用户
		if m, find := mmLocalOnlineGroupMember[groupId]; find {
			m[accountId] = nil
		} else {
			m_ := map[int64]interface{}{}
			m_[accountId] = nil
			mmLocalOnlineGroupMember[groupId] = m_
			ids = append(ids, groupId)
		}
	}
	lock.Unlock()

	for _, groupId := range ids {
		rdsSessChatGroupPos.AddSetMembers(strconv.FormatInt(groupId, 10), localTopic)
		rdsSessChatGroupPos.Expire(strconv.FormatInt(groupId, 10), time.Minute*2)
	}
}

// 本地群成员下线
func localChatGroupMemberLogout(groupIds []int64, accountId int64) {
	ids := make([]int64, 0, 4)
	lock.Lock()
	for _, groupId := range groupIds {
		//mmLocalOnlineGroupMember map[int64]map[int64]interface{} //  groupId( 聊天群id ) - 在线用户
		if m, find := mmLocalOnlineGroupMember[groupId]; find {
			delete(m, accountId)
			if len(m) == 0 {
				ids = append(ids, groupId)
			}
		}
	}
	lock.Unlock()

	for _, groupId := range ids {
		rdsSessChatGroupPos.RemoveSetMembers(strconv.FormatInt(groupId, 10), localTopic)
	}
}

// 向远程服务器发送群聊消息
func sendToRemoteChatGroup(groupId int64, cmdId msg_id.NetMsgId, chatGroupMsg *msg_struct.GroupMessage) {
	topices, err := rdsSessChatGroupPos.GetSetMembers(strconv.FormatInt(groupId, 10))
	if err != nil {
		return
	}

	buf, b := function.ProtoMarshal(chatGroupMsg, "msg_struct.GroupMessage")
	if b {
		for i := 0; i < len(topices); i++ {
			strTopic := fmt.Sprintf("%s", topices[i])
			if strTopic != localTopic {
				if err := redisPubSub.SendGroupBuf(strTopic, groupId, cmdId, buf); nil != err {
					log.Errorf("向 MQ 发送群聊消息失败: %s", err.Error())
				}
			}
		}

	}
}

// 向本地聊天群发消息
func SendToLocalChatGroup(groupId int64, cmdId msg_id.NetMsgId, chatGroupMsg *msg_struct.GroupMessage) {
	members := []int64{}
	lock.RLock()
	//mmLocalOnlineGroupMember map[int64]map[int64]interface{} //  groupId( 聊天群id ) - 在线用户
	if mapMember, find := mmLocalOnlineGroupMember[groupId]; find {
		for accountId, _ := range mapMember {
			members = append(members, accountId)
		}
	}
	lock.RUnlock()

	for _, member := range members {
		cli := clientManager.GetClient(member, clientManager.TerminalTypeApp)
		// 向其他群友发送群聊消息, 不需要向自己发送
		if cli != nil && member != chatGroupMsg.SenderId {
			cli.SendPbMsg(cmdId, &msg_struct.S2CRecvGroupChat{
				ChatMsg: chatGroupMsg,
			})
		}

		cli = clientManager.GetClient(member, clientManager.TerminalTypeWeb)
		// 向其他群友发送群聊消息, 不需要向自己发送
		if cli != nil && member != chatGroupMsg.SenderId {
			cli.SendPbMsg(cmdId, &msg_struct.S2CRecvGroupChat{
				ChatMsg: chatGroupMsg,
			})
		}

	}
}
