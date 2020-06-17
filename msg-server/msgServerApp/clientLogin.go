package msgServerApp

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"instant-message/common-library/function"
	"instant-message/common-library/log"
	"instant-message/common-library/models"
	"instant-message/common-library/packet"
	"instant-message/common-library/proto/msg_err_code"
	"instant-message/common-library/proto/msg_id"
	"instant-message/common-library/proto/msg_struct"
	"instant-message/im-library/im_net"
	"instant-message/im-library/im_sms"
	"instant-message/msg-server/client"
	"instant-message/msg-server/clientManager"
	"instant-message/msg-server/eventmanager"
	"net"
	"regexp"
	"strconv"
	"time"

	"github.com/gomodule/redigo/redis"

	"github.com/hh8456/go-common/redisObj"

	"github.com/lifei6671/gorand"
)

func (g *msgServerObj) clientRegOrLogin(conn net.Conn, c *im_net.Socket) {
	defer function.Catch()

	buf, err := c.ReadOne()
	if err != nil {
		log.Errorf("服务器读取tcp数据c.ReadOne()出错:%v", err)
		c.Close()
		return
	}

	msgId := binary.BigEndian.Uint32(buf)

	switch msgId {
	// 注册
	case uint32(msg_id.NetMsgId_C2SRegisterAccount):
		g.registerAccount(conn, c, buf)

	// 手机号、密码登录
	case uint32(msg_id.NetMsgId_C2SLogin):
		g.clientLogin(conn, c, buf)

	// 用户名、密码登录 XXX 现在还没用用户名登录, 需要处理用户名重名如何处理?
	case uint32(msg_id.NetMsgId_C2SUsernameLogin):
		g.userNameLogin(conn, c, buf)

	// 第三方平台 登录（如果没有帐号自动注册）, 格式 "手机号-密码"
	case uint32(msg_id.NetMsgId_C2SThirdPlatformAccountLogin):
		g.ThirdPlatformLogin(conn, c, buf)

	// 获取验证码
	case uint32(msg_id.NetMsgId_C2SGetVerificationCode): // 需要客户端自己断开tcp连接，因为未开启收发协程，所以服务器会无响应
		g.getVerificationCode(conn, c, buf)

	// 忘记密码
	case uint32(msg_id.NetMsgId_C2SForgetPassword): // 需要客户端自己断开tcp连接，因为未开启收发协程，所以服务器会无响应
		g.forgetPassword(conn, c, buf)

	default:
		log.Errorf("客户端发来的第一条消息不是 msg_id.NetMsgId_C2SRegisterAccount 或 msg_id.NetMsgId_C2SLogin, 无条件关闭连接")
		c.Close()
		return
	}
}

