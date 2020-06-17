package client

import (
	"encoding/json"
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"instant-message/common-library/models"
	"instant-message/common-library/packet"
	"instant-message/common-library/proto/msg_err_code"
	"instant-message/common-library/proto/msg_id"
	"instant-message/common-library/proto/msg_struct"
	"strconv"
	"time"
)

const (
	WEIBO_BRIEF_PIC = 4
)

func init() {
	// 客户端心跳包
	registerLogicCall(msg_id.NetMsgId_C2SHeartbeat, C2SHeartbeat)
	// 客户端上传厂商及推送id
	registerLogicCall(msg_id.NetMsgId_C2SDeviceInfo, C2SDeviceInfo)
	// 设置用户名
	registerLogicCall(msg_id.NetMsgId_C2SSetUserName, C2SSetUserName)
	// 查找用户
	registerLogicCall(msg_id.NetMsgId_C2SFindUser, C2SFindUser)
	// 设置头像
	registerLogicCall(msg_id.NetMsgId_C2SSetHeadPortrait, C2SSetHeadPortrait)
	// 修改密码
	registerLogicCall(msg_id.NetMsgId_C2SChangePassword, C2SChangePassword)
}

// C2SHeartbeat 客户端心跳包
func C2SHeartbeat(c *Client, msg []byte) {
	reqMsg := &msg_struct.C2SHeartbeat{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], reqMsg, "msg_struct.C2SHeartbeat") {
		return
	}

	SetClientOnlineExpire(c, c.GetAccountId())

	ackMsg := &msg_struct.S2CHeartbeat{}
	ackMsg.TimeStamp = reqMsg.GetTimeStamp()
	c.SendPbMsg(msg_id.NetMsgId_S2CHeartbeat, ackMsg)
}

// C2SDeviceInfo 客户端上传推送id和厂商
func C2SDeviceInfo(c *Client, msg []byte) {
	c2sDeviceInfo := &msg_struct.C2SDeviceInfo{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], c2sDeviceInfo, "msg_struct.C2SDeviceInfo") {
		log.Errorf("%d用户上传推送id和厂商信息，反序列化msg_struct.C2SDeviceInfo失败", c.GetAccountId())
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SDeviceInfo_has_error, 0)
		return
	}

	/* 校验厂商信息 */
	if "" == c2sDeviceInfo.DeviceInfo.DeviceInfo {
		log.Errorf("%d用户上传推送id和厂商信息，设备信息为空", c.GetAccountId())
		c.SendErrorCode(msg_err_code.MsgErrCode_device_info_device_info_is_empty, 0)
		return
	}
	if "" == c2sDeviceInfo.DeviceInfo.DeviceProducter {
		log.Errorf("%d用户上传推送id和厂商信息，厂商信息为空", c.GetAccountId())
		c.SendErrorCode(msg_err_code.MsgErrCode_device_info_device_producter_is_empty, 0)
		return
	}

	/* 报文校验通过 */

	/* 设备推送id，存入redis */
	deviceBytes, bRet := function.ProtoMarshal(&msg_struct.DeviceInfo{
		DeviceInfo:      c2sDeviceInfo.DeviceInfo.DeviceInfo,
		DeviceProducter: c2sDeviceInfo.DeviceInfo.DeviceProducter,
	}, "msg_struct.DeviceInfo")
	if !bRet {
		log.Errorf("%d用户上传推送id和厂商信息，序列化 msg_struct.DeviceInfo 失败", c.GetAccountId())
		c.SendErrorCode(msg_err_code.MsgErrCode_device_info_device_proto_marshal_msg_struct_DeviceInfo_has_error, 0)
		return
	}
	/* 校验此 c2sDeviceInfo.DeviceInfo.DeviceInfo 是否已经有账户使用，如果有，则清空 */
	strAccountId, err := c.rdsPushInfo.Get(c2sDeviceInfo.DeviceInfo.DeviceInfo)
	if nil == err {
		_, err = c.rdsPushInfo.Del(strAccountId)
		if nil != err {
			log.Errorf("%d用户上传推送id和厂商信息，删除 accountId-DeviceInfo键值对出错:%v", c.GetAccountId(), err)
		}
	}
	/* 存入 accountId-deviceInfo 键值对到 redis */
	if err = c.rdsPushInfo.Set(c.GetStrAccount(), string(deviceBytes)); nil != err {
		log.Errorf("客户端上传%d推送id%s和厂商%s时，存入用户与设备的映射redis失败", c.GetAccountId(), c2sDeviceInfo.DeviceInfo.DeviceInfo, c2sDeviceInfo.DeviceInfo.DeviceProducter)
		c.SendErrorCode(msg_err_code.MsgErrCode_device_info_account_device_info_save_into_redis_failed, 0)
		return
	}
	/* 存入 deviceInfo-accountId 键值对到 redis */
	if err = c.rdsPushInfo.Set(c2sDeviceInfo.DeviceInfo.DeviceInfo, c.GetStrAccount()); nil != err {
		log.Error("客户端上传%d推送id%s和厂商%s时，存入设备的映射与用户redis失败", c.GetAccountId(), c2sDeviceInfo.DeviceInfo.DeviceInfo, c2sDeviceInfo.DeviceInfo.DeviceProducter)
		c.SendErrorCode(msg_err_code.MsgErrCode_device_info_device_info_account_save_into_redis_failed, 0)
		return
	}

	/* 设备推送id、厂商信息，存入数据库 */
	if _, err = c.orm.DB().Exec("UPDATE account SET device_producter = ?, device_info = ? WHERE account_id = ?",
		c2sDeviceInfo.DeviceInfo.DeviceProducter, c2sDeviceInfo.DeviceInfo.DeviceInfo, c.GetAccountId()); nil != err {
		log.Errorf("客户端上传推送id和厂商时，存入数据库失败")
		c.SendErrorCode(msg_err_code.MsgErrCode_device_info_save_into_mysql_failed, 0)
		return
	}

	/* 回复客户端 */
	c.SendPbMsg(msg_id.NetMsgId_S2CDeviceInfo, &msg_struct.S2CDeviceInfo{
		Ret: "Success",
	})
}

