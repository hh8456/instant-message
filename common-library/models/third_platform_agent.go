package models

import (
	"time"
)

type ThirdPlatformAgent struct {
	Id              int64     `json:"id" xorm:"pk autoincr comment('自增id，不参与代码逻辑') BIGINT(20)"`
	AgentId         int64     `json:"agent_id" xorm:"not null comment('代理id') index BIGINT(20)"`
	AgentName       string    `json:"agent_name" xorm:"not null VARCHAR(256)"`
	ThirdPlatformId int64     `json:"third_platform_id" xorm:"not null comment('第三方平台id') BIGINT(20)"`
	RegisterTime    time.Time `json:"register_time" xorm:"not null comment('注册时间') DATETIME(6)"`
}