func (g *msgServerObj) registerAccount(conn net.Conn, c *im_net.Socket, buf []byte) {
	c2sRegisterAccount := &msg_struct.C2SRegisterAccount{}
	if !function.ProtoUnmarshal(buf[packet.PacketHeaderSize:], c2sRegisterAccount, "msg_struct.C2SRegisterAccount") {
		sendErrCode(c, msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SRegisterAccount_failed, c2sRegisterAccount.CliTimeStamp)
		return
	}

	accountId := c2sRegisterAccount.GetAccountId()
	pwd := c2sRegisterAccount.GetPwd()
	/* 校验帐号 */
	if accountId == 0 {
		sendErrCode(c, msg_err_code.MsgErrCode_register_account_account_id_is_zero, c2sRegisterAccount.CliTimeStamp)
		log.Errorf("服务器接收到客户端注册信息，accountId为0,断开客户端连接")
		c.Close()
		return
	}

	/* 校验密码 */
	if pwd == "" {
		sendErrCode(c, msg_err_code.MsgErrCode_register_account_password_is_empty, c2sRegisterAccount.CliTimeStamp)
		log.Errorf("服务器接收到客户端注册信息，密码为空,断开客户端连接")
		c.Close()
		return
	}

	/* 校验验证码 */
	if "" == c2sRegisterAccount.VerificationCode {
		sendErrCode(c, msg_err_code.MsgErrCode_register_account_verification_is_empty, c2sRegisterAccount.CliTimeStamp)
		log.Errorf("服务器接收到客户端注册信息，验证码为空,断开客户端连接")
		c.Close()
		return
	}
	validateCode, err := g.rdsRegister.Get(strconv.FormatInt(c2sRegisterAccount.AccountId, 10))
	if nil != err {
		sendErrCode(c, msg_err_code.MsgErrCode_register_account_verification_is_error, c2sRegisterAccount.CliTimeStamp)
		log.Errorf("服务器接收到客户端注册信息，验证码错误,断开客户端连接")
		c.Close()
		return
	}
	if validateCode != c2sRegisterAccount.VerificationCode {
		sendErrCode(c, msg_err_code.MsgErrCode_register_account_verification_is_error, c2sRegisterAccount.CliTimeStamp)
		log.Errorf("服务器接收到客户端注册信息，验证码错误,断开客户端连接")
		c.Close()
		return
	}

	strAccount := strconv.FormatInt(accountId, 10)
	rdsSessClientBriefInfo := redisObj.NewSessionWithPrefix(client.RdsKeyPrefixBriefInfo)
	_, err = rdsSessClientBriefInfo.Get(strAccount)
	if err == nil {
		// 用户名已被注册
		sendErrCode(c, msg_err_code.MsgErrCode_register_account_account_is_already_used, c2sRegisterAccount.CliTimeStamp)

		return
	}

	rdsSessReg := redisObj.NewSessionWithPrefix(client.RdsKeyRegisterAccount)
	// 用分布式锁来保障同一时刻只有一个同名用户登录
	e2 := rdsSessReg.Setex(strAccount, time.Second*30, 1)
	if e2 != nil {
		c.Close()
		return
	}

	defer rdsSessReg.Del(strAccount)

	// 创建账号
	account, e2 := g.createAccount(accountId, pwd)
	if e2 != nil {
		log.Errorf("客户端创建角色时, 写入 mysql 出错: %v", e2)
		sendErrCode(c, msg_err_code.MsgErrCode_register_access_mysql_has_error_when_create_account, c2sRegisterAccount.CliTimeStamp)
		c.Close()
		return
	}

	if account == nil {
		log.Errorf("客户端注册时, 不应该出现这条日志")
		c.Close()
		return
	}

	cli := client.NewClient(account, conn, c, g.orm, c2sRegisterAccount.TerminalType, g.chatRecord)
	cli.SaveUsrBriefInfoToRedis()
	clientManager.StoreClient(cli)
	log.Tracef("account: %d 创建账号后,保持 client 对象到内存中", accountId)

	clientManager.WatchCloseSignal(cli)

	// 处理注册账号后, 只管转发消息
	cli.Run()

	log.Tracef("account: %d 注册", cli.GetAccountId())

	p := &msg_struct.S2CRegisterAccount{}
	p.AccountId = accountId
	p.Nickname = account.NickName
	p.LastMsgTimestamp = account.LastMsgTimestamp

	client.SetClientOnlineExpire(cli, cli.GetAccountId())

	g.recordLogin(cli.GetAccountId(), c)

	huanXinToken := cli.GetHuanXinToken()
	if "" == huanXinToken {
		log.Errorf("用户%d登录时，获取环信帐号失败", cli.GetAccountId())
		return
	} else {
		p.HuanXinAccount = &msg_struct.HuanXinAccount{
			Password: cli.ConfirmOwnHuanXinAccountExist(huanXinToken),
		}
	}

	cli.SendPbMsg(msg_id.NetMsgId_S2CRegisterAccount, p)
}