// C2SSetUserName 设置用户名
func C2SSetUserName(c *Client, msg []byte) {
	setUserName := &msg_struct.C2SSetUserName{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], setUserName, "msg_struct.C2SSetUserName") {
		log.Errorf("%d用户设置用户名，反序列化 msg_struct.C2SSetUserName 失败", c.GetAccountId())
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_set_user_name_has_error, 0)
		return
	}

	/* 用户名不能为空 */
	if "" == setUserName.GetUserName() {
		log.Errorf("%d用户设置用户名，用户名为空", c.GetAccountId())
		c.SendErrorCode(msg_err_code.MsgErrCode_device_info_device_info_is_empty, 0)
		return
	}

	/* 报文校验通过 */

	/* 检测用户是否已经设置过用户名 */
	if "" != c.GetUserName() {
		log.Errorf("%d用户设置用户名，已设置过用户名，不允许修改", c.GetAccountId())
		c.SendErrorCode(msg_err_code.MsgErrCode_device_info_device_info_is_empty, 0)
		return
	}

	if c.rdsUserNameInfo.Exists(setUserName.GetUserName()) {
		log.Errorf("%d用户设置用户名，用户名已存在", c.GetAccountId(), setUserName.GetUserName())
		c.SendErrorCode(msg_err_code.MsgErrCode_set_user_name_user_name_is_exist, 0)
		return
	}

	/* 用户名存入redis简略信息 */
	c.SaveUsrBriefInfoToRedis()

	/* 用户名存入redis键值对，方便他人通过用户名查找到此用户 */
	if err := c.rdsUserNameInfo.Set(setUserName.UserName, c.GetStrAccount()); nil != err {
		log.Errorf("%d用户设置用户名，存入redis %s:%d失败", setUserName.UserName, c.GetStrAccount())
		c.SendErrorCode(msg_err_code.MsgErrCode_set_user_name_save_into_redis_failed, 0)
		return
	}

	// TODO:此处需要添加用户名合法性判断
	/* 用户名存入数据库 */
	_, err := c.orm.DB().Exec("UPDATE account SET user_name = ? WHERE account_id = ?",
		setUserName.UserName, c.GetAccountId())
	if nil != err {
		log.Errorf("%d用户设置用户名，存入数据库失败", setUserName.UserName, c.GetStrAccount())
		c.SendErrorCode(msg_err_code.MsgErrCode_set_user_name_save_into_mysql_failed, 0)
		return
	}

	/* 回复客户端 */
	c.SendPbMsg(msg_id.NetMsgId_S2CSetUserName, &msg_struct.S2CSetUserName{
		Ret: "Success",
	})
}

