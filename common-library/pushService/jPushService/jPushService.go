package jPushService

import jPushClient "github.com/ylywyn/jpush-api-go-client"

const (
	jPushServiceAppKey = "29ad4ba97b4c2f396db62a40"
	jPushServiceSecret = "c3b212332bb60caaf66d9812"
	jPushNoticeAlert   = "IMNoticeAlert"
)

const (
	ANDROID = 1 << iota
	IOS
)

type JPushServiceParam struct {
	AndroidReceiver []string
	AndroidTitle    string
	AndroidContent  string
	IOSReceiver     []string
	IOSTitle        string
	IOSContent      string
}

type jPushService struct {
	param         *JPushServiceParam
	platFormIndex int
	platform      *jPushClient.Platform
	audience      *jPushClient.Audience
	notice        *jPushClient.Notice
}

// Push 极光推送
func Push(param *JPushServiceParam) (string, error) {
	service := &jPushService{}

	if 0 < len(param.AndroidReceiver) {
		service.platFormIndex |= ANDROID
		service.addAudience(param.AndroidReceiver)
	}
	if 0 < len(param.IOSReceiver) {
		service.platFormIndex |= IOS
		service.addAudience(param.IOSReceiver)
	}

	service.newJPushPlatForm()
	service.newNotice(param.AndroidTitle, param.AndroidContent, nil)

	payLoad := &jPushClient.PayLoad{}
	payLoad.SetPlatform(service.platform)
	payLoad.SetAudience(service.audience)
	payLoad.SetNotice(service.notice)

	bytes, err := payLoad.ToBytes()
	if nil != err {
		return "", err
	}

	client := jPushClient.NewPushClient(jPushServiceSecret, jPushServiceAppKey)
	send, err := client.Send(bytes)
	if nil != err {
		return "", err
	}
	return send, nil
}

// newJPushPlatForm 构建要推送的平台
func (this *jPushService) newJPushPlatForm() {
	this.platform = &jPushClient.Platform{}

	if 0 == ANDROID&^this.platFormIndex {
		this.platform.AddAndrid()
	}
	if 0 == IOS&^this.platFormIndex {
		this.platform.AddIOS()
	}
}

// newJPushAudience 构建接收推送听众
func (this *jPushService) newJPushAudience() *jPushClient.Audience {
	return &jPushClient.Audience{}
}

// addAudience 增加此次推送的听众
func (this *jPushService) addAudience(registrationIds []string) {
	if nil == this.audience {
		this.audience = this.newJPushAudience()
	}
	this.audience.SetID(registrationIds)
}

// newNotice 构建通知
func (this *jPushService) newNotice(title, content string, extras map[string]interface{}) {
	this.notice = &jPushClient.Notice{
		Alert: jPushNoticeAlert,
	}
	if 0 == ANDROID&^this.platFormIndex {
		this.notice.Android = &jPushClient.AndroidNotice{
			Title:  "IMChat",
			Alert:  "您有一条新消息",
			Extras: extras,
		}
	}
	if 0 == IOS&^this.platFormIndex {
		this.notice.IOS = &jPushClient.IOSNotice{
			Alert:    "您有一条新消息",
			Badge:    "+1",
			Sound:    "default",
			Category: "Category",
		}
	}
}
