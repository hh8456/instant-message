package models

import (
	"time"
)

type ChatGroupMemberQuitLog struct {
	Id               int64     `json:"id" xorm:"pk autoincr comment('自增Id，不参与代码逻辑') BIGINT(20)"`
	ChatGroupId      int64     `json:"chat_group_id" xorm:"not null comment('群Id') index BIGINT(20)"`
	ChatGroupOwnerId int64     `json:"chat_group_owner_id" xorm:"not null BIGINT(20)"`
	GroupMemberId    int64     `json:"group_member_id" xorm:"not null comment('群成员Id') BIGINT(20)"`
	Timestamp        time.Time `json:"timestamp" xorm:"not null comment('群成员离开群时间') DATETIME(6)"`
	Flag             int       `json:"flag" xorm:"not null comment('0：解散群；1：群成员主动退群；2：群主踢人') TINYINT(2)"`
}
