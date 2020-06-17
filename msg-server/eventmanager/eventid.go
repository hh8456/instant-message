package eventmanager

type EventID int

const (
	TestEvent EventID = 0
	//
	CreateCharacterSuccessEvent     EventID = 1
	ClientLoginEvent                EventID = 2  // 用户登录
	ClientLogoutEvent               EventID = 3  // 用户下线
	JoinChatGroupEvent              EventID = 4  // 用户加群
	UnChatGroupEvent                EventID = 5  // 群主解散群
	CancelChatGroupEvent            EventID = 6  // 用户离开群
	C2SGroupChatEvent               EventID = 7  // 发送群聊信息
	PublishWeibo                    EventID = 8  // 发朋友圈
	PrivateChatEvent                EventID = 9  // 用户发送私聊消息
	AgreeBecomeFriendEvent          EventID = 10 // 用户同意了其他人的好友请求
	ReadFriendReqEvent              EventID = 11 // 用户拒绝了其他人的好友请求
	C2SLastMsgTimeStampEvent        EventID = 12 // 用户把收到的最后一条群消息时间戳上报给服务器
	DeleteFriendEvent               EventID = 13 // 删除好友
	ConfirmDeleteFriendEvent        EventID = 14 // A 删除 B, B 发送 C2SConfirmDeleteFriend 确认删除 A
	C2SAddFriendBlackListEvent      EventID = 15 // 好友拉入黑名单
	S2CMoveOutFriendBlackListEvent  EventID = 16 // 好友移出黑名单列表
	S2CSetFriendRemarkNameEvent     EventID = 17 // 好友打备注
	PrivateChatWithDrawMessageEvent EventID = 18 // 用户撤回私聊消息
)