func (g *msgServerObj) clientLogin(conn net.Conn, c *im_net.Socket, buf []byte) {
	c2sLogin := &msg_struct.C2SLogin{}
	if !function.ProtoUnmarshal(buf[packet.PacketHeaderSize:], c2sLogin, "msg_struct.C2SLogin") {
		sendErrCode(c, msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SLogin_failed, c2sLogin.CliTimeStamp)
		return
	}

	accountId := c2sLogin.GetAccountId()
	pwd := c2sLogin.Pwd
	/* 校验帐号 */
	if accountId == 0 {
		log.Errorf("服务器接收到客户端登录信息，accountId为0,断开客户端连接")
		sendErrCode(c, msg_err_code.MsgErrCode_login_account_id_is_zero, c2sLogin.CliTimeStamp)
		c.Close()
		return
	}

	// 去数据库中查找
	account, e1 := g.queryAccountById(accountId, pwd)
	if e1 != nil {
		if e1 == sql.ErrNoRows {
			sendErrCode(c, msg_err_code.MsgErrCode_login_account_or_pwd_is_wrong, c2sLogin.CliTimeStamp)
			log.Errorf("用户名或者密码错误, account_id: %d", accountId)
			c.Close()
			return
		} else {
			log.Errorf("客户端登录时, accountId:%d, 读取 mysql 出错: %v", accountId, e1)
			sendErrCode(c, msg_err_code.MsgErrCode_login_access_mysql_has_error_when_load_account, c2sLogin.CliTimeStamp)
			c.Close()
			return
		}
	}

	/* 判断帐号状态，1为停用，0为启用 */
	if account.Status == 1 {
		log.Tracef("客户端%d登录，账号已停用")
		sendErrCode(c, msg_err_code.MsgErrCode_login_account_is_out_of_service, c2sLogin.CliTimeStamp)
		return
	}

	if account == nil {
		log.Errorf("客户端登录, 不应该出现这条日志")
		c.Close()
		return
	}

	strAccount := strconv.FormatInt(accountId, 10)
	rdsSessLogin := redisObj.NewSessionWithPrefix(client.RdskeyLoginAccount)
	// 用分布式锁来保障同一时刻只有一个同名用户登录
	e2 := rdsSessLogin.Setex(strAccount, time.Second*30, 1)
	if e2 != nil {
		// 提示客户端,账号在其他地方登录
		p := &msg_struct.S2CLoginSomewhereElse{SrvTimeStamp: time.Now().UnixNano() / 1e3}
		c.SendPbMsg(msg_id.NetMsgId_S2CLoginSomewhereElse, p)
		c.Close()
		return
	}

	defer rdsSessLogin.Del(strAccount)

	// 处理顶号
	done := clientManager.KickClient(accountId, c2sLogin.TerminalType)
	<-done

	cli := client.NewClient(account, conn, c, g.orm, c2sLogin.TerminalType, g.chatRecord)
	// 把简略信息保存到 redis
	cli.SaveUsrBriefInfoToRedis()
	// 从 redis 中读取群信息
	cli.LoadChatGroupInfo()
	clientManager.StoreClient(cli)
	log.Tracef("account: %d 登录后,保存 client 对象到内存中", accountId)

	clientManager.WatchCloseSignal(cli)

	eventArg := eventmanager.NewEventArg()
	eventArg.SetUserData("client", cli)
	eventmanager.Publish(eventmanager.ClientLoginEvent, eventArg)

	// 处理登录后, 只管转发消息
	cli.Run()

	log.Tracef("account: %d 登录", cli.GetAccountId())

	p := &msg_struct.S2CLogin{}
	p.AccountId = accountId
	p.Nickname = account.NickName
	p.PortraitUrl = account.HeadPortrait
	p.LastMsgTimestamp = account.LastMsgTimestamp

	// 处理离线好友请求或者好友验证
	g.handleOfflineFriendData(p, cli)

	// 从内存中拷贝群消息到 p 中
	g.assignPersonChatGroup(p, cli)
	// 其他好友删除"我"的记录
	g.GetDeletedFriendMsg(p, cli)

	p.AgentInfo = cli.GetAgentAndMember()

	// 在"我"离线期间,对方同意了我的好友请求,又把"我"删除了
	for _, deleteFriendId := range p.DeleteFriendId {
		for _, friendVer := range p.FriendVers {
			if deleteFriendId == friendVer.FriendId {
				cli.DeleteFriend(deleteFriendId)
				log.Tracef("account: %d, 在离线期间,对方 %d 同意了好友请求又删除了好友, 容错性处理完毕",
					cli.GetAccountId(), deleteFriendId)
			}
		}
	}

	// XXX 客户端在 S2CLogin 中发现有好友删除"我"的记录( S2CLogin.deleteFriendId ),
	// 如果在好友列表中( S2CLogin.FriendInfo )有对方,才发送 C2SConfirmDeleteFriend 给服务器
	// 如果好友列表中没有对方,就不需要发送 C2SConfirmDeleteFriend

	p.FriendInfo = cli.GetFriendInfoFromRedis()
	client.SetClientOnlineExpire(cli, cli.GetAccountId())

	g.recordLogin(cli.GetAccountId(), c)

	huanXinToken := cli.GetHuanXinToken()
	if "" == huanXinToken {
		log.Errorf("用户%d登录时，获取环信帐号失败", cli.GetAccountId())
		return
	}

	p.HuanXinAccount = &msg_struct.HuanXinAccount{
		Password: cli.ConfirmOwnHuanXinAccountExist(huanXinToken),
	}

	cli.SendPbMsg(msg_id.NetMsgId_S2CLogin, p)
}

