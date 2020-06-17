package models

import (
	"time"
)

type ChatGroupMember struct {
	Id                    int64     `json:"id" xorm:"pk autoincr BIGINT(20)"`
	AccountId             int64     `json:"account_id" xorm:"not null unique(account_group) BIGINT(20)"`
	ChatGroupId           int64     `json:"chat_group_id" xorm:"not null unique(account_group) BIGINT(20)"`
	NickName              string    `json:"nick_name" xorm:"not null comment('群里的昵称') VARCHAR(60)"`
	JoinChatGroupDatetime time.Time `json:"join_chat_group_datetime" xorm:"not null comment('加入群的时间') DATETIME(6)"`
	LastMsgTimestamp      int64     `json:"last_msg_timestamp" xorm:"not null comment('收到的最后一条群消息时间戳') BIGINT(20)"`
}
