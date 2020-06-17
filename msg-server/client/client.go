package client

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"instant-message/common-library/models"
	"instant-message/common-library/packet"
	"instant-message/common-library/proto/msg_err_code"
	"instant-message/common-library/proto/msg_id"
	"instant-message/common-library/proto/msg_struct"
	"instant-message/im-library/im_net"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gomodule/redigo/redis"

	"github.com/go-xorm/xorm"
	"github.com/gogo/protobuf/proto"
	"github.com/hh8456/go-common/cache"
	"github.com/hh8456/go-common/redisObj"
	"github.com/hh8456/redisSession"
)

var (
	gSerialNum int64
)

type Client struct {
	account                        *models.Account
	strAccount                     string
	socket                         *im_net.Socket
	chanToClientMsg                chan []byte // 发送缓冲区,存放的数据, 需要下发给客户端
	chanRecvMsg                    chan []byte // 接收缓冲区
	writer                         *bufio.Writer
	lock                           sync.RWMutex
	isClosed                       bool
	closeSignal                    chan interface{}
	done                           chan interface{}
	rdsPrivateChatRecord           *redisSession.RedisSession // 私聊记录
	rdsUid                         *redisSession.RedisSession
	rdsClientBriefInfo             *redisSession.RedisSession // XXX 需要做注入工具
	rdsGroupChatRecord             *redisSession.RedisSession // 群聊记录
	rdsChatGroupInfo               *redisSession.RedisSession // 存放 mysql 表 chat_group(聊天群信息) 中的数据, XXX 需要做注入工具
	rdsPushInfo                    *redisSession.RedisSession // 存放推送相关信息
	rdsUserNameInfo                *redisSession.RedisSession // 存放用户名-帐号
	rdsPhoneInfo                   *redisSession.RedisSession // 存放手机号-帐号
	rdsOnline                      *redisSession.RedisSession // 管理用户在线信息
	rdsFriend                      *redisSession.RedisSession // 好友列表
	rdsBlackList                   *redisSession.RedisSession // 黑名单列表
	rdsHeadPortraitUrl             *redisSession.RedisSession // 用户头像地址
	rdsHuanXin                     *redisSession.RedisSession // 用户环信信息
	rdsAgent                       *redisSession.RedisSession // 用户代理信息（用于兼容第三方平台）
	rdsWeiboCover                  *redisSession.RedisSession // 朋友圈相册封面
	rdsWeiboCoverLiker             *redisSession.RedisSession // 朋友圈相册封面点赞
	rdsWeiboList                   *redisSession.RedisSession // 朋友圈
	rdsWeiboSet                    *redisSession.RedisSession // 朋友圈
	rdsWeiboString                 *redisSession.RedisSession // 朋友圈
	rdsWeiboZset                   *redisSession.RedisSession // 朋友圈
	rdsWeiboLikeHash               *redisSession.RedisSession // 朋友圈点赞
	rdsWeiboLikeSet                *redisSession.RedisSession // 朋友圈点赞
	rdsWeiboReplyList              *redisSession.RedisSession // 朋友圈评论
	rdsWeiboReplySet               *redisSession.RedisSession // 朋友圈评论
	rdsDontLookWeiboSet            *redisSession.RedisSession // 不看某人的朋友圈
	rdsForbiddenFriendLookWeiboSet *redisSession.RedisSession // 不让某人看朋友圈
	rdsStrangerLookWeiboSet        *redisSession.RedisSession // 陌生人查看朋友圈权限
	rdsFriendLookWeiboString       *redisSession.RedisSession // 好友查看朋友圈权限
	orm                            *xorm.Engine
	mapContact                     map[int64]*msg_struct.Contact   // 联系人列表
	agentInfo                      *msg_struct.AgentInfo           // 上级代理
	mapMember                      map[int64]*msg_struct.AgentInfo // 下级会员
	mapBlackList                   map[int64]*msg_struct.Contact   // 黑名单列表
	mapChatGroup                   map[int64]*msg_struct.ChatGroup // 聊天群列表
	mapChatGroupLastMsgTimestamp   map[int64]int64                 // 收到的最后一条群聊时间戳
	loginTimestamp                 int64
	serialNum                      int64
	terminalType                   int32 // 终端类型, 1 手机, 2 web
	chatRecord                     *cache.Cache
}

