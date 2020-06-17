package models

import (
	"time"
)

type Account struct {
	Id                 int64     `json:"id" xorm:"pk autoincr BIGINT(20)"`
	AccountId          int64     `json:"account_id" xorm:"not null default 0 unique index(account_pwd) BIGINT(20)"`
	UserName           string    `json:"user_name" xorm:"comment('用户名，用户只能设置一次') index(userName_pwd) VARCHAR(50)"`
	NickName           string    `json:"nick_name" xorm:"not null default 'noname' comment('昵称') VARCHAR(60)"`
	ContactList        []byte    `json:"contact_list" xorm:"comment('联系人列表') BLOB"`
	BlackList          []byte    `json:"black_list" xorm:"comment('黑名单列表') BLOB"`
	LastMsgTimestamp   int64     `json:"last_msg_timestamp" xorm:"not null default 0 comment('收到的最后一条消息的时间戳') BIGINT(20)"`
	CreationTime       time.Time `json:"creation_time" xorm:"not null comment('角色创建时间') DATETIME(6)"`
	Pwd                string    `json:"pwd" xorm:"not null index(account_pwd) index(userName_pwd) VARCHAR(128)"`
	HeadPortrait       string    `json:"head_portrait" xorm:"comment('头像url地址') VARCHAR(200)"`
	LastLoginTimestamp time.Time `json:"last_login_timestamp" xorm:"comment('客户端上次登录时间') DATETIME(6)"`
	LastLoginIp        string    `json:"last_login_ip" xorm:"comment('客户端上次登录ip地址') VARCHAR(50)"`
	DeviceProducter    string    `json:"device_producter" xorm:"comment('设备厂商（app手机厂商）') VARCHAR(50)"`
	DeviceInfo         string    `json:"device_info" xorm:"comment('手机推送ID') VARCHAR(50)"`
	Status             int       `json:"status" xorm:"not null default 0 comment('帐号状态，0为启用，1为禁用') INT(2)"`
	AgentId            int64     `json:"agent_id" xorm:"default 0 comment('用户的上级代理id; 如果没有就填 0') BIGINT(20)"`
}
