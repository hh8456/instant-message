package models

type ChatGroup struct {
	Id          int64  `json:"id" xorm:"pk autoincr BIGINT(20)"`
	ChatGroupId int64  `json:"chat_group_id" xorm:"not null comment('聊天群唯一 id') unique BIGINT(20)"`
	Creator     int64  `json:"creator" xorm:"not null comment('创建者 id') index BIGINT(20)"`
	Name        string `json:"name" xorm:"not null comment('群名字') VARCHAR(60)"`
	Pic         []byte `json:"pic" xorm:"not null comment('群头像') VARBINARY(2048)"`
}
