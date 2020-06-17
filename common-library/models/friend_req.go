package models

import (
	"time"
)

type FriendReq struct {
	Id          int64     `json:"id" xorm:"pk autoincr BIGINT(20)"`
	ReceiverId  int64     `json:"receiver_id" xorm:"not null unique(pk) BIGINT(20)"`
	SenderId    int64     `json:"sender_id" xorm:"not null unique(pk) BIGINT(20)"`
	ReqDatetime time.Time `json:"req_datetime" xorm:"not null DATETIME(6)"`
}
