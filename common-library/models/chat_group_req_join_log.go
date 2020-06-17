package models

import (
	"time"
)

type ChatGroupReqJoinLog struct {
	Id             int64     `json:"id" xorm:"pk autoincr BIGINT(20)"`
	ChatGroupId    int64     `json:"chat_group_id" xorm:"not null index BIGINT(20)"`
	RequesterId    int64     `json:"requester_id" xorm:"not null index BIGINT(20)"`
	RequestMessage string    `json:"request_message" xorm:"comment('请求信息') VARCHAR(60)"`
	Processed      int       `json:"processed" xorm:"not null comment('0 是未处理, 1 是处理过; 拒绝或者同意加群,都算处理过') INT(6)"`
	ReqUid         int64     `json:"req_uid" xorm:"not null index BIGINT(20)"`
	ReqDate        time.Time `json:"req_date" xorm:"not null DATETIME(6)"`
}
