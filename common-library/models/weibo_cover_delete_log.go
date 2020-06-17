package models

import (
	"time"
)

type WeiboCoverDeleteLog struct {
	Id           int64     `json:"id" xorm:"pk autoincr comment('自增id，不参与逻辑判断') BIGINT(20)"`
	AccountId    int64     `json:"account_id" xorm:"not null comment('用户id') BIGINT(20)"`
	CoverUrl     string    `json:"cover_url" xorm:"not null comment('朋友圈相册封面url') VARCHAR(256)"`
	DateTime     time.Time `json:"date_time" xorm:"not null DATETIME(6)"`
	ThumbugTimes int       `json:"thumbug_times" xorm:"comment('朋友圈相册封面点赞次数总和') INT(20)"`
}