func NewClient(account *models.Account, conn net.Conn, s *im_net.Socket,
	ormObj *xorm.Engine, terminalType int32, chatRecord *cache.Cache) *Client {

	mc := make(map[int64]*msg_struct.Contact)
	cl := &msg_struct.ContactList{}
	if function.ProtoUnmarshal(account.ContactList, cl, "msg_struct.ContactList") {
		for _, v := range cl.Contacts {
			mc[v.AccountId] = &msg_struct.Contact{
				AccountId:  v.AccountId,
				RemarkName: v.RemarkName,
			}
		}
	}

	mapBlackList := make(map[int64]*msg_struct.Contact)
	blackList := &msg_struct.BlackList{}
	if function.ProtoUnmarshal(account.BlackList, blackList, "msg_struct.BlackList") {
		for _, contact := range blackList.Contacts {
			mapBlackList[contact.AccountId] = &msg_struct.Contact{
				AccountId:  contact.AccountId,
				RemarkName: contact.RemarkName,
			}
		}
	}

	return &Client{account: account,
		strAccount:                     strconv.Itoa(int(account.AccountId)),
		socket:                         s,
		chanToClientMsg:                make(chan []byte, 100),
		chanRecvMsg:                    make(chan []byte, 100),
		writer:                         bufio.NewWriter(conn),
		closeSignal:                    make(chan interface{}),
		done:                           make(chan interface{}),
		rdsPrivateChatRecord:           redisObj.NewSessionWithPrefix(RdsKeyPrefixPrivateChatRecord), // 私聊记录
		rdsUid:                         redisObj.NewSessionWithPrefix(RdsKeyPrefixUid),
		rdsClientBriefInfo:             redisObj.NewSessionWithPrefix(RdsKeyPrefixBriefInfo),
		rdsGroupChatRecord:             redisObj.NewSessionWithPrefix(RdsKeyPrefixGroupChatRecord),             // 群聊记录
		rdsChatGroupInfo:               redisObj.NewSessionWithPrefix(RdsKeyPrefixChatGroupInfo),               // 聊天群信息
		rdsPushInfo:                    redisObj.NewSessionWithPrefix(RdsKeyPrefixPushInfo),                    // 推送相关
		rdsUserNameInfo:                redisObj.NewSessionWithPrefix(RdsKeyPrefixUserNameInfo),                // 用户名-帐号
		rdsPhoneInfo:                   redisObj.NewSessionWithPrefix(RdsKeyPrefixPhoneInfo),                   // 手机号-帐号
		rdsOnline:                      redisObj.NewSessionWithPrefix(RdsKeyPrefixUserOnline),                  // 用户在线管理
		rdsFriend:                      redisObj.NewSessionWithPrefix(RdsKeyPrefixFriendSet),                   // 好友列表
		rdsBlackList:                   redisObj.NewSessionWithPrefix(RdsKeyPrefixBlackListSet),                // 黑名单列表
		rdsHeadPortraitUrl:             redisObj.NewSessionWithPrefix(RdsKeyPrefixHeadPortraitUrl),             // 用户头像
		rdsHuanXin:                     redisObj.NewSessionWithPrefix(RdsKeyPrefixHuanXinString),               // 用户环信信息
		rdsAgent:                       redisObj.NewSessionWithPrefix(RdsKeyPrefixHighAgentSet),                // 用户代理信息（用于兼容第三方平台）
		rdsWeiboCover:                  redisObj.NewSessionWithPrefix(RdsKeyPrefixWeiboCover),                  // 朋友圈封面相册
		rdsWeiboCoverLiker:             redisObj.NewSessionWithPrefix(RdsKeyPrefixWeiboCoverLiker),             // 朋友圈封面相册点赞
		rdsWeiboList:                   redisObj.NewSessionWithPrefix(RdsKeyPrefixWeiboList),                   // 朋友圈管理
		rdsWeiboSet:                    redisObj.NewSessionWithPrefix(RdsKeyPrefixWeiboSet),                    // 朋友圈管理
		rdsWeiboString:                 redisObj.NewSessionWithPrefix(RdsKeyPrefixWeiboString),                 // 朋友圈管理
		rdsWeiboZset:                   redisObj.NewSessionWithPrefix(RdsKeyPrefixWeiboZset),                   // 朋友圈管理
		rdsWeiboLikeHash:               redisObj.NewSessionWithPrefix(RdsKeyPrefixWeiboLikeHash),               // 朋友圈点赞管理
		rdsWeiboLikeSet:                redisObj.NewSessionWithPrefix(RdsKeyPrefixWeiboLikeSet),                // 朋友圈点赞管理
		rdsWeiboReplyList:              redisObj.NewSessionWithPrefix(RdsKeyPrefixWeiboReplyList),              // 朋友圈评论管理
		rdsWeiboReplySet:               redisObj.NewSessionWithPrefix(RdsKeyPrefixWeiboReplySet),               // 朋友圈评论管理
		rdsDontLookWeiboSet:            redisObj.NewSessionWithPrefix(RdsKeyPrefixDontLookFriendWeiboSet),      // 不看某人的朋友圈
		rdsForbiddenFriendLookWeiboSet: redisObj.NewSessionWithPrefix(RdsKeyPrefixForbiddenFriendLookWeiboSet), // 不让某人看朋友圈
		rdsStrangerLookWeiboSet:        redisObj.NewSessionWithPrefix(RdsKeyPrefixAllowStragerLookWeiboSet),    // 陌生人查看朋友圈权限
		rdsFriendLookWeiboString:       redisObj.NewSessionWithPrefix(RdsKeyPrefixAllowFriendLookWeiboString),  // 好友查看朋友圈权限
		orm:                            ormObj,
		mapContact:                     mc,
		agentInfo:                      &msg_struct.AgentInfo{},
		mapMember:                      map[int64]*msg_struct.AgentInfo{},
		mapBlackList:                   mapBlackList,
		mapChatGroup:                   map[int64]*msg_struct.ChatGroup{},
		mapChatGroupLastMsgTimestamp:   map[int64]int64{},
		loginTimestamp:                 time.Now().UnixNano(),
		serialNum:                      atomic.AddInt64(&gSerialNum, 1),
		terminalType:                   terminalType,
		chatRecord:                     chatRecord,
	}
}

