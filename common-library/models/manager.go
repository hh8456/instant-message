package models

import (
	"time"
)

type Manager struct {
	Id            int       `json:"id" xorm:"not null pk autoincr comment('无效id，不参与代码逻辑') INT(11)"`
	Username      string    `json:"username" xorm:"not null comment('用户名') index VARCHAR(256)"`
	Password      string    `json:"password" xorm:"not null comment('密码') VARCHAR(256)"`
	LastLoginTime time.Time `json:"last_login_time" xorm:"not null comment('上次登录时间') DATETIME(6)"`
	RegisterDate  time.Time `json:"register_date" xorm:"not null comment('注册时间') DATETIME(6)"`
	Level         string    `json:"level" xorm:"not null comment('管理帐号等级') VARCHAR(256)"`
	Status        int       `json:"status" xorm:"not null default 0 comment('管理员帐号状态，0为启用，1为停用') INT(2)"`
	Authority     int64     `json:"authority" xorm:"not null default 0 comment('管理员帐号权限') BIGINT(20)"`
}