func (g *msgServerObj) userNameLogin(conn net.Conn, c *im_net.Socket, buf []byte) {
	c2sLogin := &msg_struct.C2SUsernameLogin{}
	if !function.ProtoUnmarshal(buf[packet.PacketHeaderSize:], c2sLogin, "msg_struct.C2SLogin") {
		sendErrCode(c, msg_err_code.MsgErrCode_proto_unmarshal_msg_struct_C2SLogin_failed, c2sLogin.CliTimeStamp)
		return
	}

	username := c2sLogin.GetUsername()
	pwd := c2sLogin.Pwd
	/* 校验帐号 */
	if "" == username {
		log.Errorf("服务器接收到客户端登录信息，accountId为0,断开客户端连接")
		sendErrCode(c, msg_err_code.MsgErrCode_login_account_id_is_zero, c2sLogin.CliTimeStamp)
		c.Close()
		return
	}

	// 去数据库中查找
	account, e1 := g.queryAccountByUsername(username, pwd)
	if e1 != nil {
		if e1 == sql.ErrNoRows {
			sendErrCode(c, msg_err_code.MsgErrCode_login_account_or_pwd_is_wrong, c2sLogin.CliTimeStamp)
			log.Errorf("用户名或者密码错误, username: %s", username)
			c.Close()
			return
		} else {
			log.Errorf("客户端登录时, username:%s, 读取 mysql 出错: %v", username, e1)
			sendErrCode(c, msg_err_code.MsgErrCode_login_access_mysql_has_error_when_load_account, c2sLogin.CliTimeStamp)
			c.Close()
			return
		}
	}

	/* 判断帐号状态，1为停用，0为启用 */
	if account.Status == 1 {
		log.Tracef("客户端%d登录，账号已停用")
		sendErrCode(c, msg_err_code.MsgErrCode_login_account_is_out_of_service, c2sLogin.CliTimeStamp)
		return
	}

	if account == nil {
		log.Errorf("客户端登录, 不应该出现这条日志")
		c.Close()
		return
	}

	strAccount := strconv.FormatInt(account.AccountId, 10)
	rdsSessLogin := redisObj.NewSessionWithPrefix(client.RdskeyLoginAccount)
	// 用分布式锁来保障同一时刻只有一个同名用户登录
	e2 := rdsSessLogin.Setex(strAccount, time.Second*30, 1)
	if e2 != nil {
		// 提示客户端,账号在其他地方登录
		p := &msg_struct.S2CLoginSomewhereElse{SrvTimeStamp: time.Now().UnixNano() / 1e3}
		c.SendPbMsg(msg_id.NetMsgId_S2CLoginSomewhereElse, p)
		c.Close()
		return
	}

	defer rdsSessLogin.Del(strAccount)

	// 处理顶号
	done := clientManager.KickClient(account.AccountId, c2sLogin.TerminalType)
	<-done

	cli := client.NewClient(account, conn, c, g.orm, c2sLogin.TerminalType, g.chatRecord)
	// 把简略信息保存到 redis
	cli.SaveUsrBriefInfoToRedis()
	// 从 redis 中读取群信息
	cli.LoadChatGroupInfo()
	clientManager.StoreClient(cli)
	log.Tracef("account: %d 登录后,保存 client 对象到内存中", account.AccountId)

	clientManager.WatchCloseSignal(cli)

	eventArg := eventmanager.NewEventArg()
	eventArg.SetUserData("client", cli)
	eventmanager.Publish(eventmanager.ClientLoginEvent, eventArg)

	// 处理登录后, 只管转发消息
	cli.Run()

	log.Tracef("account: %d 登录", cli.GetAccountId())

	p := &msg_struct.S2CLogin{}
	p.AccountId = account.AccountId
	p.Nickname = account.NickName
	p.PortraitUrl = account.HeadPortrait
	p.LastMsgTimestamp = account.LastMsgTimestamp

	// 处理离线好友请求或者好友验证
	g.handleOfflineFriendData(p, cli)
	p.AgentInfo = cli.GetAgentAndMember()

	// 从内存中拷贝群消息到 p 中
	g.assignPersonChatGroup(p, cli)
	// 其他好友删除"我"的记录
	g.GetDeletedFriendMsg(p, cli)

	// 在"我"离线期间,对方同意了我的好友请求,又把"我"删除了
	for _, deleteFriendId := range p.DeleteFriendId {
		for _, friendVer := range p.FriendVers {
			if deleteFriendId == friendVer.FriendId {
				cli.DeleteFriend(deleteFriendId)
				log.Tracef("account: %d, 在离线期间,对方 %d 同意了好友请求又删除了好友, 容错性处理完毕",
					cli.GetAccountId(), deleteFriendId)
			}
		}
	}

	// XXX 客户端在 S2CLogin 中发现有好友删除"我"的记录( S2CLogin.deleteFriendId ),
	// 如果在好友列表中( S2CLogin.FriendInfo )有对方,才发送 C2SConfirmDeleteFriend 给服务器
	// 如果好友列表中没有对方,就不需要发送 C2SConfirmDeleteFriend

	p.FriendInfo = cli.GetFriendInfoFromRedis()
	client.SetClientOnlineExpire(cli, cli.GetAccountId())

	g.recordLogin(cli.GetAccountId(), c)

	huanXinToken := cli.GetHuanXinToken()
	if "" == huanXinToken {
		log.Errorf("用户%d登录时，获取环信帐号失败", cli.GetAccountId())
		return
	}

	p.HuanXinAccount = &msg_struct.HuanXinAccount{
		Password: cli.ConfirmOwnHuanXinAccountExist(huanXinToken),
	}

	p2 := &msg_struct.S2CUsernameLogin{
		AccountId:        p.AccountId,
		Nickname:         p.Nickname,
		PortraitUrl:      p.PortraitUrl,
		LastMsgTimestamp: p.LastMsgTimestamp,
		FriendReqs:       p.FriendReqs,
		FriendVers:       p.FriendVers,
		ChatGroupMembers: p.ChatGroupMembers,
		ReqJoinChatGroup: p.ReqJoinChatGroup,
		FriendInfo:       p.FriendInfo,
		ChatGroups:       p.ChatGroups,
		DeleteFriendId:   p.DeleteFriendId,
		HuanXinAccount:   p.HuanXinAccount,
		AgentInfo:        p.AgentInfo,
	}

	cli.SendPbMsg(msg_id.NetMsgId_S2CLogin, p2)
}