func (c *Client) Run() {
	go c.recvLoop()
	go c.sendLoop()
	go c.handleMsg()
}

func (c *Client) Done() {
	close(c.done)
}

func (c *Client) GetDoneSignal() chan interface{} {
	return c.done
}

func (c *Client) GetCloseSignal() chan interface{} {
	return c.closeSignal
}

func (c *Client) GetAccountId() int64 {
	return c.account.AccountId
}

// 接收客户端发来的数据,并酌情投递给 MQ
func (c *Client) recvLoop() {
	defer func() {
		log.Tracef("account: %d 退出登录", c.GetAccountId())
		function.Catch()
	}()
	for {
		msg, err := c.socket.ReadOne()
		if err != nil {
			if err != io.EOF {
				log.Errorf("Client.recvLoop read error: %s, accountId: %v", err, c.GetAccountId())
			}

			c.Close()
			return
		}

		select {
		case c.chanRecvMsg <- msg:

		default:
			log.Errorf("client.chanRecvMsg 通道容量满了, 导致客户端接收的消息溢出")
		}
	}
}

// 接收 MQ 发来的数据,并下发给客户端
func (c *Client) sendLoop() {
	defer function.Catch()

	for {
		select {
		case msg, ok := <-c.chanToClientMsg:
			if !ok {
				//log.Tracef("client[%v] exit sendLoop", c.accountId)
				return
			}

			_, e := c.writer.Write(msg)
			if e != nil {
				log.Errorf("发送数据出错: %v ", e)
			}

			exitLoop := false
			for {
				select {
				case msg, ok := <-c.chanToClientMsg:
					if !ok {
						//log.Tracef("client[%v] exit sendLoop", c.accountId)
						return
					}

					_, e := c.writer.Write(msg)
					if e != nil {
						log.Errorf("发送数据出错: %v ", e)
					}

				default:
					exitLoop = true
				}

				if exitLoop {
					break
				}
			}

			err := c.writer.Flush()
			if err != nil {
				log.Errorf("client[%v] bufio.Writer.Flush error: %v", c.account.AccountId, err)
			}

		case <-c.closeSignal:
			return

		}

	}
}

func (c *Client) HandleMsg(msg []byte) {
	select {
	case c.chanRecvMsg <- msg:

	default:
		log.Errorf("client.chanRecvMsg 通道容量满了, 导致客户端接收的消息溢出")
	}
}

func (c *Client) SendBuf(msg []byte) {
	select {
	case c.chanToClientMsg <- msg:

	default:
		log.Errorf("client.chanToClientMsg 通道容量满了, 导致客户端收不到消息")
	}
}

