package models

import (
	"time"
)

type FriendVerification struct {
	Id                   int64     `json:"id" xorm:"pk autoincr BIGINT(20)"`
	FriendId             int64     `json:"friend_id" xorm:"not null unique(pk) BIGINT(20)"`
	AccountId            int64     `json:"account_id" xorm:"not null unique(pk) BIGINT(20)"`
	BecomeFriendDatetime time.Time `json:"become_friend_datetime" xorm:"not null DATETIME(6)"`
}
