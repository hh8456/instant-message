package models

type Version struct {
	Id                    int64  `json:"id" xorm:"pk autoincr BIGINT(20)"`
	IosMinVersion         string `json:"ios_min_version" xorm:"not null VARCHAR(32)"`
	IosMaxVersion         string `json:"ios_max_version" xorm:"not null VARCHAR(32)"`
	IosUpdateUrl          string `json:"ios_update_url" xorm:"not null VARCHAR(1024)"`
	IosUpdateDescribe     string `json:"ios_update_describe" xorm:"not null TEXT"`
	AndroidMinVersion     string `json:"android_min_version" xorm:"not null VARCHAR(32)"`
	AndroidMaxVersion     string `json:"android_max_version" xorm:"not null VARCHAR(32)"`
	AndroidUpdateUrl      string `json:"android_update_url" xorm:"not null VARCHAR(1024)"`
	AndroidUpdateDescribe string `json:"android_update_describe" xorm:"TEXT"`
}