func (g *msgServerObj) CheckPhoneNum(region string, phoneNum int64) bool {
	regular, err := g.rdsRegionRegular.Get(region)
	if nil != err {
		return false
	}

	return regexp.MustCompile(regular).MatchString(strconv.FormatInt(phoneNum, 10))
}

func (g *msgServerObj) getVerificationCode(conn net.Conn, c *im_net.Socket, buf []byte) {
	getVerificationCode := &msg_struct.C2SGetVerificationCode{}
	if !function.ProtoUnmarshal(buf[packet.PacketHeaderSize:], getVerificationCode, "msg_struct.C2SGetVerificationCode") {
		sendErrCode(c, msg_err_code.MsgErrCode_proto_unmarshal_get_verification_code_has_error, 0)
		log.Errorf("%s获取短信验证码，反序列化 msg_struct.C2SGetVerificationCode 失败", c.RemoteAddr())
		return
	}

	// if !g.CheckPhoneNum(getVerificationCode.Region, getVerificationCode.PhoneNum) {
	// 	log.Errorf("%s获取短信验证码，手机号正则匹配失败", c.RemoteAddr())
	// 	sendErrCode(c, msg_err_code.MsgErrCode_get_verification_code_phone_num_error, 0)
	// 	return
	// }

	switch getVerificationCode.Action {
	case msg_struct.VerificationCodeAction_register: // 获取短信验证码用于注册帐号
		{
			/* 验证帐号是否存在 */
			account := &models.Account{}
			bGet, err := g.orm.Where("account_id = ?", getVerificationCode.PhoneNum).Exist(account)
			if nil != err {
				log.Errorf("%s获取短信验证码用于注册帐号，验证帐号是否存在出错%v", c.RemoteAddr(), err)
				sendErrCode(c, msg_err_code.MsgErrCode_get_verification_code_server_internal_error, 0)
				return
			}
			if bGet {
				log.Errorf("%s获取短信验证码用于注册帐号，帐号已存在", c.RemoteAddr())
				sendErrCode(c, msg_err_code.MsgErrCode_get_verification_code_register_account_exist, 0)
				return
			}

			/* 生成6位随机验证码 */
			validateCode := im_sms.GetValidateCode(6)

			/* 校验帐号是否已经获取过验证码，是否超时 */
			exist, err := redis.Int(g.rdsRegister.Do("SETNX", g.rdsRegister.AddPrefix(strconv.FormatInt(getVerificationCode.PhoneNum, 10)), validateCode))
			if nil != err {
				log.Errorf("%s获取短信验证码用于注册帐号，redis SETNX 出错%v", c.RemoteAddr(), err)
				sendErrCode(c, msg_err_code.MsgErrCode_get_verification_code_server_internal_error, 0)
				return
			}
			if 0 == exist { // 帐号已获取过密码，且还未超时
				log.Errorf("%s获取短信验证码用于注册帐号，帐号%d已获取过密码，且未超时", c.RemoteAddr(), getVerificationCode.PhoneNum)
				sendErrCode(c, msg_err_code.MsgErrCode_get_verification_code_register_not_timeout, 0)
				return
			}

			/* 设置超时时间 120s */
			if err := g.rdsRegister.Setex(strconv.FormatInt(getVerificationCode.PhoneNum, 10), time.Second*120, validateCode); nil != err {
				log.Errorf("%s获取短信验证码用于注册帐号，帐号%d设置验证码超时出错%v", c.RemoteAddr(), getVerificationCode.PhoneNum, err)
				sendErrCode(c, msg_err_code.MsgErrCode_get_verification_code_server_internal_error, 0)
				return
			}

			/* 发送短信验证码 */
			_, err = im_sms.SendSms(strconv.FormatInt(getVerificationCode.PhoneNum, 10), validateCode, "+86" == getVerificationCode.Region)
			if nil != err {
				log.Errorf("%s获取短信验证码用于注册帐号，帐号%d设置验证码超时出错%v", c.RemoteAddr(), getVerificationCode.PhoneNum, err)
				sendErrCode(c, msg_err_code.MsgErrCode_get_verification_code_server_internal_error, 0)
				return
			}
		}
	case msg_struct.VerificationCodeAction_forget_password: // 获取短信验证码用于忘记密码
		{
			/* 验证帐号是否存在 */
			account := &models.Account{}
			bGet, err := g.orm.Where("account_id = ?", getVerificationCode.PhoneNum).Exist(account)
			if nil != err {
				log.Errorf("%s获取短信验证码用于忘记密码，验证帐号是否存在出错%v", c.RemoteAddr(), err)
				sendErrCode(c, msg_err_code.MsgErrCode_get_verification_code_server_internal_error, 0)
				return
			}
			if !bGet {
				log.Errorf("%s获取短信验证码用于忘记密码，帐号不存在", c.RemoteAddr())
				sendErrCode(c, msg_err_code.MsgErrCode_get_verification_code_forget_password_account_is_not_exist, 0)
				return
			}

			/* 生成6位随机验证码 */
			validateCode := im_sms.GetValidateCode(6)

			/* 校验帐号是否已经获取过验证码，是否超时 */
			exist, err := redis.Int(g.rdsForgetPasswordSMS.Do("SETNX", g.rdsForgetPasswordSMS.AddPrefix(strconv.FormatInt(getVerificationCode.PhoneNum, 10)), validateCode))
			if nil != err {
				log.Errorf("%s获取短信验证码用于忘记密码，redis SETNX 出错%v", c.RemoteAddr(), err)
				sendErrCode(c, msg_err_code.MsgErrCode_get_verification_code_server_internal_error, 0)
				return
			}
			if 0 == exist { // 帐号已获取过密码，且还未超时
				log.Errorf("%s获取短信验证码用于忘记密码，帐号%d已获取过密码，且未超时", c.RemoteAddr(), getVerificationCode.PhoneNum)
				sendErrCode(c, msg_err_code.MsgErrCode_get_verification_code_forget_password_not_timeout, 0)
				return
			}

			/* 设置超时时间 120s */
			if err := g.rdsForgetPasswordSMS.Setex(strconv.FormatInt(getVerificationCode.PhoneNum, 10), time.Second*120, validateCode); nil != err {
				log.Errorf("%s获取短信验证码用于忘记密码，帐号%d设置验证码超时出错%v", c.RemoteAddr(), getVerificationCode.PhoneNum, err)
				sendErrCode(c, msg_err_code.MsgErrCode_get_verification_code_server_internal_error, 0)
				return
			}

			/* 发送短信验证码 */
			_, err = im_sms.SendSms(strconv.FormatInt(getVerificationCode.PhoneNum, 10), validateCode, "+86" == getVerificationCode.Region)
			if nil != err {
				log.Errorf("%s获取短信验证码用于忘记密码，帐号%d设置验证码超时出错%v", c.RemoteAddr(), getVerificationCode.PhoneNum, err)
				sendErrCode(c, msg_err_code.MsgErrCode_get_verification_code_server_internal_error, 0)
				return
			}
		}
	default: // 未知用途
		{
			sendErrCode(c, msg_err_code.MsgErrCode_get_verification_code_action_has_error, 0)
			log.Errorf("%s获取短信验证码，用途未知", c.RemoteAddr())
			return
		}
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CGetVerificationCode, &msg_struct.S2CGetVerificationCode{
		Ret: "Success",
	})
}

