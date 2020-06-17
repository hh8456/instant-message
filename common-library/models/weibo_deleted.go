package models

import (
	"time"
)

type WeiboDeleted struct {
	Id           int64     `json:"id" xorm:"pk autoincr BIGINT(20)"`
	AccountId    int64     `json:"account_id" xorm:"not null index(accountId_msgDatetime_delDatetime) BIGINT(20)"`
	PhotosBin    []byte    `json:"photos_bin" xorm:"BLOB"`
	MsgContent   string    `json:"msg_content" xorm:"VARCHAR(2048)"`
	WeiboId      int64     `json:"weibo_id" xorm:"not null BIGINT(20)"`
	MsgDatetime  time.Time `json:"msg_datetime" xorm:"not null index(accountId_msgDatetime_delDatetime) DATETIME(6)"`
	ThumbupTimes int       `json:"thumbup_times" xorm:"not null default 0 comment('点赞次数') INT(10)"`
	DelDatetime  time.Time `json:"del_datetime" xorm:"not null index(accountId_msgDatetime_delDatetime) DATETIME(6)"`
}