func (c *Client) SendPbMsg(cmdId msg_id.NetMsgId, pb proto.Message) {
	msg, err := proto.Marshal(pb)

	if err == nil {
		msgLen := len(msg)

		buf := make([]byte, packet.PacketHeaderSize+msgLen)
		binary.BigEndian.PutUint32(buf, uint32(cmdId))
		binary.BigEndian.PutUint32(buf[4:], uint32(msgLen))
		copy(buf[packet.PacketHeaderSize:], msg)
		fmt.Printf("Send Client %d, addr=%s, Message:[", c.GetAccountId(), c.socket.RemoteAddr())
		for i := 0; i < len(buf); i++ {
			fmt.Printf("%x ", buf[i])
		}
		fmt.Printf("], messageId = %d\n", cmdId)

		select {
		case c.chanToClientMsg <- buf:

		default:
			log.Errorf("client.chanToClientMsg 通道容量满了, 导致客户端收不到消息")
		}

	} else {
		log.Errorf("向客户端发送数据失败,因为 protobuf 序列化出错, accountId: %d, cmdId: %d, err: %v", c.account.AccountId, int32(cmdId), err)

	}
}

// 阻塞式发送
func (c *Client) BlockingSendPbMsg(cmdId msg_id.NetMsgId, pb proto.Message) {
	msg, err := proto.Marshal(pb)

	if err == nil {
		msgLen := len(msg)
		buf := make([]byte, packet.PacketHeaderSize+msgLen)
		binary.BigEndian.PutUint32(buf, uint32(cmdId))
		binary.BigEndian.PutUint32(buf[4:], uint32(msgLen))
		copy(buf[packet.PacketHeaderSize:], msg)
		c.socket.Write(buf)
	} else {
		log.Errorf("向客户端发送数据失败,因为 protobuf 序列化出错, accountId: %d, cmdId: %d, err: %v", c.account.AccountId, int32(cmdId), err)

	}
}

func (c *Client) Close() {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.isClosed == true {
		return
	}

	c.isClosed = true
	c.socket.Close()
	// 通知 SendLoop 协程退出
	close(c.closeSignal)

	c.SaveBaseInfo()
	c.SaveChatGroupLastMsgTimestamp()
	DelClientOnlieExpire(c, c.GetAccountId())
}

func (c *Client) isClose() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.isClosed
}

func (c *Client) handleMsg() {
	for {
		select {
		case msg := <-c.chanRecvMsg:
			{
				msgId := binary.BigEndian.Uint32(msg)
				f, ok := getMsgFunc(msg_id.NetMsgId(msgId))
				if ok {
					fmt.Printf("Recv From Client=%d, addr=%s, Message16:[", c.GetAccountId(), c.socket.RemoteAddr())
					for i := 0; i < len(msg); i++ {
						fmt.Printf("%x ", msg[i])
					}
					fmt.Printf("], messageId = %d\n", msgId)
					f(c, msg)
				} else {
					log.Errorf("收到未定义的消息 id: %d, accountId: %d", msgId, c.account.AccountId)
				}
			}

		case <-c.closeSignal:
			return
		}
	}
}

func (c *Client) IsFriends(userIds []int64) []int64 {
	friendIds := make([]int64, 0)
	c.lock.RLock()
	defer c.lock.RUnlock()
	for _, userid := range userIds {
		if _, ok := c.mapContact[userid]; ok {
			friendIds = append(friendIds, userid)
		}
	}
	return friendIds
}

func (c *Client) IsFriend(userId int64) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	_, ok := c.mapContact[userId]
	return ok
}

func (c *Client) BeFriend(userId int64) {
	c.lock.Lock()
	c.mapContact[userId] = &msg_struct.Contact{
		AccountId:  userId,
		RemarkName: "",
	}
	c.lock.Unlock()
}

func (c *Client) BeNotFriend(userId int64) {
	c.lock.Lock()
	delete(c.mapContact, userId)
	c.lock.Unlock()
}

func (c *Client) GetLastMsgTimestamp() int64 {
	return c.account.LastMsgTimestamp
}

func (c *Client) GetUserName() string {
	return c.account.UserName
}

func (c *Client) GetStrAccount() string {
	return c.strAccount
}

func (c *Client) GetLoginTimestamp() int64 {
	return c.loginTimestamp
}