// C2SFindUser 查找用户
func C2SFindUser(c *Client, msg []byte) {
	findUser := &msg_struct.C2SFindUser{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], findUser, "msg_struct.C2SFindUser") {
		log.Errorf("用户%d搜索帐号时，反序列化 msg_struct.C2SFindUser 失败")
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_find_user_has_error, 0)
		return
	}

	if "" == findUser.GetKey() {
		log.Errorf("用户%d搜索帐号时，搜索关键字为空", c.GetAccountId())
		c.SendErrorCode(msg_err_code.MsgErrCode_find_user_key_is_empty, 0)
		return
	}

	/* 校验报文通过 */

	userBriefInfo := findUserByKey(c, findUser.GetKey())
	if nil == userBriefInfo {
		log.Tracef("用户%d搜索帐号关键字%s时，未查找到匹配结果", c.GetAccountId(), findUser.GetKey())
		c.SendErrorCode(msg_err_code.MsgErrCode_find_user_user_is_not_exist, 0)
		return
	}

	var userWeiboBriefUrls []string
	someoneWeiboBins, _, err := getSomeoneWeiboRds(c, userBriefInfo.AccountId, 0,0)
	if nil == err && nil != someoneWeiboBins {
		for _, someoneWeiboBin := range someoneWeiboBins {
			if WEIBO_BRIEF_PIC <= len(userWeiboBriefUrls) {
				break
			}
			weibo := &msg_struct.Weibo{}
			if !function.ProtoUnmarshal(someoneWeiboBin, weibo, "msg_struct.Weibo") {
				continue
			}
			if weibo.Type != weibo.Type&int64(msg_id.MsgWeibo_picture_only) && weibo.Type != weibo.Type&int64(msg_id.MsgWeibo_radio_only) {
				continue
			}
			weiboUrl := &msg_struct.WeiboUrl{}
			if !function.ProtoUnmarshal(weibo.UrlBin, weiboUrl, "msg_struct.WeiboUrl") {
				continue
			}
			for _, url := range weiboUrl.Urls {
				if WEIBO_BRIEF_PIC <= len(userWeiboBriefUrls) {
					break
				}
				userWeiboBriefUrls = append(userWeiboBriefUrls, url)
			}
		}
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CFindUser, &msg_struct.S2CFindUser{
		UserBriefInfo: userBriefInfo,
		BShow:         true,
		Url:           userWeiboBriefUrls,
	})
}

