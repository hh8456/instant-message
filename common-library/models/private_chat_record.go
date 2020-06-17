package models

import (
	"time"
)

type PrivateChatRecord struct {
	Id           int64     `json:"id" xorm:"pk autoincr BIGINT(20)"`
	SenderId     int64     `json:"sender_id" xorm:"not null index(senderId_receiverId_datetime) BIGINT(20)"`
	ReceiverId   int64     `json:"receiver_id" xorm:"not null index(senderId_receiverId_datetime) BIGINT(20)"`
	SrvTimestamp int64     `json:"srv_timestamp" xorm:"not null BIGINT(20)"`
	MsgDatetime  time.Time `json:"msg_datetime" xorm:"not null index(senderId_receiverId_datetime) DATETIME(6)"`
	MsgContent   string    `json:"msg_content" xorm:"not null VARCHAR(1024)"`
	MsgType      string    `json:"msg_type" xorm:"not null VARCHAR(12)"`
}
