package models

import (
	"time"
)

type FriendDelete struct {
	Id         int64     `json:"id" xorm:"pk autoincr comment('自增id，不参与代码逻辑') BIGINT(20)"`
	SenderId   int64     `json:"sender_id" xorm:"not null comment('发起删除好友的帐号') index(receiverId_sendId) BIGINT(20)"`
	ReceiverId int64     `json:"receiver_id" xorm:"not null comment('被删除的好友帐号') index(receiverId_sendId) BIGINT(20)"`
	DeleteTime time.Time `json:"delete_time" xorm:"not null comment('删除好友的时间') DATETIME(6)"`
	Status     int       `json:"status" xorm:"not null default 0 comment('标记被删除的用户是否已读') TINYINT(1)"`
}
