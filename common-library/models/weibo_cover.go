package models

import (
	"time"
)

type WeiboCover struct {
	Id           int64     `json:"id" xorm:"pk autoincr comment('数据库自增id，不参与逻辑判断') BIGINT(20)"`
	AccountId    int64     `json:"account_id" xorm:"not null comment('朋友圈封面所属用户id') index BIGINT(20)"`
	CoverUrl     string    `json:"cover_url" xorm:"not null comment('朋友圈封面url') VARCHAR(256)"`
	DateTime     time.Time `json:"date_time" xorm:"not null comment('用户设置朋友圈相册封面时间') DATETIME(6)"`
	ThumbupTimes int       `json:"thumbup_times" xorm:"comment('朋友圈相册封面点赞次数') INT(20)"`
}