func (g *msgServerObj) forgetPassword(conn net.Conn, c *im_net.Socket, buf []byte) {
	forgetPassword := &msg_struct.C2SForgetPassword{}
	if !function.ProtoUnmarshal(buf[packet.PacketHeaderSize:], forgetPassword, "msg_struct.C2SForgetPassword") {
		log.Errorf("%s忘记密码时，反序列化 msg_struct.C2SForgetPassword 失败", c.RemoteAddr())
		sendErrCode(c, msg_err_code.MsgErrCode_proto_unmarshal_forget_password_has_error, 0)
		return
	}

	if 0 == forgetPassword.PhoneNum {
		log.Errorf("%s忘记密码时，帐号为空", c.RemoteAddr())
		sendErrCode(c, msg_err_code.MsgErrCode_forget_password_phone_num_is_empty, 0)
		return
	}

	if "" == forgetPassword.VerificationCode {
		log.Errorf("%s忘记密码时，帐号%d，验证码为空", c.RemoteAddr(), forgetPassword.PhoneNum)
		sendErrCode(c, msg_err_code.MsgErrCode_forget_password_verification_code_is_empty, 0)
		return
	}

	if "" == forgetPassword.Password {
		log.Errorf("%s忘记密码时，帐号%d，密码为空", c.RemoteAddr(), forgetPassword.PhoneNum)
		sendErrCode(c, msg_err_code.MsgErrCode_forget_password_password_is_empty, 0)
		return
	}

	/* 从 redis 获取验证码 */
	validateCode, err := g.rdsForgetPasswordSMS.Get(strconv.FormatInt(forgetPassword.PhoneNum, 10))
	if nil != err {
		log.Errorf("%s忘记密码时，帐号%d，从 redis 获取验证码出错%v", c.RemoteAddr(), forgetPassword.PhoneNum, err)
		sendErrCode(c, msg_err_code.MsgErrCode_forget_password_server_internal_error, 0)
		return
	}

	/* 校验验证码 */
	if validateCode != forgetPassword.VerificationCode {
		log.Errorf("%s忘记密码时，帐号%d，用户上传验证码与服务器记录的不一致", c.RemoteAddr(), forgetPassword.PhoneNum)
		sendErrCode(c, msg_err_code.MsgErrCode_forget_password_verification_code_error, 0)
		return
	} else { // 删除验证码
		g.rdsForgetPasswordSMS.Del(strconv.FormatInt(forgetPassword.PhoneNum, 10))
	}

	_, err = g.orm.DB().Exec("UPDATE account SET pwd = ? WHERE account_id = ?", forgetPassword.Password, forgetPassword.PhoneNum)
	if nil != err {
		log.Errorf("%s忘记密码时，帐号%d，修改数据库密码出错%v", c.RemoteAddr(), forgetPassword.PhoneNum, err)
		sendErrCode(c, msg_err_code.MsgErrCode_forget_password_server_internal_error, 0)
		return
	}

	c.SendPbMsg(msg_id.NetMsgId_S2CForgetPassword, &msg_struct.S2CForgetPassword{
		Ret: "Success",
	})
}