func (c *Client) SendErrorCode(errCode msg_err_code.MsgErrCode, msgTimeStamp int64) {
	var errNotice string
	c.SendPbMsg(msg_id.NetMsgId_S2CErrorCode,
		&msg_struct.S2CErrorCode{
			ErrCode:      int64(errCode),
			MsgTimeStamp: msgTimeStamp,
			ErrNotice:    errNotice,
		})
}

func (c *Client) GetChatGroupIds() []int64 {
	groupIds := []int64{}

	c.lock.RLock()
	for groupId, _ := range c.mapChatGroup {
		groupIds = append(groupIds, groupId)
	}
	c.lock.RUnlock()

	return groupIds
}

func (c *Client) GetFriendIds() []int64 {
	var ids []int64
	c.lock.RLock()
	for id, _ := range c.mapContact {
		ids = append(ids, id)
	}
	c.lock.RUnlock()
	return ids
}

func (c *Client) GetFriendStrIds() []string {
	var ids []string
	c.lock.RLock()
	for id, _ := range c.mapContact {
		ids = append(ids, strconv.FormatInt(id, 10))
	}
	c.lock.RUnlock()
	return ids
}

func (c *Client) GetName() string {
	return c.account.NickName
}

func (c *Client) SetName(newName string) {
	c.account.NickName = newName
}

func (c *Client) GetSerialNum() int64 {
	return c.serialNum
}

func (c *Client) GetChatGroupInfo(chatGroupId int64) *msg_struct.ChatGroup {
	c.lock.RLock()
	defer c.lock.RUnlock()
	if v, ok := c.mapChatGroup[chatGroupId]; ok {
		return v
	}

	return nil
}

func (c *Client) GetAddFriendOfflineRequest() ([]*msg_struct.FriendRequest, error) {
	rows, err := c.orm.DB().Query("select sender_id, req_datetime from friend_req WHERE receiver_id = ?", c.GetAccountId())

	if nil != err {
		log.Errorf("执行 orm 查询时出错: %v", err)
		return nil, err
	}

	defer rows.Close()

	friendRequests := []*msg_struct.FriendRequest{}
	for rows.Next() {
		var senderId int64
		var reqDateTime time.Time
		if err := rows.Scan(&senderId, &reqDateTime); nil != err {
			log.Errorf("执行 orm 查询时, 获取数据出错: %v", err)
			return nil, err
		}

		p, err := c.QueryOtherBriefInfo(senderId)
		if err != nil {
			log.Errorf("account: %d 登录时从 redis 中获取其他用户( id ) %d 时出错, error: %s",
				c.GetAccountId(), senderId, err.Error())
			continue
		}

		friendRequests = append(friendRequests, &msg_struct.FriendRequest{
			SenderId:           senderId,
			SenderNickName:     p.Nickname,
			ReqDateTime:        reqDateTime.Unix(),
			SenderHeadPortrait: c.GetUserHeadPortraitUrl(senderId),
		})
	}

	return friendRequests, nil
}

func (c *Client) QueryOtherBriefInfo(accountId int64) (*msg_struct.UserBriefInfo, error) {
	strId := strconv.Itoa(int(accountId))
	str, err := c.rdsClientBriefInfo.Get(strId)
	if err != nil {
		log.Errorf("account: %d 从 redis 中获取其他用户( id ) %d 时出错, error: %s",
			c.GetAccountId(), accountId, err.Error())
		return nil, err
	}

	p := &msg_struct.UserBriefInfo{}
	err = proto.Unmarshal([]byte(str), p)

	return p, err

}

func (c *Client) QueryOthersBriefInfos(accountIds []int64) ([]*msg_struct.UserBriefInfo, error) {
	userBriefInfos := make([]*msg_struct.UserBriefInfo, 0)
	for _, accountId := range accountIds {
		userBriefInfo, err := c.QueryOtherBriefInfo(accountId)
		if nil != err {
			return nil, err
		} else {
			userBriefInfos = append(userBriefInfos, userBriefInfo)
		}
	}
	return userBriefInfos, nil
}

func (c *Client) GetChatGroupLastMsgTimestamp(chatGroupId int64) (bool, int64) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	v, ok := c.mapChatGroupLastMsgTimestamp[chatGroupId]
	if ok {
		return true, v
	}

	return false, 0
}

