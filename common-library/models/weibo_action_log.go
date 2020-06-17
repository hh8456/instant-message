package models

import (
	"time"
)

type WeiboActionLog struct {
	Id             int64     `json:"id" xorm:"pk autoincr BIGINT(20)"`
	AccountId      int64     `json:"account_id" xorm:"not null index(accountId_weiboId_actionDatetime) BIGINT(20)"`
	WeiboId        int64     `json:"weibo_id" xorm:"comment('只有为别人点赞或留言时,weibo_id 是对方的微博 id') index(accountId_weiboId_actionDatetime) BIGINT(20)"`
	Action         int       `json:"action" xorm:"not null default 0 comment('0 发微博, 1 删除微博, 2 点赞, 3 取消点赞, 4 评论微博, 5 删除微博评论') INT(6)"`
	FriendId       int64     `json:"friend_id" xorm:"default 0 comment('只有为别人点赞或留言时, friend_id 才有意义') BIGINT(20)"`
	ActionDatetime time.Time `json:"action_datetime" xorm:"not null comment('个人动态产生的时间') index(accountId_weiboId_actionDatetime) DATETIME(6)"`
}