func sendErrCode(c *im_net.Socket, errCode msg_err_code.MsgErrCode, msgTimeStamp int64) {
	p := &msg_struct.S2CErrorCode{ErrCode: int64(errCode), MsgTimeStamp: msgTimeStamp}
	c.SendPbMsg(msg_id.NetMsgId_S2CErrorCode, p)
}

func (g *msgServerObj) recordLogin(accountId int64, c *im_net.Socket) {
	tm := time.Unix(time.Now().Unix(), 0)
	timeStr := tm.Format("2006-01-02 15:04:05")
	updateSql := fmt.Sprintf("UPDATE account SET last_login_timestamp = '%s', last_login_ip = '%s' WHERE account_id = %d", timeStr, c.RemoteAddr().String(), accountId)
	_, err := g.orm.DB().Exec(updateSql)
	if nil != err {
		log.Errorf("记录用户%d登录信息失败%v", accountId, err)
	}
}

// XXX 20.02.26 注释,系统稳定后删除
//func (g *msgServerObj) queryAccountById(accountId int64, pwd string) (*models.Account, error) {
//account := &models.Account{AccountId: accountId, Pwd: pwd}

////bGet, err := g.orm.Where("account_id = ?", accountId).And("pwd = ?", pwd).Get(account)
//has, err := g.orm.Cols("id", "contact_list", "last_msg_timestamp", "nick_name", "user_name", "agent_id", "head_portrait").Get(account)
//if err == nil && !has {
//err = sql.ErrNoRows
//}

//return account, err
//}

func (g *msgServerObj) queryAccountById(accountId int64, pwd string) (*models.Account, error) {
	account := &models.Account{AccountId: accountId, Pwd: pwd}
	has, err := g.orm.Get(account)
	if err == nil && !has {
		err = sql.ErrNoRows
	}

	return account, err
}

// XXX 20.02.26 注释,系统稳定后删除
//func (g *msgServerObj) queryAccountByUsername(username, pwd string) (*models.Account, error) {
//account := &models.Account{UserName: username, Pwd: pwd}

////bGet, err := g.orm.Where("account_id = ?", accountId).And("pwd = ?", pwd).Get(account)
//has, err := g.orm.Cols("id", "account_id", "contact_list", "last_msg_timestamp", "nick_name", "user_name", "agent_id", "head_portrait").Get(account)
//if err == nil && !has {
//err = sql.ErrNoRows
//}

//return account, err
//}

func (g *msgServerObj) queryAccountByUsername(username, pwd string) (*models.Account, error) {
	account := &models.Account{UserName: username, Pwd: pwd}

	has, err := g.orm.Get(account)
	if err == nil && !has {
		err = sql.ErrNoRows
	}

	return account, err
}

func (g *msgServerObj) createAccount(accountId int64, pwd string) (*models.Account, error) {
	nickname := string(gorand.KRand(6, gorand.KC_RAND_KIND_LOWER))
	account := &models.Account{
		AccountId:    accountId,
		NickName:     nickname,
		CreationTime: time.Now(),
		Pwd:          pwd,
	}
	id, err := g.orm.Insert(account)
	account.Id = id

	return account, err
}

func (g *msgServerObj) getAgreeBecomeFriendOfflineRequest(cli *client.Client) ([]*msg_struct.FriendVerification, error) {
	// account_id 通过了 "我" (friend_id) 的好友申请
	rows, err := g.orm.DB().Query("select account_id, become_friend_datetime from friend_verification where friend_id = ?", cli.GetAccountId())
	if nil != err {
		return nil, err
	}
	defer rows.Close()

	friendVerifications := []*msg_struct.FriendVerification{}
	for rows.Next() {
		var accountId int64
		var t time.Time
		if err := rows.Scan(&accountId, &t); nil != err {
			return nil, err
		}

		briefInfo, e := cli.GetOtherBriefInfoFromRds(accountId)
		if e != nil {
			continue
		}

		friendVerifications = append(friendVerifications, &msg_struct.FriendVerification{
			// account_id 通过了 "我" (friend_id) 的好友申请, 所以这里填 verification.AccountId
			FriendId:             accountId,
			FriendNickName:       briefInfo.Nickname,
			BecomeFriendDateTime: t.Unix(),
			FriendHeadPortrait:   cli.GetUserHeadPortraitUrl(accountId),
		})
	}

	return friendVerifications, nil
}