// C2SSetHeadPortrait 设置头像
func C2SSetHeadPortrait(c *Client, msg []byte) {
	setHeadPortrait := &msg_struct.C2SSetHeadPortrait{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], setHeadPortrait, "msg_struct.C2SSetHeadPortrait") {
		log.Errorf("%d用户设置头像，反序列化 msg_struct.C2SSetHeadPortrait 失败", c.GetAccountId())
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_set_head_portrait_has_error, 0)
		return
	}

	/* 头像地址不能为空 */
	if "" == setHeadPortrait.HeadPortraitUrl {
		log.Errorf("%d用户设置头像，头像地址为空", c.GetAccountId())
		c.SendErrorCode(msg_err_code.MsgErrCode_set_head_portrait_head_portrait_url_is_empty, 0)
		return
	}

	/* 头像地址存入内存 */
	c.account.HeadPortrait = setHeadPortrait.HeadPortraitUrl

	/* 头像地址存入redis */
	if err := setUserHeadPortraitUrlRds(c, setHeadPortrait.HeadPortraitUrl); nil != err {
		log.Errorf("用户%d设置头像url地址%s，存入redis出错：%v", c.GetAccountId(), setHeadPortrait.HeadPortraitUrl, err)
		c.SendErrorCode(msg_err_code.MsgErrCode_set_head_save_head_portrait_url_into_redis_failed, 0)
		return
	}

	/* 头像地址存入mysql */
	if err := setUserHeadPortraitUrlDB(c, setHeadPortrait.HeadPortraitUrl); nil != err {
		log.Errorf("用户%d设置头像url地址%s，存入数据库出错：%v", c.GetAccountId(), setHeadPortrait.HeadPortraitUrl, err)
		c.SendErrorCode(msg_err_code.MsgErrCode_set_head_save_head_portrait_url_into_mysql_failed, 0)
		return
	}

	/* 回复客户端设置头像成功 */
	c.SendPbMsg(msg_id.NetMsgId_S2CSetHeadPortrait, &msg_struct.S2CSetHeadPortrait{
		Ret: "Success",
	})
}

// C2SChangePassword 修改密码
func C2SChangePassword(c *Client, msg []byte) {
	changePassword := &msg_struct.C2SChangePassword{}
	if !function.ProtoUnmarshal(msg[packet.PacketHeaderSize:], changePassword, "msg_struct.C2SChangePassword") {
		log.Errorf("%d用户修改登录密码，反序列化msg_struct.C2SChangePassword出错", c.GetAccountId())
		c.SendErrorCode(msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SChangePassword_has_error, 0)
		return
	}

	if "" == changePassword.GetStrPassword() {
		log.Errorf("%d用户修改登录密码，新密码为空", c.GetAccountId())
		c.SendErrorCode(msg_err_code.MsgErrCode_change_password_new_password_is_empty, 0)
		return
	}

	/* 数据库修改登录密码 */
	_, err := c.orm.DB().Exec("UPDATE account SET pwd = ? WHERE account_id = ?", changePassword.GetStrPassword(), c.GetAccountId())
	if nil != err {
		log.Errorf("%d用户修改登录密码，更新数据库出错%v", c.GetAccountId(), err)
		c.SendErrorCode(msg_err_code.MsgErrCode_change_password_save_into_mysql_error, 0)
		return
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CChangePassword, &msg_struct.S2CChangePassword{
		Ret: "Success",
	})
}

// findUserByKey 通过关键字查找用户
func findUserByKey(c *Client, key string) *msg_struct.UserBriefInfo {
	/* 通过电话号码查找用户 */
	if userBriefInfoByPhoneNum := FindUserByPhoneNum(c, key); nil != userBriefInfoByPhoneNum {
		return userBriefInfoByPhoneNum
	}

	/* 通过用户名查找用户 */
	if userBriefInfoByUserName := FindUserByUserName(c, key); nil != userBriefInfoByUserName {
		return userBriefInfoByUserName
	}

	return nil
}

// 用SetClientOnlineExpire、DelClientOnlieExpire、IsUserOnline这三个函数暂时实现判断用户是否在线的功能
// TODO：后期优化点：需要在程序中加入时间轮，去掉心跳包中的更新有效期，用时间轮来更新有效期
// SetClientOnlineExpire 设置用户在线有效期
func SetClientOnlineExpire(c *Client, clientId int64) {
	_ = c.rdsOnline.Setex(c.strAccount, time.Minute*10, nil)
}

// DelClientOnlieExpire 删除用户在线状态
func DelClientOnlieExpire(c *Client, clientId int64) {
	_, _ = c.rdsOnline.Del(strconv.FormatInt(clientId, 10))
}

// IsUserOnline 判断用户是否在线
func IsUserOnline(c *Client, clientId int64) bool {
	return c.rdsOnline.Exists(strconv.FormatInt(clientId, 10))
}

