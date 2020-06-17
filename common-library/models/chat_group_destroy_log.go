package models

import (
	"time"
)

type ChatGroupDestroyLog struct {
	Id              int64     `json:"id" xorm:"pk autoincr comment('自增id') BIGINT(20)"`
	GroupId         int64     `json:"group_id" xorm:"not null comment('群id') index BIGINT(20)"`
	Creater         int64     `json:"creater" xorm:"not null comment('群主id') BIGINT(20)"`
	DestroyDatetime time.Time `json:"destroy_datetime" xorm:"not null comment('解散群时间') DATETIME(6)"`
}
