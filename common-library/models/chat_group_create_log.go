package models

import (
	"time"
)

type ChatGroupCreateLog struct {
	Id             int64     `json:"id" xorm:"pk autoincr BIGINT(20)"`
	GroupId        int64     `json:"group_id" xorm:"not null index BIGINT(20)"`
	Creator        int64     `json:"creator" xorm:"not null index BIGINT(20)"`
	CreateDatetime time.Time `json:"create_datetime" xorm:"not null DATETIME(6)"`
}
