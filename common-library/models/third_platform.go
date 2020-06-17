package models

import (
	"time"
)

type ThirdPlatform struct {
	Id                int64     `json:"id" xorm:"pk autoincr comment('自增id，不参与代码逻辑') BIGINT(6)"`
	ThirdPlatformId   int64     `json:"third_platform_id" xorm:"not null comment('第三方平台id，用以给此平台代理和玩家id加前缀') index BIGINT(20)"`
	ThirdPlatformName string    `json:"third_platform_name" xorm:"not null comment('第三方平台名称') unique VARCHAR(256)"`
	RegisterTime      time.Time `json:"register_time" xorm:"not null comment('第三方平台注册时间') DATETIME(6)"`
	AppKey            string    `json:"app_key" xorm:"not null index VARCHAR(256)"`
	AppPassword       string    `json:"app_password" xorm:"not null VARCHAR(256)"`
	Status            int       `json:"status" xorm:"not null default 0 comment('0表示正常，1表示禁用') TINYINT(2)"`
	Authority         int64     `json:"authority" xorm:"not null default 0 comment('权限') BIGINT(20)"`
}