func (c *Client) SetChatGroupLastMsgTimestamp(chatGroupId, lastMsgTimestamp int64) {
	c.lock.Lock()
	v, ok := c.mapChatGroupLastMsgTimestamp[chatGroupId]
	if ok {
		if lastMsgTimestamp > v {
			c.mapChatGroupLastMsgTimestamp[chatGroupId] = lastMsgTimestamp
		} else {
			log.Errorf("account: %d 更新聊天群( chatGroupId: %d )的最后一条时间戳时出错, 新的时间戳( %d ) 应该比当前时间戳 ( %d ) 大", c.GetAccountId(), chatGroupId, lastMsgTimestamp, v)
		}
	} else {
		log.Errorf("account: %d 更新聊天群的最后一条时间戳时, 发现没有加入聊天群 ( chatGroupId: %d ) ", c.GetAccountId(), chatGroupId)
	}
	c.lock.Unlock()
}

// IsBlackList userId 是否在黑名单列表中
func (c *Client) IsBlackList(userId int64) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	_, ok := c.mapBlackList[userId]
	return ok
}

// BeBlackList userId 加入黑名单列表
func (c *Client) BeBlackList(userId int64) {
	friendRemarkName := c.GetFriendRemarkName(userId)
	c.lock.Lock()
	c.mapBlackList[userId] = &msg_struct.Contact{
		AccountId:  userId,
		RemarkName: friendRemarkName,
	}
	c.lock.Unlock()
}

// BeNotBlackList userId 移除黑名单列表
func (c *Client) BeNotBlackList(userId int64) {
	c.lock.Lock()
	delete(c.mapBlackList, userId)
	c.lock.Unlock()
}

// GetBlackListIds 获取所有黑名单用户id(int64)
func (c *Client) GetBlackListIds() []int64 {
	ids := make([]int64, 0)
	c.lock.RLock()
	for id, _ := range c.mapBlackList {
		ids = append(ids, id)
	}
	c.lock.RUnlock()
	return ids
}

// GetBlackListStrIds 获取所有黑名单用户id(string)
func (c *Client) GetBlackListStrIds() []string {
	var ids []string
	c.lock.RLock()
	for id, _ := range c.mapBlackList {
		ids = append(ids, strconv.FormatInt(id, 10))
	}
	c.lock.RUnlock()
	return ids
}

// SetFriendRemarkName 设置好友的备注名（调用此函数之前需检测是否为好友，如果不是好友，设置会失败，但无任何错误提示）
func (c *Client) SetFriendRemarkName(friendId int64, remarkName string) {
	c.lock.Lock()
	contact, ok := c.mapContact[friendId]
	if ok {
		contact.RemarkName = remarkName
	}
	c.lock.Unlock()
}

// GetFriendRemarkName 获取好友的备注名（调用此函数之前需检测是否为好友，如果不是好友，会返回空字符串）
func (c *Client) GetFriendRemarkName(friendId int64) string {
	c.lock.RLock()
	defer c.lock.RUnlock()
	contact, ok := c.mapContact[friendId]
	if ok {
		return contact.RemarkName
	}
	return ""
}

// GetUserHeadPortraitUrl 获取某人的头像url地址
func (c *Client) GetUserHeadPortraitUrl(userId int64) string {
	headPortraitUrl, err := c.rdsHeadPortraitUrl.Get(strconv.FormatInt(userId, 10))
	if nil != err {
		return ""
	}
	return headPortraitUrl
}

// GetHuanXinToken 获取环信token，如果redis中没有，重新调用环信接口，获取token，并更新到redis
func (c *Client) GetHuanXinToken() string {
	token, err := c.rdsHuanXin.Get("token")
	if nil != err {
		if redis.ErrNil == err {
			log.Warnf("从 redis 获取环信 token 失败: 环信 token 已过期")
		} else {
			log.Errorf("从 redis 获取环信 token 失败:%v, 重置 token", err)
		}

		response := GetHuanXinToken()
		if nil == response {
			return ""
		}
		_, _ = c.rdsHuanXin.Del("token")
		err := c.rdsHuanXin.Setex("token", time.Duration(response.ExpiresIn)*time.Second, response.AccessToken)
		if nil != err {
			log.Errorf("设置环信 token 存入 redis 失败:%v", err) // 此处不返回空，毕竟token在这一次是可用的
		}
		return response.AccessToken
	}
	return token
}

