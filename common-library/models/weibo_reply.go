package models

import (
	"time"
)

type WeiboReply struct {
	Id            int64     `json:"id" xorm:"pk autoincr BIGINT(20)"`
	AccountId     int64     `json:"account_id" xorm:"not null comment('朋友圈发布者Id') BIGINT(20)"`
	WeiboId       int64     `json:"weibo_id" xorm:"not null comment('当前朋友圈Id') index(weiboId,commentId) BIGINT(20)"`
	CommentId     int64     `json:"comment_id" xorm:"not null comment('此条朋友圈评论的评论Id') index(weiboId,commentId) BIGINT(20)"`
	SenderId      int64     `json:"sender_id" xorm:"not null comment('此条朋友圈评论的发布者Id') BIGINT(20)"`
	BeCommentId   int64     `json:"be_comment_id" xorm:"comment('此条朋友圈评论回复评论Id') BIGINT(20)"`
	ReceiverId    int64     `json:"receiver_id" xorm:"comment('此条朋友圈回复评论的用户Id') BIGINT(20)"`
	MsgContent    string    `json:"msg_content" xorm:"not null VARCHAR(2048)"`
	ReplyDatetime time.Time `json:"reply_datetime" xorm:"not null DATETIME(6)"`
	Deleted       int       `json:"deleted" xorm:"default 0 comment('此条评论所在是朋友圈状态，0为正常，1为删除状态') INT(10)"`
	Status        int       `json:"status" xorm:"not null default 0 comment('用户评论状态，0为正常，1为删除') INT(10)"`
}