// 登录时处理离线好友请求和验证通过
func (g *msgServerObj) handleOfflineFriendData(p *msg_struct.S2CLogin, cli *client.Client) {
	// 别人发来的离线好友申请
	p.FriendReqs, _ = cli.GetAddFriendOfflineRequest()
	// 好友验证信息("我"加别人当好友,别人同意了)
	p.FriendVers, _ = g.getAgreeBecomeFriendOfflineRequest(cli)

	for _, friendVer := range p.FriendVers {
		cli.BeFriend(friendVer.FriendId)
		log.Tracef("account: %d 登录时处理了离线好友验证请求, 和 %d 成为好友", cli.GetAccountId(), friendVer.FriendId)
	}

	if len(p.FriendVers) > 0 {
		// 更新表 account 中的  contact_list 字段
		cli.SaveContactInfo()
		cli.SaveFriendInfoToRedis()

		g.orm.DB().Exec("delete from friend_verification where friend_id = ?",
			cli.GetAccountId())

		log.Tracef("account: %d 登录时处理了离线好友验证请求, 删除了表 "+
			"friend_verification 中 friend_id = %d 的所有记录", cli.GetAccountId(), cli.GetAccountId())
	}
}

//func (g *msgServerObj) assignPersonChatGroup(p *msg_struct.S2CLogin, cli *client.Client) {
//chatGroupMember := &models.ChatGroupMember{}
//rows, err := g.orm.Where("account_id = ?", cli.GetAccountId()).Rows(chatGroupMember)
//if nil != err {
//log.Errorf("account: %d 登录时读取表 chat_group_id 时出错: %v",
//cli.GetAccountId(), err)
//return
//}
//defer rows.Close()

//var groupIds []int64
//var lastMsgTimestamps []int64
//for rows.Next() {
//groupMember := &models.ChatGroupMember{}
//if err := rows.Scan(groupMember); nil == err {
//groupIds = append(groupIds, groupMember.ChatGroupId)
//lastMsgTimestamps = append(lastMsgTimestamps, groupMember.LastMsgTimestamp.UnixNano()/1e3)
//}
//}

//for i := 0; i < len(groupIds); i++ {
//pcg := &msg_struct.PersonChatGroup{}
//pcg.ChatGroup = cli.GetChatGroupInfo(groupIds[i])
//pcg.LastMsgTimestamp = lastMsgTimestamps[i]

//p.ChatGroups = append(p.ChatGroups, pcg)
//}
//}

//登录时加载群信息
func (g *msgServerObj) assignPersonChatGroup(p *msg_struct.S2CLogin, cli *client.Client) {
	rows, err := g.orm.DB().Query("select chat_group_id, last_msg_timestamp from chat_group_member WHERE account_id = ?", cli.GetAccountId())
	if nil != err {
		log.Errorf("account: %d 登录时读取表 chat_group_id 时出错: %v",
			cli.GetAccountId(), err)
		return
	}

	defer rows.Close()

	var groupIds []int64
	var lastMsgTimestamps []int64
	for rows.Next() {
		var groupId int64
		var lastMsgTimestamp int64
		if err := rows.Scan(&groupId, &lastMsgTimestamp); nil == err {
			groupIds = append(groupIds, groupId)
			lastMsgTimestamps = append(lastMsgTimestamps, lastMsgTimestamp)
		}
	}

	for i := 0; i < len(groupIds); i++ {
		pcg := &msg_struct.PersonChatGroup{}
		pcg.ChatGroup = cli.GetChatGroupInfo(groupIds[i])
		pcg.LastMsgTimestamp = lastMsgTimestamps[i]

		p.ChatGroups = append(p.ChatGroups, pcg)
	}
}

func (g *msgServerObj) GetDeletedFriendMsg(p *msg_struct.S2CLogin, cli *client.Client) {
	rows, err := g.orm.DB().Query("SELECT sender_id FROM friend_delete WHERE receiver_id = ? AND status = 0", cli.GetAccountId())
	if nil != err {
		log.Errorf("account:%d 登录时读取表 friend_delete 时出错：%v", cli.GetAccountId(), err)
		return
	}
	defer rows.Close()

	// m 用来去重
	m := map[int64]interface{}{}
	for rows.Next() {
		var deleteFriend int64
		if err := rows.Scan(&deleteFriend); nil == err {
			_, find := m[deleteFriend]
			if !find {
				p.DeleteFriendId = append(p.DeleteFriendId, deleteFriend)
			}
			m[deleteFriend] = nil
		}
	}
}