// ConfirmOwnHuanXinAccountExist 确保环信帐号正常，先从redis读取，失败再注册（若注册失败，则写日志）
func (c *Client) ConfirmOwnHuanXinAccountExist(token string) string {
	huanXinPassword, err := c.rdsHuanXin.Get(c.GetStrAccount())
	if nil != err {
		if redis.ErrNil == err {
			log.Warnf("用户%d从 redis 获取用户%d的环信帐号失败: 环信帐号可能还未注册,或者注册了但并未写入 redis", c.GetAccountId(), c.GetStrAccount())

			/* 正常情况下，程序运行到此处，是因为没有注册环信帐号，在此处进行注册 */
			huanXinNewPassword := fmt.Sprintf("%d%d", c.GetAccountId(), time.Now().Unix())
			_, err = CreateHuanXinAccount(token, c.GetStrAccount(), huanXinNewPassword, c.GetName())
			if nil != err {
				log.Errorf("用户%d注册环信账户时，注册失败：%v", c.GetAccountId(), err)
				return ""
			}

			if err := c.rdsHuanXin.Set(c.GetStrAccount(), huanXinNewPassword); nil != err {
				log.Errorf("用户%d注册环信账户时，注册成功，存入 redis 失败：%v", c.GetAccountId(), err)
			}
			return huanXinNewPassword
		} else {
			log.Errorf("用户%d从 redis 获取用户%d的环信帐号失败:%v", c.GetAccountId(), c.GetStrAccount(), err)
		}
	}
	return huanXinPassword
}

func (c *Client) GetTerminalType() int32 {
	return c.terminalType
}

func (c *Client) UpdateBlackListInfoInMemory() {
	blackLists := &msg_struct.BlackList{}
	for _, blackList := range c.mapBlackList {
		blackLists.Contacts = append(blackLists.Contacts, blackList)
	}
	buf, b := function.ProtoMarshal(blackLists, "msg_struct.BlackList")
	if b {
		c.account.BlackList = buf
	}

}

func (c *Client) UpdateContactInfoInMemory() {
	cl := &msg_struct.ContactList{}

	c.lock.RLock()
	for k, v := range c.mapContact {
		contact := &msg_struct.Contact{AccountId: k, RemarkName: v.RemarkName}
		cl.Contacts = append(cl.Contacts, contact)
	}
	c.lock.RUnlock()

	buf, b := function.ProtoMarshal(cl, "msg_struct.ContactList")
	if b {
		c.account.ContactList = buf
	}
}

func (c *Client) GetAgentAndMember() []*msg_struct.AgentInfo {
	agentInfos := make([]*msg_struct.AgentInfo, 0)

	// 获取上级代理
	var agentId int64
	err := c.orm.DB().QueryRow("SELECT agent_id FROM account WHERE account_id = ?", c.GetAccountId()).Scan(&agentId)
	if nil == err && 0 != agentId {
		var agentName string
		c.orm.DB().QueryRow("SELECT user_name FROM account WHERE account_id = ?", c.GetAccountId()).Scan(&agentName)
		c.agentInfo = &msg_struct.AgentInfo{
			AgentId:          agentId,
			AgentUsername:    agentName,
			AgentportraitUrl: c.GetUserHeadPortraitUrl(agentId),
		}
		log.Debug(fmt.Sprintf("account_id = %d, agent_id = %d", c.GetAccountId(), agentId))
		agentInfos = append(agentInfos, c.agentInfo)
	}

	// 获取下级会员
	rows, err := c.orm.DB().Query("SELECT account_id, user_name FROM account WHERE agent_id = ?", c.GetAccountId())
	if nil == err && nil != rows {
		for rows.Next() {
			var (
				accountId int64
				userName  string
			)
			rows.Scan(&accountId, &userName)
			c.mapMember[accountId] = &msg_struct.AgentInfo{
				AgentId:          accountId,
				AgentUsername:    userName,
				AgentportraitUrl: c.GetUserHeadPortraitUrl(accountId),
			}
		}
	}

	return agentInfos
}

func (c *Client) IsAgentOrMember(userId int64) bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.GetAgentAndMember()
	if c.agentInfo.AgentId == userId {
		return true
	}
	_, ok := c.mapMember[userId]
	return ok
}

func (c *Client) GetAgentOrMemberUsername(userId int64) string {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.GetAgentAndMember()
	if userId == c.agentInfo.AgentId {
		return c.agentInfo.AgentUsername
	}
	info, ok := c.mapMember[userId]
	if ok && nil != info {
		return info.AgentUsername
	}
	return ""
}
