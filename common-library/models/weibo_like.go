package models

import (
	"time"
)

type WeiboLike struct {
	Id               int64     `json:"id" xorm:"pk autoincr BIGINT(20)"`
	AccountId        int64     `json:"account_id" xorm:"not null index(accountId_srvTimestamp_thumbupTimestamp) BIGINT(20)"`
	WeiboId          int64     `json:"weibo_id" xorm:"not null index(accountId_srvTimestamp_thumbupTimestamp) BIGINT(20)"`
	LikerId          int64     `json:"liker_id" xorm:"not null comment('点赞者用户id') BIGINT(20)"`
	ThumbupTimestamp time.Time `json:"thumbup_timestamp" xorm:"not null comment('点赞时刻') index(accountId_srvTimestamp_thumbupTimestamp) DATETIME(6)"`
	Deleted          int       `json:"deleted" xorm:"not null default 0 comment('此条朋友圈是否被删除') TINYINT(6)"`
}