// GetUserPushInfo 读取用户的推送信息
func GetUserPushInfo(c *Client, userId int64) *msg_struct.DeviceInfo {
	str, err := c.rdsPushInfo.Get(strconv.FormatInt(userId, 10))
	if nil != err {
		log.Errorf("获取%d，redis推送信息失败", userId)
		return nil
	}
	deviceInfo := &msg_struct.DeviceInfo{}
	if !function.ProtoUnmarshal([]byte(str), deviceInfo, "msg_struct.DeviceInfo") {
		log.Errorf("获取%d，反序列化redis推送信息失败", userId)
		return nil
	}
	return deviceInfo
}

// FindUserByUserName 通过用户名查找用户
func FindUserByUserName(c *Client, userName string) *msg_struct.UserBriefInfo {
	/* 从redis通过用户名查询用户 */
	briefInfo := &msg_struct.UserBriefInfo{}
	userBriefInfo, err := c.rdsUserNameInfo.Get(userName)
	if nil != err {
		/* 从redis通过用户名查找用户 */
		findAccount := &models.Account{}
		bGet, err := c.orm.Where("user_name = ?", userName).Get(findAccount)
		if nil != err || !bGet {
			return nil
		}
		briefInfo.AccountId = findAccount.AccountId
		briefInfo.UserName = findAccount.UserName
		briefInfo.Nickname = findAccount.NickName
		briefInfo.HeadPortrait = c.GetUserHeadPortraitUrl(findAccount.AccountId)
		briefInfo.Signature = ""
		return briefInfo
	}
	if nil != json.Unmarshal([]byte(userBriefInfo), briefInfo) {
		return nil
	}
	return briefInfo
}

// FindUserByPhoneNum 通过电话号码查找用户
func FindUserByPhoneNum(c *Client, phoneNum string) *msg_struct.UserBriefInfo {
	/* 从redis通过电话号码查询用户 */
	briefInfo := &msg_struct.UserBriefInfo{}
	strBriefInfo, err := c.rdsClientBriefInfo.Get(phoneNum)
	if nil != err {
		/* 从数据库通过电话号码查找用户 */
		findAccount := &models.Account{}
		bGet, err := c.orm.Where("account_id = ?", phoneNum).Get(findAccount)
		if nil != err || !bGet {
			return nil
		}
		briefInfo.AccountId = findAccount.AccountId
		briefInfo.UserName = findAccount.UserName
		briefInfo.Nickname = findAccount.NickName
		briefInfo.HeadPortrait = c.GetUserHeadPortraitUrl(findAccount.AccountId)
		briefInfo.Signature = ""
		return briefInfo
	}

	bRet := function.ProtoUnmarshal([]byte(strBriefInfo), briefInfo, "msg_struct.UserBriefInfo")
	if !bRet {
		/* 从数据库通过电话号码查找用户 */
		findAccount := &models.Account{}
		bGet, err := c.orm.Where("account_id = ?", phoneNum).Get(findAccount)
		if nil != err || !bGet {
			return nil
		}
		briefInfo.AccountId = findAccount.AccountId
		briefInfo.UserName = findAccount.UserName
		briefInfo.Nickname = findAccount.NickName
		briefInfo.HeadPortrait = c.GetUserHeadPortraitUrl(findAccount.AccountId)
		briefInfo.Signature = ""
		return briefInfo
	}

	return briefInfo
}

// setUserHeadPortraitUrlRds redis中设置用户头像
func setUserHeadPortraitUrlRds(c *Client, headPortraitUrl string) error {
	c.SaveUsrBriefInfoToRedis()
	return c.rdsHeadPortraitUrl.Set(c.GetStrAccount(), headPortraitUrl)
}

// setUserHeadPortraitUrlDB 数据库中设置用户头像
func setUserHeadPortraitUrlDB(c *Client, headPortraitUrl string) error {
	_, err := c.orm.DB().Exec("UPDATE account SET head_portrait = ? WHERE account_id = ?", headPortraitUrl, c.GetAccountId())
	return err
}
